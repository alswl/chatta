//go:build darwin || linux

package services

import (
	"context"
	"os"
	"testing"

	"github.com/alswl/chatta/pkg/config"

	"github.com/stretchr/testify/require"
)

func TestHealthWithoutSessionNamesFailure(t *testing.T) {
	m := NewChatService(config.ChatConfig{Home: t.TempDir()})
	r := m.Health(context.Background(), true)
	require.NotEmpty(t, r.Failure, "report: %+v", r)
	require.False(t, r.Owner, "report: %+v", r)
	require.False(t, r.Supervisor, "report: %+v", r)
}

func TestHealthDeepFalseSkipsServerLinkCheckWithoutSession(t *testing.T) {
	m := NewChatService(config.ChatConfig{Home: t.TempDir()})
	r := m.Health(context.Background(), false)
	require.NotEmpty(t, r.Failure, "report: %+v", r)
}

// The migration text is frozen verbatim in contracts/cli-commands.md; it is
// the one user-visible message this feature adds, so drift is a contract
// break rather than a wording preference.
func TestPreUpgradeMigrationMessageMatchesFrozenContract(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.Mkdir(home+"/irc", 0700))
	got := preUpgradeMigrationMessage(home, nil)
	const want = "this client home was created by an older Chatta and cannot be reused;\nrun: chatta chat session stop --force && chatta chat session start <nick>"
	require.Equal(t, want, got, "migration text drifted from the contract")
}
