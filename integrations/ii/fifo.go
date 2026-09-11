//go:build darwin || linux

package ii

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func WriteFIFO(path, line string, retries int) error {
	for attempt := 0; attempt <= retries; attempt++ {
		fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0)
		if err == nil {
			f := os.NewFile(uintptr(fd), filepath.Base(path))
			_, writeErr := io.WriteString(f, line+"\n")
			closeErr := f.Close()
			if writeErr == nil {
				writeErr = closeErr
			}
			if writeErr == nil {
				return nil
			}
			err = writeErr
		}
		if (errors.Is(err, unix.ENXIO) || errors.Is(err, unix.ENOENT) || errors.Is(err, syscall.EPIPE)) && attempt < retries {
			time.Sleep(time.Second)
			continue
		}
		return err
	}
	return unix.ENXIO
}

func FIFOReader(path string) bool {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	out, err := runShort(lsofExecutable(), "-n", "-P", "-F", "n")
	return err == nil && lsofHasPath(out, path)
}

func lsofExecutable() string {
	if path, err := exec.LookPath("lsof"); err == nil {
		return path
	}
	for _, path := range []string{"/usr/sbin/lsof", "/usr/bin/lsof"} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return "lsof"
}

func lsofHasPath(output []byte, path string) bool {
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimPrefix(line, "n") == path {
			return true
		}
	}
	return false
}
