package dal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMalformedStateIsRejected(t *testing.T) {
	dir := t.TempDir()
	paths := ResolvePaths(dir)
	if err := os.MkdirAll(filepath.Dir(paths.State), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.State, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(paths.State); err == nil {
		t.Fatal("expected malformed state error")
	}
}
