//go:build darwin || linux

package ii

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alswl/chatta/pkg/dal"
	"golang.org/x/sys/unix"
)

const execTimeout = 5 * time.Second

func runShort(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

func ReapStray(conversations string) error {
	pids, err := PIDs(conversations)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		fingerprint := dal.ProcessStart(pid)
		if fingerprint == "" {
			continue
		}
		if err := stopVerified(pid, fingerprint, conversations); err != nil {
			return err
		}
	}
	return nil
}

func PIDs(conversations string) ([]int, error) {
	out, err := runShort("ps", "-axo", "pid=,command=")
	if err != nil {
		return nil, err
	}
	pids := make([]int, 0)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		command := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), fields[0]))
		if err != nil || !isIIForHome(command, conversations) {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func isIIForHome(command, conversations string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 || filepath.Base(fields[0]) != "ii" {
		return false
	}
	for index := 1; index+1 < len(fields); index++ {
		if fields[index] == "-i" && fields[index+1] == conversations {
			return true
		}
	}
	return false
}

func stopVerified(pid int, fingerprint, conversations string) error {
	if dal.ProcessStart(pid) != fingerprint || !isIIForHome(dal.ProcessCommand(pid), conversations) {
		return nil
	}
	if err := unix.Kill(pid, unix.SIGTERM); err != nil && err != unix.ESRCH {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if dal.ProcessStart(pid) != fingerprint || !isIIForHome(dal.ProcessCommand(pid), conversations) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := unix.Kill(pid, unix.SIGKILL); err != nil && err != unix.ESRCH {
		return err
	}
	return nil
}
