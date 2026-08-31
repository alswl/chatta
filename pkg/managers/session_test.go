//go:build darwin || linux

package managers

import (
	"os"
	"testing"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/config"
)

func testSession() common.ChatSession {
	return common.ChatSession{Nick: "agent-a", Host: "127.0.0.1", Port: 6667, HomeChannel: common.ChannelMembership{Name: "#lobby"}, Channels: []common.ChannelMembership{{Name: "#lobby"}}, Owner: common.OwnerBinding{PID: 1, StartFingerprint: "test"}}
}

func TestManagerUsesConfiguredChatHome(t *testing.T) {
	m := NewManager(config.ChatConfig{Home: t.TempDir(), Host: "127.0.0.1", Port: 6667, Channel: "#agents"})
	if m.Paths.State == "" || m.Paths.Home != m.Home {
		t.Fatalf("unexpected manager paths: %+v", m.Paths)
	}
}

func TestNicknameTaken(t *testing.T) {
	path := t.TempDir() + "/out"
	if err := os.WriteFile(path, []byte("1700000000 misky Nickname already in use\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !nicknameTaken(path, 0, "misky") {
		t.Fatal("nickname collision was not detected")
	}
}
