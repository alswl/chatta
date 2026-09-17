package dal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMalformedStateIsRejected(t *testing.T) {
	dir := t.TempDir()
	paths := ResolvePaths(dir)
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.State), 0o755))
	require.NoError(t, os.WriteFile(paths.State, []byte("{"), 0o600))
	_, err := LoadState(paths.State)
	require.Error(t, err, "expected malformed state error")
}
