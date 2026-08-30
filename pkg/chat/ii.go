//go:build darwin || linux

package chat

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var (
	chatLineRE   = regexp.MustCompile(`^(\d+) <([^>]*)> (.*)$`)
	chatAnyRE    = regexp.MustCompile(`^(\d+) (.*)$`)
	chatNamesRE  = regexp.MustCompile(`^\d+ = (\S+) (.*)$`)
	rawNamesRE   = regexp.MustCompile(`^\d+ :\S+ 353 \S+ = (\S+) :(.*)$`)
	rawMessageRE = regexp.MustCompile(`^(\d+) :([^! ]+)!\S+ PRIVMSG \S+ :(.*)$`)
)

type II struct {
	Paths Paths
	Log   *os.File
	Cmd   *exec.Cmd
}

func (i *II) Start(session ChatSession, executable string) error {
	if executable == "" {
		executable = "ii"
	}
	if _, err := exec.LookPath(executable); err != nil {
		return fmt.Errorf("ii is unavailable: %w", err)
	}
	if err := os.MkdirAll(i.Paths.Conversations, 0700); err != nil {
		return err
	}
	log, err := os.OpenFile(i.Paths.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	i.Log = log
	cmd := exec.Command(executable, "-s", session.Host, "-p", strconv.Itoa(session.Port), "-n", session.Nick, "-f", session.Role, "-i", i.Paths.Conversations)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return err
	}
	i.Cmd = cmd
	return nil
}

func (i *II) Close() error {
	if i.Log != nil {
		return i.Log.Close()
	}
	return nil
}

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
	// Opening and closing a writer can make ii observe EOF on its channel FIFO
	// and reconnect. Inspect the existing descriptor instead of touching the
	// transport path.
	out, _ := exec.Command("lsof", "-t", path).Output()
	return strings.TrimSpace(string(out)) != ""
}

func Tail(path string, offset int64) ([]string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, offset, nil
		}
		return nil, offset, err
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() < offset {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, offset, err
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	return lines, offset + int64(len(data)), nil
}

func ParseNames(line string) (string, []string, bool) {
	m := chatNamesRE.FindStringSubmatch(line)
	if m != nil {
		return m[1], strings.Fields(m[2]), true
	}
	m = rawNamesRE.FindStringSubmatch(line)
	if m != nil {
		return m[1], strings.Fields(m[2]), true
	}
	return "", nil, false
}

func ParseLine(line string) (timestamp int64, nick, text string, ok bool) {
	if m := chatLineRE.FindStringSubmatch(line); m != nil {
		ts, err := strconv.ParseInt(m[1], 10, 64)
		return ts, m[2], m[3], err == nil
	}
	if m := chatAnyRE.FindStringSubmatch(line); m != nil {
		if raw := rawMessageRE.FindStringSubmatch(line); raw != nil {
			ts, err := strconv.ParseInt(raw[1], 10, 64)
			return ts, raw[2], raw[3], err == nil
		}
		ts, err := strconv.ParseInt(m[1], 10, 64)
		return ts, "", m[2], err == nil
	}
	return 0, "", line, false
}
