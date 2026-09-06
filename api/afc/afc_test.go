package afc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Wire helpers
// ---------------------------------------------------------------------------

var testBO = binary.LittleEndian

func readFull(conn net.Conn, buf []byte) error {
	for off := 0; off < len(buf); {
		n, err := conn.Read(buf[off:])
		off += n
		if err != nil {
			return err
		}
	}
	return nil
}

// newTestConn returns a pair of buffered net.Conn backed by a real TCP loopback,
// unlike net.Pipe which is synchronous and blocks Write until the peer reads.
func newTestConn(t *testing.T) (client, server net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	ch := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			close(ch)
			return
		}
		ch <- c
	}()
	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	server = <-ch
	t.Cleanup(func() { client.Close(); server.Close() })
	return
}

// readOneRequest reads exactly one AFC request from conn.
// Returns (opcode, null-terminated-args, sn, error).
func readOneRequest(conn net.Conn) (op uint64, args []byte, sn uint64, err error) {
	hdr := make([]byte, headerSize)
	if err = readFull(conn, hdr); err != nil {
		return
	}
	pktLen := testBO.Uint64(hdr[8:16])
	hdrLen := testBO.Uint64(hdr[16:24])
	sn = testBO.Uint64(hdr[24:32])
	op = testBO.Uint64(hdr[32:40])
	argsLen := int(hdrLen) - headerSize
	if argsLen > 0 {
		args = make([]byte, argsLen)
		if err = readFull(conn, args); err != nil {
			return
		}
	}
	bodyLen := int(pktLen - hdrLen)
	if bodyLen > 0 {
		drain := make([]byte, bodyLen)
		err = readFull(conn, drain)
	}
	return
}

// writeFrame writes one AFC response frame to conn.
func writeFrame(conn net.Conn, sn, op uint64, payload []byte) error {
	buf := &bytes.Buffer{}
	rsv := make([]byte, 8)
	copy(rsv, magic)
	buf.Write(rsv)
	length := uint64(headerSize + len(payload))
	testBO.PutUint64(rsv, length)
	buf.Write(rsv)
	testBO.PutUint64(rsv, uint64(headerSize))
	buf.Write(rsv)
	testBO.PutUint64(rsv, sn)
	buf.Write(rsv)
	testBO.PutUint64(rsv, op)
	buf.Write(rsv)
	buf.Write(payload)
	_, err := conn.Write(buf.Bytes())
	return err
}

func writeDataFrame(conn net.Conn, sn uint64, payload []byte) error {
	return writeFrame(conn, sn, opData, payload)
}

func writeStatusFrame(conn net.Conn, sn uint64, rc Retcode) error {
	b := make([]byte, 8)
	testBO.PutUint64(b, uint64(rc))
	return writeFrame(conn, sn, opStatus, b)
}

// buildKV encodes AFC key-value pairs: key\0value\0...
func buildKV(pairs ...string) []byte {
	var b []byte
	for _, s := range pairs {
		b = append(b, []byte(s)...)
		b = append(b, 0)
	}
	return b
}

// buildDirListing encodes a directory listing: name\0name\0...
func buildDirListing(names ...string) []byte {
	var b []byte
	for _, n := range names {
		b = append(b, []byte(n)...)
		b = append(b, 0)
	}
	return b
}

// pathArg extracts the null-terminated path string from AFC args bytes.
func pathArg(args []byte) string {
	end := bytes.IndexByte(args, 0)
	if end < 0 {
		return string(args)
	}
	return string(args[:end])
}

// ---------------------------------------------------------------------------
// Filesystem model
// ---------------------------------------------------------------------------

type treeEntry struct {
	isDir bool
	size  int64
}

// directChildren returns the immediate children of dir in the given fs map.
func directChildren(fs map[string]treeEntry, dir string) []string {
	prefix := dir
	if prefix != "/" {
		prefix += "/"
	}
	var out []string
	for k := range fs {
		if k == dir {
			continue
		}
		if len(k) <= len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		rest := k[len(prefix):]
		hasSlash := false
		for _, c := range rest {
			if c == '/' {
				hasSlash = true
				break
			}
		}
		if !hasSlash && len(rest) > 0 {
			out = append(out, rest)
		}
	}
	return out
}

