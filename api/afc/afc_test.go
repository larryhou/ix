package afc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"testing"
)

// mockServer simulates the device-side AFC protocol over a net.Pipe connection.
// It reads one request at a time and dispatches to the provided handler.
type mockServer struct {
	conn net.Conn
	bo   binary.ByteOrder
	sn   uint64 // last seen request sn, echoed back
}

func newMockServer(conn net.Conn) *mockServer {
	return &mockServer{conn: conn, bo: binary.LittleEndian}
}

// readRequest reads one AFC request frame and returns (opcode, args, sn, error).
func (s *mockServer) readRequest() (op uint64, args []byte, sn uint64, err error) {
	hdr := make([]byte, headerSize)
	if _, err = readFull(s.conn, hdr); err != nil {
		return
	}
	pktLen := s.bo.Uint64(hdr[8:16])
	hdrLen := s.bo.Uint64(hdr[16:24])
	sn = s.bo.Uint64(hdr[24:32])
	op = s.bo.Uint64(hdr[32:40])
	argsLen := hdrLen - headerSize
	if argsLen > 0 {
		args = make([]byte, argsLen)
		if _, err = readFull(s.conn, args); err != nil {
			return
		}
	}
	// drain body if any
	bodyLen := int(pktLen - hdrLen)
	if bodyLen > 0 {
		drain := make([]byte, bodyLen)
		_, err = readFull(s.conn, drain)
	}
	s.sn = sn
	return
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// sendData writes an opData response with the given payload.
func (s *mockServer) sendData(payload []byte) error {
	return s.sendFrame(opData, payload)
}

// sendStatus writes an opStatus response with the given retcode.
func (s *mockServer) sendStatus(rc Retcode) error {
	b := make([]byte, 8)
	s.bo.PutUint64(b, uint64(rc))
	return s.sendFrame(opStatus, b)
}

func (s *mockServer) sendFrame(op uint64, payload []byte) error {
	buf := &bytes.Buffer{}
	rsv := make([]byte, 8)

	copy(rsv, magic)
	buf.Write(rsv)

	length := uint64(headerSize + len(payload))
	s.bo.PutUint64(rsv, length)
	buf.Write(rsv) // packet length

	s.bo.PutUint64(rsv, uint64(headerSize))
	buf.Write(rsv) // header length

	s.bo.PutUint64(rsv, s.sn)
	buf.Write(rsv) // sn (echo)

	s.bo.PutUint64(rsv, op)
	buf.Write(rsv) // opcode

	buf.Write(payload)
	_, err := s.conn.Write(buf.Bytes())
	return err
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

// --- test helpers ---

// treeEntry describes one node in a fake filesystem tree.
type treeEntry struct {
	isDir bool
	size  int64
}

// runMockAFC sets up a net.Pipe, starts a mock AFC server in a goroutine
// that responds according to `fs`, and returns the client-side *Service.
//
// The mock server handles:
//   - opReadDir  → returns child names from fs
//   - opGetFileInfo → returns stat for a known path; PermDenied for unknown
func runMockAFC(t *testing.T, fs map[string]treeEntry) *Service {
	t.Helper()
	client, server := net.Pipe()

	go func() {
		defer server.Close()
		srv := newMockServer(server)
		for {
			op, args, _, err := srv.readRequest()
			if err != nil {
				return
			}
			// extract null-terminated path from args
			end := bytes.IndexByte(args, 0)
			name := string(args)
			if end >= 0 {
				name = string(args[:end])
			}

			switch op {
			case opReadDir:
				entry, ok := fs[name]
				if !ok || !entry.isDir {
					srv.sendStatus(retObjectNotFound)
					continue
				}
				prefix := name
				if prefix != "/" {
					prefix = prefix + "/"
				}
				var children []string
				for k := range fs {
					if k == name {
						continue
					}
					// direct child: k starts with prefix and has no further slash
					if len(k) <= len(prefix) {
						continue
					}
					if k[:len(prefix)] != prefix {
						continue
					}
					rest := k[len(prefix):]
					if len(rest) == 0 {
						continue
					}
					// no slash in rest means direct child
					hasSlash := false
					for _, c := range rest {
						if c == '/' {
							hasSlash = true
							break
						}
					}
					if !hasSlash {
						children = append(children, rest)
					}
				}
				srv.sendData(buildDirListing(children...))

			case opGetFileInfo:
				entry, ok := fs[name]
				if !ok {
					srv.sendStatus(retPermDenied)
					continue
				}
				ifmt := "S_IFREG"
				if entry.isDir {
					ifmt = "S_IFDIR"
				}
				payload := buildKV(
					"st_ifmt", ifmt,
					"st_size", fmt.Sprintf("%d", entry.size),
					"st_nlink", "1",
					"st_blocks", "0",
				)
				srv.sendData(payload)

			default:
				srv.sendStatus(retOpNotSupported)
			}
		}
	}()

	return New(client)
}

// --- tests ---

// TestListFlat verifies a single directory with only files.
func TestListFlat(t *testing.T) {
	fs := map[string]treeEntry{
		"/":         {isDir: true},
		"/a.txt":    {size: 100},
		"/b.mp4":    {size: 200},
		"/c.jpg":    {size: 300},
	}
	svc := runMockAFC(t, fs)
	items, err := svc.List("/", false)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	// verify names
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	sort.Strings(names)
	want := []string{"/a.txt", "/b.mp4", "/c.jpg"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("item[%d]: got %q want %q", i, names[i], w)
		}
	}
}

// TestListNonRecursive verifies that recursive=false does not descend into subdirs.
func TestListNonRecursive(t *testing.T) {
	fs := map[string]treeEntry{
		"/":             {isDir: true},
		"/sub":          {isDir: true},
		"/sub/deep.txt": {size: 42},
		"/root.txt":     {size: 10},
	}
	svc := runMockAFC(t, fs)
	items, err := svc.List("/", false)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	// should see /sub (as dir) and /root.txt, not /sub/deep.txt
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

// TestListRecursive verifies BFS correctly descends into all subdirs.
func TestListRecursive(t *testing.T) {
	fs := map[string]treeEntry{
		"/":              {isDir: true},
		"/a":             {isDir: true},
		"/a/b":           {isDir: true},
		"/a/b/file1.txt": {size: 1},
		"/a/file2.txt":   {size: 2},
		"/file3.txt":     {size: 3},
	}
	svc := runMockAFC(t, fs)
	items, err := svc.List("/", true)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	// dirs are not returned, only files
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

// TestListPermDeniedOnStat verifies that a PermDenied on opGetFileInfo is
// skipped rather than aborting the entire walk (mirrors VendDocuments behaviour).
func TestListPermDeniedOnStat(t *testing.T) {
	fs := map[string]treeEntry{
		"/":         {isDir: true},
		"/ok.txt":   {size: 99},
		// "/secret" is intentionally absent → mock returns PermDenied
	}
	// inject a child name that the mock cannot stat
	overrideFs := map[string]treeEntry{
		"/":         {isDir: true},
		"/ok.txt":   {size: 99},
	}

	client, server := net.Pipe()
	go func() {
		defer server.Close()
		srv := newMockServer(server)
		callCount := 0
		for {
			op, args, _, err := srv.readRequest()
			if err != nil {
				return
			}
			end := bytes.IndexByte(args, 0)
			name := string(args)
			if end >= 0 {
				name = string(args[:end])
			}
			switch op {
			case opReadDir:
				if name == "/" {
					// return two children, one of which will fail stat
					srv.sendData(buildDirListing("ok.txt", "secret"))
				} else {
					srv.sendStatus(retObjectNotFound)
				}
			case opGetFileInfo:
				callCount++
				if entry, ok := overrideFs[name]; ok {
					ifmt := "S_IFREG"
					if entry.isDir {
						ifmt = "S_IFDIR"
					}
					srv.sendData(buildKV(
						"st_ifmt", ifmt,
						"st_size", fmt.Sprintf("%d", entry.size),
						"st_nlink", "1",
						"st_blocks", "0",
					))
				} else {
					srv.sendStatus(retPermDenied)
				}
			}
		}
	}()

	svc := New(client)
	items, err := svc.List("/", false)
	if err != nil {
		t.Fatalf("List should not fail on PermDenied stat, got: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 accessible item, got %d: %v", len(items), items)
	}
	if items[0].Name != "/ok.txt" {
		t.Errorf("expected /ok.txt, got %q", items[0].Name)
	}
	_ = fs
}

// TestListSizes verifies that file sizes are parsed correctly.
func TestListSizes(t *testing.T) {
	fs := map[string]treeEntry{
		"/":        {isDir: true},
		"/big.mp4": {size: 1_500_000_000},
		"/tiny.db": {size: 42},
	}
	svc := runMockAFC(t, fs)
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

// TestListEmptyDir verifies that an empty directory returns no items.
func TestListEmptyDir(t *testing.T) {
	fs := map[string]treeEntry{
		"/": {isDir: true},
	}
	svc := runMockAFC(t, fs)
	items, err := svc.List("/", true)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}
