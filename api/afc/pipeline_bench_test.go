package afc

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"
)

// sequentialList is a reference implementation: strictly send one request,
// wait for the reply, then send the next. No goroutines, no pipelining.
func sequentialList(svc *Service, dir string) ([]*FileStat, error) {
	type command struct {
		code uint64
		name string
	}
	queue := []command{{code: opReadDir, name: dir}}
	var out []*FileStat
	for len(queue) > 0 {
		cmd := queue[0]
		queue = queue[1:]
		req := make([]byte, len(cmd.name)+1)
		copy(req, cmd.name)
		sn, err := svc.send(cmd.code, &request{Args: req})
		if err != nil {
			return nil, err
		}
		var opcode uint64
		rsp, err := svc.recv(&opcode, sn, false)
		if err != nil {
			if cmd.code == opGetFileInfo {
				continue
			}
			return nil, err
		}
		if opcode != opData {
			continue
		}
		raw := rsp.(*bytes.Buffer).Bytes()
		switch cmd.code {
		case opGetFileInfo:
			fst := &FileStat{Name: cmd.name}
			if e := svc.parse(raw, fst); e == nil {
				if fst.IsDir() {
					queue = append(queue, command{code: opReadDir, name: cmd.name})
				} else {
					out = append(out, fst)
				}
			}
		case opReadDir:
			p := 0
			for i := range raw {
				if raw[i] == 0 {
					ent := string(raw[p:i])
					if ent != "." && ent != ".." && ent != "" {
						queue = append(queue, command{code: opGetFileInfo, name: path.Join(cmd.name, ent)})
					}
					p = i + 1
				}
			}
		}
	}
	return out, nil
}

func TestPipelineVsSequential(t *testing.T) {
	const (
		depth = 4
		nDirs = 3
		nFpL  = 2
		rtt   = 5 * time.Millisecond
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

	type serverStats struct {
		elapsed  time.Duration
		maxGap   time.Duration // max gap between consecutive request arrivals
		totalGap time.Duration // sum of all gaps (= time server was idle waiting for client)
	}

	startServer := func(t *testing.T) (*Service, chan serverStats) {
		t.Helper()
		c, server := newTestConn(t)
		statsCh := make(chan serverStats, 1)
		go func() {
			defer server.Close()
			rxCh := make(chan struct {
				op, sn uint64
				name   string
				at     time.Time
			}, 512)
			go func() {
				defer close(rxCh)
				for {
					op, args, sn, err := readOneRequest(server)
					if err != nil {
						return
					}
					rxCh <- struct {
						op, sn uint64
						name   string
						at     time.Time
					}{op, sn, pathArg(args), time.Now()}
				}
			}()
			var stats serverStats
			start := time.Now()
			var lastAt time.Time
			for r := range rxCh {
				if !lastAt.IsZero() {
					gap := r.at.Sub(lastAt)
					stats.totalGap += gap
					if gap > stats.maxGap {
						stats.maxGap = gap
					}
				}
				lastAt = r.at
				time.Sleep(rtt)
				respOp, payload := buildResponse(fs, r.op, r.name)
				if err := writeFrame(server, r.sn, respOp, payload); err != nil {
					break
				}
			}
			stats.elapsed = time.Since(start)
			statsCh <- stats
		}()
		return New(c), statsCh
	}

	// sequential
	seqSvc, seqStatsCh := startServer(t)
	seqSvc.gm.Lock()
	seqItems, err := sequentialList(seqSvc, "/")
	seqSvc.gm.Unlock()
	seqSvc.conn.Close()
	seqStats := <-seqStatsCh
	if err != nil {
		t.Fatalf("sequential List error: %v", err)
	}

	// pipelined
	pipeSvc, pipeStatsCh := startServer(t)
	pipeItems, err := pipeSvc.List("/", 0)
	pipeSvc.conn.Close()
	pipeStats := <-pipeStatsCh
	if err != nil {
		t.Fatalf("pipeline List error: %v", err)
	}

	if len(seqItems) != totalFiles {
		t.Errorf("sequential: expected %d files, got %d", totalFiles, len(seqItems))
	}
	if len(pipeItems) != totalFiles {
		t.Errorf("pipeline:   expected %d files, got %d", totalFiles, len(pipeItems))
	}

	t.Logf("files=%d  rtt=%v", totalFiles, rtt)
	t.Logf("sequential: elapsed=%v  server_idle=%v  max_gap=%v",
		seqStats.elapsed.Round(time.Millisecond),
		seqStats.totalGap.Round(time.Millisecond),
		seqStats.maxGap.Round(time.Millisecond))
	t.Logf("pipelined:  elapsed=%v  server_idle=%v  max_gap=%v",
		pipeStats.elapsed.Round(time.Millisecond),
		pipeStats.totalGap.Round(time.Millisecond),
		pipeStats.maxGap.Round(time.Millisecond))

	// Pipeline should reduce server idle time significantly
	if pipeStats.totalGap >= seqStats.totalGap {
		t.Errorf("pipeline server idle time (%v) >= sequential (%v) — no benefit",
			pipeStats.totalGap.Round(time.Millisecond),
			seqStats.totalGap.Round(time.Millisecond))
	}
}
