//go:build darwin || linux

package chat

import (
	"testing"

	"github.com/alswl/chatta/pkg/config"
)

func TestHealthWithoutSessionNamesFailure(t *testing.T) {
	m := NewManager(config.ChatConfig{Home: t.TempDir()})
	r := m.Health(true)
	if r.Failure == "" || r.Owner || r.Supervisor {
		t.Fatalf("unexpected health report: %+v", r)
	}
}
