//go:build darwin || linux

package chat

import (
	"testing"

	"github.com/alswl/chatta/pkg/config"
)

func TestManagerUsesConfiguredChatHome(t *testing.T) {
	m := NewManager(config.ChatConfig{Home: t.TempDir(), Host: "127.0.0.1", Port: 6667, Channel: "#agents"})
	if m.Paths.State == "" || m.Paths.Home != m.Home {
		t.Fatalf("unexpected manager paths: %+v", m.Paths)
	}
}
