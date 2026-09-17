//go:build darwin || linux

package dal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLockHomeExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "supervisor.lock")
	first, err := LockHome(path)
	require.NoError(t, err)
	defer first.Close()
	second, err := LockHome(path)
	if err == nil {
		second.Close()
	}
	require.Error(t, err, "second lock unexpectedly succeeded")
}

func TestIsSupervisorRejectsUnrelatedProcess(t *testing.T) {
	require.False(t, IsSupervisor(os.Getpid()), "test process must not be treated as supervisor")
}
