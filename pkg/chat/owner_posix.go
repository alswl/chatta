//go:build darwin || linux

package chat

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// FindAgentOwner walks the command ancestry looking for the invoking agent
// runtime. Shell wrappers are deliberately skipped because they outlive only
// the individual command, not the interactive agent session.
func FindAgentOwner() (OwnerBinding, error) {
	pid := os.Getppid()
	for i := 0; i < 32 && pid > 1; i++ {
		cmdline, ppid, err := processCommand(pid)
		if err != nil {
			return OwnerBinding{}, err
		}
		base := filepath.Base(strings.Fields(cmdline)[0])
		base = strings.TrimLeft(base, "-")
		if strings.Contains(base, "claude") || strings.Contains(base, "codex") {
			return OwnerBinding{PID: pid, StartFingerprint: ProcessStart(pid), Runtime: base}, nil
		}
		pid = ppid
	}
	return OwnerBinding{}, fmt.Errorf("could not find the Claude/Codex session that owns this client")
}

func processCommand(pid int) (string, int, error) {
	out, err := exec.Command("ps", "-o", "ppid=,command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", 0, err
	}
	parts := strings.Fields(string(out))
	if len(parts) < 2 {
		return "", 0, fmt.Errorf("cannot inspect process %d", pid)
	}
	ppid, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", 0, err
	}
	trimmed := strings.TrimSpace(string(out))
	if idx := strings.IndexAny(trimmed, " \t"); idx >= 0 {
		return strings.TrimSpace(trimmed[idx:]), ppid, nil
	}
	return trimmed, ppid, nil
}

func ProcessStart(pid int) string {
	out, err := exec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ProcessAlive(binding OwnerBinding) bool {
	if binding.PID <= 0 || binding.StartFingerprint == "" {
		return false
	}
	if err := exec.Command("kill", "-0", strconv.Itoa(binding.PID)).Run(); err != nil {
		return false
	}
	return ProcessStart(binding.PID) == binding.StartFingerprint
}

func RuntimeSessionID() string {
	for _, key := range []string{"AGENT_CHAT_SESSION", "CLAUDE_CODE_SESSION_ID", "CODEX_SESSION_ID"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func sameOwner(left, right OwnerBinding) bool {
	return left.PID > 0 && left.PID == right.PID && left.StartFingerprint != "" && left.StartFingerprint == right.StartFingerprint
}

func ownerSessionID(owner OwnerBinding) string {
	if id := RuntimeSessionID(); id != "" {
		return id
	}
	return fmt.Sprintf("pid:%d:%s", owner.PID, owner.StartFingerprint)
}