// buildResponse returns the response payload for a single request.
func buildResponse(fs map[string]treeEntry, op uint64, name string) (respOp uint64, payload []byte) {
	switch op {
	case opReadDir:
		entry, ok := fs[name]
		if !ok || !entry.isDir {
			b := make([]byte, 8)
			testBO.PutUint64(b, uint64(retObjectNotFound))
			return opStatus, b
		}
		return opData, buildDirListing(directChildren(fs, name)...)

	case opGetFileInfo:
		entry, ok := fs[name]
		if !ok {
			b := make([]byte, 8)
			testBO.PutUint64(b, uint64(retPermDenied))
			return opStatus, b
		}
		ifmt := "S_IFREG"
		if entry.isDir {
			ifmt = "S_IFDIR"
		}
		return opData, buildKV(
			"st_ifmt", ifmt,
			"st_size", fmt.Sprintf("%d", entry.size),
			"st_nlink", "1",
			"st_blocks", "0",
		)
	}
	b := make([]byte, 8)
	testBO.PutUint64(b, uint64(retOpNotSupported))
	return opStatus, b
}

// ---------------------------------------------------------------------------
// Mock servers
// ---------------------------------------------------------------------------

// runSequentialMockAFC is the naive one-request-one-response server.
// A pipeline implementation will still pass this, but so will a sequential one.
func runSequentialMockAFC(t *testing.T, fs map[string]treeEntry) *Service {
	t.Helper()
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		for {
			op, args, sn, err := readOneRequest(server)
			if err != nil {
				return
			}
			name := pathArg(args)
			respOp, payload := buildResponse(fs, op, name)
			if err := writeFrame(server, sn, respOp, payload); err != nil {
				return
			}
		}
	}()
	return New(client)
}

// runPipelineMockAFC starts a full-duplex mock server that models a real
// device: the rx goroutine reads requests as fast as they arrive and queues
// them; the tx goroutine dequeues and replies, sleeping perReqDelay before
// each reply to simulate processing / network latency.
//
// Because rx and tx are independent, the server never stalls the client's
// send path — exactly like a real device.  The delay means:
//   - sequential client:  total ≈ N * perReqDelay   (send→wait→send→wait…)
//   - pipelined client:   total ≈ 1 * perReqDelay   (all requests in flight)
//
// The test uses elapsed time to distinguish the two.
func runPipelineMockAFC(t *testing.T, fs map[string]treeEntry, perReqDelay time.Duration) *Service {
	t.Helper()
	client, server := net.Pipe()

	go func() {
		defer server.Close()

		type pendingReq struct {
			op   uint64
			name string
			sn   uint64
		}

		queue := make(chan pendingReq, 1024)

		// rx goroutine: reads requests off the wire as fast as possible
		go func() {
			defer close(queue)
			for {
				op, args, sn, err := readOneRequest(server)
				if err != nil {
					return
				}
				queue <- pendingReq{op: op, name: pathArg(args), sn: sn}
			}
		}()

		// tx goroutine: dequeues and replies, with a per-request delay
		for req := range queue {
			time.Sleep(perReqDelay)
			respOp, payload := buildResponse(fs, req.op, req.name)
			if err := writeFrame(server, req.sn, respOp, payload); err != nil {
				return
			}
		}
	}()

	return New(client)
}

// ---------------------------------------------------------------------------
// Correctness tests (use sequential server — verify results)
// ---------------------------------------------------------------------------

func TestListFlat(t *testing.T) {
	fs := map[string]treeEntry{
		"/":      {isDir: true},
		"/a.txt": {size: 100},
		"/b.mp4": {size: 200},
		"/c.jpg": {size: 300},
	}
	svc := runSequentialMockAFC(t, fs)
	items, err := svc.List("/", false)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	sort.Strings(names)
	for i, want := range []string{"/a.txt", "/b.mp4", "/c.jpg"} {
		if names[i] != want {
			t.Errorf("item[%d]: got %q want %q", i, names[i], want)
		}
	}
}

