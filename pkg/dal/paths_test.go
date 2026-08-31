package dal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeHomeDeterministicAndDistinct(t *testing.T) {
	root := t.TempDir()
	a := WorktreeHome(root, "/repo/worktrees/a")
	if a != WorktreeHome(root, "/repo/worktrees/a") {
		t.Fatal("not deterministic")
	}
	if a == WorktreeHome(root, "/repo/worktrees/b") {
		t.Fatal("collision")
	}
	if !strings.HasPrefix(filepath.Base(a), "a-") {
		t.Fatalf("missing worktree basename: %s", a)
	}
}

func TestWorktreeHomeResolvesSymlink(t *testing.T) {
	root := t.TempDir()
	worktree := filepath.Join(root, "actual-worktree")
	if err := os.Mkdir(worktree, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "worktree-link")
	if err := os.Symlink(worktree, link); err != nil {
		t.Fatal(err)
	}
	if got, want := WorktreeHome(filepath.Join(root, "clients"), link), WorktreeHome(filepath.Join(root, "clients"), worktree); got != want {
		t.Fatalf("symlink home = %q, want %q", got, want)
	}
}
func TestResolvePaths(t *testing.T) {
	p := ResolvePaths("/tmp/chat-home")
	if p.State != filepath.Join(p.Home, "state.json") || p.Cursors != filepath.Join(p.Home, "cursors.json") || p.Lock != filepath.Join(p.Home, ".lock") {
		t.Fatalf("unexpected paths: %+v", p)
	}
}
