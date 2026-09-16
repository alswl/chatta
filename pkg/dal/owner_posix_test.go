//go:build darwin || linux

package dal

import (
	"os"
	"strconv"
	"testing"

	"github.com/alswl/chatta/pkg/common"

	"github.com/stretchr/testify/require"
)

func TestProcessStartAndAlive(t *testing.T) {
	fingerprint := ProcessStart(os.Getpid())
	require.NotEmpty(t, fingerprint, "ProcessStart returned an empty fingerprint")
	require.True(t, ProcessAlive(common.OwnerBinding{PID: os.Getpid(), StartFingerprint: fingerprint}),
		"current process should be alive")
	require.False(t, ProcessAlive(common.OwnerBinding{PID: os.Getpid(), StartFingerprint: "different"}),
		"mismatched fingerprint must not be alive")
}

func TestInvocationSessionIDPrefersCallingRuntime(t *testing.T) {
	t.Setenv("CODEX_SESSION_ID", "reader-session")
	require.Equal(t, "reader-session", InvocationSessionID("owner-session"))
}

func TestFindAgentOwnerUsesValidatedExplicitTestOwner(t *testing.T) {
	t.Setenv(testOwnerPIDEnv, strconv.Itoa(os.Getpid()))
	owner, err := FindAgentOwner()
	require.NoError(t, err)
	require.Equal(t, os.Getpid(), owner.PID)
	require.Equal(t, "verification-fixture", owner.Runtime)
	require.True(t, ProcessAlive(owner), "fixture owner is not alive")
}

func TestFindAgentOwnerRejectsInvalidExplicitTestOwner(t *testing.T) {
	t.Setenv(testOwnerPIDEnv, "not-a-pid")
	_, err := FindAgentOwner()
	require.Error(t, err, "invalid fixture owner unexpectedly succeeded")
}