func TestListNonRecursive(t *testing.T) {
	fs := map[string]treeEntry{
		"/":             {isDir: true},
		"/sub":          {isDir: true},
		"/sub/deep.txt": {size: 42},
		"/root.txt":     {size: 10},
	}
	svc := runSequentialMockAFC(t, fs)
	items, err := svc.List("/", false)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	names := map[string]bool{}
	for _, it := range items {
		names[it.Name] = true
	}
	if names["/sub/deep.txt"] {
		t.Error("non-recursive List should not descend into subdirectory")
	}
	if !names["/root.txt"] {
		t.Error("expected /root.txt in result")
	}
}

func TestListRecursive(t *testing.T) {
	fs := map[string]treeEntry{
		"/":              {isDir: true},
		"/a":             {isDir: true},
		"/a/b":           {isDir: true},
		"/a/b/file1.txt": {size: 1},
		"/a/file2.txt":   {size: 2},
		"/file3.txt":     {size: 3},
	}
	svc := runSequentialMockAFC(t, fs)
	items, err := svc.List("/", true)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	names := map[string]bool{}
	for _, it := range items {
		if it.IsDir() {
			t.Errorf("unexpected dir in result: %s", it.Name)
		}
		names[it.Name] = true
	}
	for _, want := range []string{"/a/b/file1.txt", "/a/file2.txt", "/file3.txt"} {
		if !names[want] {
			t.Errorf("missing expected file %q", want)
		}
	}
	if len(items) != 3 {
		t.Errorf("expected 3 files, got %d", len(items))
	}
}

func TestListPermDeniedOnStat(t *testing.T) {
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		for {
			op, args, sn, err := readOneRequest(server)
			if err != nil {
				return
			}
			name := pathArg(args)
			switch op {
			case opReadDir:
				if name == "/" {
					writeDataFrame(server, sn, buildDirListing("ok.txt", "secret"))
				} else {
					writeStatusFrame(server, sn, retObjectNotFound)
				}
			case opGetFileInfo:
				if name == "/ok.txt" {
					writeDataFrame(server, sn, buildKV(
						"st_ifmt", "S_IFREG",
						"st_size", "99",
						"st_nlink", "1",
						"st_blocks", "0",
					))
				} else {
					writeStatusFrame(server, sn, retPermDenied)
				}
			}
		}
	}()
	svc := New(client)
	items, err := svc.List("/", false)
	if err != nil {
		t.Fatalf("List should not fail on PermDenied stat, got: %v", err)
	}
	if len(items) != 1 || items[0].Name != "/ok.txt" {
		t.Fatalf("expected [/ok.txt], got %v", items)
	}
}

func TestListSizes(t *testing.T) {
	fs := map[string]treeEntry{
		"/":        {isDir: true},
		"/big.mp4": {size: 1_500_000_000},
		"/tiny.db": {size: 42},
	}
	svc := runSequentialMockAFC(t, fs)
	items, err := svc.List("/", false)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	sizes := map[string]int64{}
	for _, it := range items {
		sizes[it.Name] = it.Size
	}
	if sizes["/big.mp4"] != 1_500_000_000 {
		t.Errorf("big.mp4 size: got %d want 1500000000", sizes["/big.mp4"])
	}
	if sizes["/tiny.db"] != 42 {
		t.Errorf("tiny.db size: got %d want 42", sizes["/tiny.db"])
	}
}

