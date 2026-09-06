package afc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
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

// TestListPipeline verifies that send and recv truly run concurrently.
//
// Server behaviour (strictly sequential, as AFC requires):
//   1. Collect ALL requests into a queue (rx goroutine, no delay)
//   2. Once the first batch of requests stops arriving (idle for idleWindow),
//      reply to each one in order with a small perReplyDelay.
//
// A sequential client sends request-1, then blocks waiting for reply-1 before
// sending request-2.  The server never sees more than 1 request at a time, so
// it replies to request-1, client sends request-2, etc.
// Total time ≈ N * (idleWindow + perReplyDelay).
//
// A pipelined client sends ALL requests before reading any reply.
// The server sees all N requests arrive quickly, then replies to them in order.
// Total time ≈ idleWindow + N * perReplyDelay.
//
// We assert: elapsed < N/2 * (idleWindow + perReplyDelay)
// which is impossible for a sequential client but easy for a pipelined one.
func TestListPipeline(t *testing.T) {
	fs := map[string]treeEntry{
		"/": {isDir: true},
	}
	// 1 ReadDir + nFiles GetFileInfo = nFiles+1 total requests in first wave
	const nFiles = 20
	for i := 0; i < nFiles; i++ {
		fs[fmt.Sprintf("/file%02d.txt", i)] = treeEntry{size: int64(i + 1)}
	}

	const (
		idleWindow   = 30 * time.Millisecond // how long server waits for more requests
		perReplyDelay = 2 * time.Millisecond  // delay per reply (simulate processing)
		nRequests    = nFiles + 1             // ReadDir(/) + nFiles * GetFileInfo
	)

	// sequential total ≈ nRequests * (idleWindow + perReplyDelay) ≈ 640ms
	// pipelined total  ≈ idleWindow + nRequests * perReplyDelay   ≈  72ms
	sequentialBound := time.Duration(nRequests) * (idleWindow + perReplyDelay) / 2

	client, server := net.Pipe()

	go func() {
		defer server.Close()

		type req struct {
			op, sn uint64
			name   string
		}

		// rx goroutine: reads requests as fast as they arrive
		reqCh := make(chan req, 256)
		go func() {
			defer close(reqCh)
			for {
				op, args, sn, err := readOneRequest(server)
				if err != nil {
					return
				}
				reqCh <- req{op: op, sn: sn, name: pathArg(args)}
			}
		}()

		// tx loop: accumulate requests until the wire goes idle for idleWindow,
		// then flush all pending replies in order. Repeat until rx closes.
		var pending []req
		idle := time.NewTimer(idleWindow)
		for {
			select {
			case r, ok := <-reqCh:
				if !ok {
					// connection closed — flush remainder and exit
					for _, r := range pending {
						time.Sleep(perReplyDelay)
						respOp, payload := buildResponse(fs, r.op, r.name)
						writeFrame(server, r.sn, respOp, payload)
					}
					return
				}
				pending = append(pending, r)
				if !idle.Stop() {
					select { case <-idle.C: default: }
				}
				idle.Reset(idleWindow)

			case <-idle.C:
				// wire went quiet — flush accumulated batch
				for _, r := range pending {
					time.Sleep(perReplyDelay)
					respOp, payload := buildResponse(fs, r.op, r.name)
					if err := writeFrame(server, r.sn, respOp, payload); err != nil {
						return
					}
				}
				pending = pending[:0]
				idle.Reset(idleWindow)
			}
		}
	}()

	svc := New(client)
	start := time.Now()
	items, err := svc.List("/", true)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(items) != nFiles {
		t.Errorf("expected %d files, got %d", nFiles, len(items))
	}
	if elapsed >= sequentialBound {
		t.Errorf("elapsed %v >= sequentialBound %v — implementation is not pipelined\n"+
			"  sequential≈%v  pipelined≈%v",
			elapsed, sequentialBound,
			time.Duration(nRequests)*(idleWindow+perReplyDelay),
			idleWindow+time.Duration(nRequests)*perReplyDelay)
	}
	t.Logf("elapsed %v  sequential≈%v  pipelined≈%v",
		elapsed,
		time.Duration(nRequests)*(idleWindow+perReplyDelay),
		idleWindow+time.Duration(nRequests)*perReplyDelay)
}
