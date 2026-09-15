// Package dal is the data-access layer for chat: on-disk state, cursors, and
// the OS-level primitives shared by chat sessions.
package dal

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

type Paths struct {
	Home, State, Cursors, Lock, Log string
	ControlSock, Messages           string
}

func WorktreeHome(root, worktree string) string {
	if root == "" {
		if h, e := os.UserHomeDir(); e == nil {
			root = filepath.Join(h, ".irc-agent", "clients")
		}
	}
	if resolved, err := filepath.EvalSymlinks(worktree); err == nil {
		worktree = resolved
	}
	sum := sha256.Sum256([]byte(worktree))
	base := filepath.Base(filepath.Clean(worktree))
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "worktree"
	}
	return filepath.Join(root, base+"-"+hex.EncodeToString(sum[:8]))
}
func ResolvePaths(home string) Paths {
	return Paths{Home: home, State: filepath.Join(home, "state.json"), Cursors: filepath.Join(home, "cursors.json"), Lock: filepath.Join(home, ".lock"), Log: filepath.Join(home, "supervisor.log"), ControlSock: filepath.Join(home, "control.sock"), Messages: filepath.Join(home, "messages.jsonl")}
}

func CursorPath(home, sessionID string) string {
	if sessionID == "" {
		return filepath.Join(home, "cursors.json")
	}
	sum := sha256.Sum256([]byte(sessionID))
	return filepath.Join(home, "cursors-"+hex.EncodeToString(sum[:6])+".json")
}