func TestListEmptyDir(t *testing.T) {
	fs := map[string]treeEntry{
		"/": {isDir: true},
	}
	svc := runSequentialMockAFC(t, fs)
	items, err := svc.List("/", true)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

// ---------------------------------------------------------------------------
// Pipeline test (uses batch server — deadlocks a sequential implementation)
// ---------------------------------------------------------------------------

// TestListPipeline verifies pipeline benefit on a deep multi-level directory tree.
//
// Pipeline benefit requires multiple levels: when GetFileInfo reveals a subdir,
// send goroutine immediately dispatches ReadDir(subdir) without waiting for
// sibling GetFileInfo responses — overlapping layer N+1 sends with layer N recvs.
//
// Tree: 5 levels deep, 4 dirs per level, 4 files at leaf level.
//   total dirs  = 4^0 + 4^1 + 4^2 + 4^3 + 4^4 = 341
//   total files = 4^4 * 4 = 1024  (only leaves have files)
//   total requests = 341 ReadDir + (341+1024) GetFileInfo = 1706
//
// Sequential: every request waits for its reply before the next is sent.
//   At each layer boundary, client must wait for ALL GetFileInfo of that layer
//   before it knows which ReadDirs to send next.
//   Effective cost per layer transition: width * rtt
//
// Pipelined: as soon as ANY GetFileInfo comes back as a directory, ReadDir is
//   sent immediately — overlapping with remaining GetFileInfo responses.
//   The server rx queue will have multiple requests queued while processing.
//
// Assertion: server rx queue max depth > 1 (proves requests arrive in batches).
func TestListPipeline(t *testing.T) {
	// Tree: 4 levels deep, each non-leaf level has 3 dirs + 2 files.
	// This ensures pipeline benefit: when recv processes a dir GetFileInfo,
	// send dispatches ReadDir immediately — while recv still has sibling
	// file GetFileInfo responses to process.
	const (
		depth  = 4
		nDirs  = 3 // subdirs per level
		nFpL   = 2 // files per level (mixed with dirs)
		rtt    = 2 * time.Millisecond
	)

	fs := map[string]treeEntry{"/": {isDir: true}}
	var buildTree func(parent string, level int)
	buildTree = func(parent string, level int) {
		p := strings.TrimRight(parent, "/")
		for f := 0; f < nFpL; f++ {
			fs[fmt.Sprintf("%s/f%d.txt", p, f)] = treeEntry{size: int64(f + 1)}
		}
		if level >= depth {
			return
		}
		for d := 0; d < nDirs; d++ {
			child := fmt.Sprintf("%s/d%d", p, d)
			fs[child] = treeEntry{isDir: true}
			buildTree(child, level+1)
		}
	}
	buildTree("/", 0)

	totalFiles := 0
	for _, e := range fs {
		if !e.isDir {
			totalFiles++
		}
	}

	// Must use real TCP — net.Pipe is synchronous and blocks Write until peer reads.
	client, server := newTestConn(t)

	// maxDepthCh receives the maximum observed server rx-queue depth.
	// Sequential client: always 0 (next request sent only after previous reply).
	// Pipelined client:  > 0 (requests arrive while server is still processing).
	maxDepthCh := make(chan int, 1)
	go func() {
		defer server.Close()
		type req struct {
			op, sn uint64
			name   string
		}
		rxCh := make(chan req, 512)
		go func() {
			defer close(rxCh)
			for {
				op, args, sn, err := readOneRequest(server)
				if err != nil {
					return
				}
				rxCh <- req{op, sn, pathArg(args)}
			}
		}()
		maxQueue := 0
		for r := range rxCh {
			if q := len(rxCh); q > maxQueue {
				maxQueue = q
			}
			time.Sleep(rtt)
			respOp, payload := buildResponse(fs, r.op, r.name)
			if err := writeFrame(server, r.sn, respOp, payload); err != nil {
				break
			}
		}
		maxDepthCh <- maxQueue
	}()

	svc := New(client)
	items, err := svc.List("/", true)
	// Close client so server rx goroutine sees EOF and closes rxCh,
	// allowing the server goroutine to send maxQueue and exit.
	client.Close()
	maxDepth := <-maxDepthCh

	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(items) != totalFiles {
		t.Errorf("expected %d files, got %d", totalFiles, len(items))
	}
	if maxDepth == 0 {
		t.Errorf("server rx queue max depth = 0 — client is not pipelining")
	}
	t.Logf("server rx queue max depth: %d (sequential=0, pipelined>0)", maxDepth)
}
