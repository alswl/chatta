//go:build darwin || linux

package services

import (
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
