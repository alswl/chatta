//go:build darwin || linux

package services

import (
	"os"
	"testing"

	"github.com/alswl/chatta/pkg/config"
)

func TestHealthWithoutSessionNamesFailure(t *testing.T) {
	m := NewChatService(config.ChatConfig{Home: t.TempDir()})
	r := m.Health(true)
	if r.Failure == "" || r.Owner || r.Supervisor {
		t.Fatalf("unexpected health report: %+v", r)
	}
}

func TestHealthDeepFalseSkipsServerLinkCheckWithoutSession(t *testing.T) {
	m := NewChatService(config.ChatConfig{Home: t.TempDir()})
	r := m.Health(false)
	if r.Failure == "" {
		t.Fatalf("unexpected health report: %+v", r)
	}
}

// The migration text is frozen verbatim in contracts/cli-commands.md; it is
// the one user-visible message this feature adds, so drift is a contract
// break rather than a wording preference.
func TestPreUpgradeMigrationMessageMatchesFrozenContract(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(home+"/irc", 0700); err != nil {
		t.Fatal(err)
	}
	got := preUpgradeMigrationMessage(home, nil)
	const want = "this client home was created by an older Chatta and cannot be reused;\nrun: chatta chat session stop --force && chatta chat session start <nick>"
	if got != want {
		t.Fatalf("migration text drifted from the contract:\n got: %q\nwant: %q", got, want)
	}
}
