//go:build darwin

package rsd

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// hijack temporarily pauses the system remoted process on macOS while
// connecting to RSD, preventing it from racing on the same port.
func hijack(f func() error) error {
	guard.Lock()
	defer guard.Unlock()

	buf := &bytes.Buffer{}
	cmd := exec.Command(`ps`, `-ax`, `-opid,comm`)
	cmd.Stdout = buf
	cmd.Run()

	pid := 0
	for k := bufio.NewScanner(buf); k.Scan(); {
		if proc := strings.TrimSpace(k.Text()); strings.HasSuffix(proc, `/usr/libexec/remoted`) {
			if i := strings.IndexByte(proc, ' '); i > 0 {
				pid, _ = strconv.Atoi(proc[:i])
				break
			}
		}
	}

	if pid == 0 {
		return f()
	}

	err := unix.Kill(pid, unix.SIGSTOP)
	defer func(err error) {
		if err == nil {
			unix.Kill(pid, unix.SIGCONT)
		}
	}(err)
	return f()
}
