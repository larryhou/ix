package remotepair

// Tests for the pure-Go TLS-PSK client (tlspsk.go).
//
// TestPSKConn_SelfPipe  — in-process client+server over net.Pipe, no external deps.
// TestPSKConn_OpenSSL   — optional: validates against a real `openssl s_server`
//                         (skipped if openssl is absent or lacks PSK ciphers).

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ---- helpers ----------------------------------------------------------------

func findFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("findFreePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// ---- TestPSKConn_SelfPipe ---------------------------------------------------

// TestPSKConn_SelfPipe validates newPSKConn against the in-process tlsPSKServer
// (tlspsk_server_test.go). No external tools required.
func TestPSKConn_SelfPipe(t *testing.T) {
	psk := make([]byte, 32)
	if _, err := rand.Read(psk); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}

	clientSide, serverSide := net.Pipe()

	// server goroutine: handshake then echo
	serverErr := make(chan error, 1)
	go func() {
		s := &tlsPSKServer{conn: serverSide, psk: psk}
		if err := s.handshake(); err != nil {
			serverSide.Close()
			serverErr <- fmt.Errorf("server handshake: %w", err)
			return
		}
		// echo loop — read one message then send it back
		buf := make([]byte, 4096)
		n, err := s.Read(buf)
		if err != nil {
			serverSide.Close()
			serverErr <- fmt.Errorf("server read: %w", err)
			return
		}
		_, err = s.Write(buf[:n])
		serverSide.Close()
		serverErr <- err
	}()

	// client: handshake
	conn, err := newPSKConn(clientSide, psk)
	if err != nil {
		t.Fatalf("client PSK handshake failed: %v", err)
	}
	defer conn.Close()

	// client: write then read echo
	want := "hello from client"
	if _, err := conn.Write([]byte(want)); err != nil {
		t.Fatalf("client write: %v", err)
	}

	buf := make([]byte, len(want))
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("client read echo: %v", err)
	}

	if got := string(buf); got != want {
		t.Fatalf("echo mismatch: got %q, want %q", got, want)
	}
	t.Logf("SelfPipe round-trip OK: %q", buf)

	if err := <-serverErr; err != nil {
		t.Fatalf("server error: %v", err)
	}
}

// ---- TestPSKConn_OpenSSL ----------------------------------------------------

// TestPSKConn_OpenSSL validates newPSKConn against a real `openssl s_client`.
// A Go TCP listener accepts the raw connection; openssl s_client performs the
// TLS-PSK handshake from the other side, then we do a data round-trip.
// Skipped automatically when openssl is absent or lacks PSK-AES256-GCM-SHA384.
func TestPSKConn_OpenSSL(t *testing.T) {
	path, err := exec.LookPath("openssl")
	if err != nil {
		t.Skip("openssl not found in PATH")
	}
	out, _ := exec.Command(path, "ciphers", "PSK-AES256-GCM-SHA384").Output()
	if !strings.Contains(string(out), "PSK-AES256-GCM-SHA384") {
		t.Skip("openssl does not support PSK-AES256-GCM-SHA384")
	}

	psk := make([]byte, 32)
	if _, err := rand.Read(psk); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	pskHex := hex.EncodeToString(psk)

	// Start a Go TCP listener; the tlsPSKServer will handle the TLS handshake.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	const msg = "hello from openssl client\n"

	// Server goroutine: accept one connection, do PSK handshake, echo one message.
	serverErr := make(chan error, 1)
	go func() {
		raw, err := ln.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		s := &tlsPSKServer{conn: raw, psk: psk}
		if err := s.handshake(); err != nil {
			raw.Close()
			serverErr <- fmt.Errorf("server handshake: %w", err)
			return
		}
		buf := make([]byte, len(msg))
		if _, err := io.ReadFull(s, buf); err != nil {
			raw.Close()
			serverErr <- fmt.Errorf("server read: %w", err)
			return
		}
		_, err = s.Write(buf)
		raw.Close()
		serverErr <- err
	}()

	// openssl s_client: connects, sends msg, reads echo.
	// -ign_eof keeps the client alive until we close it.
	clientCmd := exec.Command(path,
		"s_client",
		"-connect", fmt.Sprintf("127.0.0.1:%d", port),
		"-psk", pskHex,
		"-cipher", "PSK-AES256-GCM-SHA384",
		"-tls1_2",
		"-quiet",
		"-ign_eof",
	)
	clientIn, _ := clientCmd.StdinPipe()
	clientOut, _ := clientCmd.StdoutPipe()
	clientStderr, _ := clientCmd.StderrPipe()

	if err := clientCmd.Start(); err != nil {
		t.Fatalf("openssl s_client start: %v", err)
	}
	defer func() {
		clientIn.Close()
		clientCmd.Process.Kill()
		clientCmd.Wait()
	}()

	// Log openssl stderr in background for diagnostics.
	go func() {
		b, _ := io.ReadAll(clientStderr)
		if len(b) > 0 {
			t.Logf("openssl stderr: %s", b)
		}
	}()

	// Give the TLS handshake a moment to complete, then send data.
	time.Sleep(500 * time.Millisecond)
	if _, err := io.WriteString(clientIn, msg); err != nil {
		t.Fatalf("write to openssl stdin: %v", err)
	}

	// Read the echo back through openssl's stdout (with timeout via goroutine).
	got := make([]byte, len(msg))
	readDone := make(chan error, 1)
	go func() {
		_, e := io.ReadFull(clientOut, got)
		readDone <- e
	}()
	select {
	case e := <-readDone:
		if e != nil {
			t.Fatalf("read openssl stdout: %v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out reading openssl echo")
	}

	if string(got) != msg {
		t.Fatalf("echo mismatch: got %q, want %q", got, msg)
	}
	t.Logf("OpenSSL round-trip OK: %q", got)

	if err := <-serverErr; err != nil {
		t.Fatalf("server error: %v", err)
	}
}
