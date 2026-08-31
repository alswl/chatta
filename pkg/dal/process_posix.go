//go:build darwin || linux

package dal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func LockHome(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func StartProcess(name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, nil
}

// runShort runs a short-lived helper process (ps, lsof, kill) with a bounded
// timeout so a stuck subprocess cannot wedge a health check or self-heal
// path forever.
func runShort(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

func ProcessCommand(pid int) string {
	out, _ := runShort("ps", "-o", "command=", "-p", strconv.Itoa(pid))
	return strings.TrimSpace(string(out))
}

func IsSupervisor(pid int) bool {
	command := ProcessCommand(pid)
	return strings.Contains(command, "chatta") && strings.Contains(command, "_supervise")
}

func IsVerifiedSupervisor(pid int, startFingerprint string) bool {
	return pid > 0 && startFingerprint != "" && IsSupervisor(pid) && ProcessStart(pid) == startFingerprint
}

func SignalProcessGroup(pid int, sig unix.Signal) error {
	if pid <= 0 {
		return nil
	}
	if err := unix.Kill(-pid, sig); err != nil && err != unix.ESRCH {
		return err
	}
	return nil
}

func StopVerifiedSupervisor(pid int, startFingerprint string) error {
	if !IsVerifiedSupervisor(pid, startFingerprint) {
		return nil
	}
	if err := SignalProcessGroup(pid, unix.SIGTERM); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !IsVerifiedSupervisor(pid, startFingerprint) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := SignalProcessGroup(pid, unix.SIGKILL); err != nil {
		return err
	}
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !IsVerifiedSupervisor(pid, startFingerprint) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("supervisor pid %d did not exit", pid)
}

func ReapStrayII(conversations string) error {
	pids, err := IIPIDs(conversations)
	if err != nil {
		return err
	}
	for _, pid := range pids {
		fingerprint := ProcessStart(pid)
		if fingerprint == "" {
			continue
		}
		if err := stopVerifiedII(pid, fingerprint, conversations); err != nil {
			return err
		}
	}
	return nil
}

// IIPIDs lists the PIDs of ii processes attached to this client's
// conversations directory, without signalling them.
func IIPIDs(conversations string) ([]int, error) {
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

func stopVerifiedII(pid int, fingerprint, conversations string) error {
	if ProcessStart(pid) != fingerprint || !isIIForHome(ProcessCommand(pid), conversations) {
		return nil
	}
	if err := unix.Kill(pid, unix.SIGTERM); err != nil && err != unix.ESRCH {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ProcessStart(pid) != fingerprint || !isIIForHome(ProcessCommand(pid), conversations) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := unix.Kill(pid, unix.SIGKILL); err != nil && err != unix.ESRCH {
		return err
	}
	return nil
}
