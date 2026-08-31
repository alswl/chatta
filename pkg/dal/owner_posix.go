//go:build darwin || linux

package dal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alswl/chatta/pkg/common"
)

const testOwnerPIDEnv = "CHATTA_CHAT_TEST_OWNER_PID"

// FindAgentOwner walks the command ancestry looking for the invoking agent
// runtime. Shell wrappers are deliberately skipped because they outlive only
// the individual command, not the interactive agent session.
func FindAgentOwner() (common.OwnerBinding, error) {
	if owner, configured, err := testOwner(); configured {
		return owner, err
	}
	pid := os.Getppid()
	for i := 0; i < 32 && pid > 1; i++ {
		cmdline, ppid, err := processCommand(pid)
		if err != nil {
			return common.OwnerBinding{}, err
		}
		base := filepath.Base(strings.Fields(cmdline)[0])
		base = strings.TrimLeft(base, "-")
		if strings.Contains(base, "claude") || strings.Contains(base, "codex") {
			return common.OwnerBinding{PID: pid, StartFingerprint: ProcessStart(pid), Runtime: base}, nil
		}
		pid = ppid
	}
	return common.OwnerBinding{}, fmt.Errorf("could not find the Claude/Codex session that owns this client")
}

// testOwner is an explicit opt-in for external black-box verification. It
// preserves the normal PID/start-fingerprint ownership checks while allowing
// a fixture to provide a durable parent process without impersonating a
// Codex or Claude runtime.
func testOwner() (common.OwnerBinding, bool, error) {
	raw, configured := os.LookupEnv(testOwnerPIDEnv)
	if !configured || raw == "" {
		return common.OwnerBinding{}, false, nil
	}
	pid, err := strconv.Atoi(raw)
	if err != nil || pid <= 1 {
		return common.OwnerBinding{}, true, fmt.Errorf("invalid %s %q", testOwnerPIDEnv, raw)
	}
	owner := common.OwnerBinding{PID: pid, StartFingerprint: ProcessStart(pid), Runtime: "verification-fixture"}
	if !ProcessAlive(owner) {
		return common.OwnerBinding{}, true, fmt.Errorf("verification owner %d is not running", pid)
	}
	return owner, true, nil
}

func processCommand(pid int) (string, int, error) {
	out, err := runShort("ps", "-o", "ppid=,command=", "-p", strconv.Itoa(pid))
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
	out, err := runShort("ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func ProcessAlive(binding common.OwnerBinding) bool {
	if binding.PID <= 0 || binding.StartFingerprint == "" {
		return false
	}
	if _, err := runShort("kill", "-0", strconv.Itoa(binding.PID)); err != nil {
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

// SameOwner reports whether two bindings share a PID and start fingerprint.
// It does not check liveness — pair with ProcessAlive when that matters.
func SameOwner(left, right common.OwnerBinding) bool {
	return left.PID > 0 && left.PID == right.PID && left.StartFingerprint != "" && left.StartFingerprint == right.StartFingerprint
}

// OwnerSessionID derives a session identifier for an owner binding,
// preferring the runtime's own session id when the calling process
// exposes one.
func OwnerSessionID(owner common.OwnerBinding) string {
	if id := RuntimeSessionID(); id != "" {
		return id
	}
	return fmt.Sprintf("pid:%d:%s", owner.PID, owner.StartFingerprint)
}

func InvocationSessionID(fallback string) string {
	if id := RuntimeSessionID(); id != "" {
		return id
	}
	if owner, err := FindAgentOwner(); err == nil {
		return OwnerSessionID(owner)
	}
	return fallback
}
