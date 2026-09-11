// Package dal is the data-access layer for chat: on-disk state, cursors, and
// the OS-level primitives shared by chat sessions.
package dal

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/alswl/chatta/pkg/common"
)

type Paths struct {
	Home, State, Cursors, Lock, Log string
	Conversations                   string
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
	return Paths{Home: home, State: filepath.Join(home, "state.json"), Cursors: filepath.Join(home, "cursors.json"), Lock: filepath.Join(home, ".lock"), Log: filepath.Join(home, "ii.log"), Conversations: filepath.Join(home, "irc")}
}

func CursorPath(home, sessionID string) string {
	if sessionID == "" {
		return filepath.Join(home, "cursors.json")
	}
	sum := sha256.Sum256([]byte(sessionID))
	return filepath.Join(home, "cursors-"+hex.EncodeToString(sum[:6])+".json")
}
func DiscoverConversations(home string) ([]common.Conversation, error) {
	entries, err := os.ReadDir(filepath.Join(home, "irc"))
	if err != nil {
		if os.IsNotExist(err) {
			return []common.Conversation{}, nil
		}
		return nil, err
	}
	out := make([]common.Conversation, 0)
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, common.Conversation{Name: e.Name(), SourceKind: sourceKind(e.Name()), TranscriptPath: filepath.Join(home, "irc", e.Name(), "out")})
		}
	}
	return out, nil
}
func sourceKind(name string) string {
	if len(name) > 0 && name[0] == '#' {
		return "channel"
	}
	return "direct"
}
