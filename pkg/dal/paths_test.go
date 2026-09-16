package dal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorktreeHomeDeterministicAndDistinct(t *testing.T) {
	root := t.TempDir()
	a := WorktreeHome(root, "/repo/worktrees/a")
	require.Equal(t, a, WorktreeHome(root, "/repo/worktrees/a"), "not deterministic")
	require.NotEqual(t, a, WorktreeHome(root, "/repo/worktrees/b"), "collision")
	require.True(t, strings.HasPrefix(filepath.Base(a), "a-"), "missing worktree basename: %s", a)
}

func TestWorktreeHomeResolvesSymlink(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "actual-worktree")
	require.NoError(t, os.Mkdir(worktree, 0755))
	link := filepath.Join(root, "worktree-link")
	require.NoError(t, os.Symlink(worktree, link))
	require.Equal(t,
		WorktreeHome(filepath.Join(root, "clients"), worktree),
		WorktreeHome(filepath.Join(root, "clients"), link),
		"a symlinked worktree must resolve to the same home")
}
func TestResolvePaths(t *testing.T) {
	p := ResolvePaths("/tmp/chat-home")
	require.Equal(t, filepath.Join(p.Home, "state.json"), p.State)
	require.Equal(t, filepath.Join(p.Home, "cursors.json"), p.Cursors)
	require.Equal(t, filepath.Join(p.Home, ".lock"), p.Lock)
}
