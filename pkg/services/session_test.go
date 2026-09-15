//go:build darwin || linux

package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/config"
)

func testSession() common.ChatSession {
	return common.ChatSession{Nick: "agent-a", Host: "127.0.0.1", Port: 6667, HomeChannel: common.ChannelMembership{Name: "#lobby"}, Channels: []common.ChannelMembership{{Name: "#lobby"}}, Owner: common.OwnerBinding{PID: 1, StartFingerprint: "test"}}
}

func TestChatServiceUsesConfiguredChatHome(t *testing.T) {
	m := NewChatService(config.ChatConfig{Home: t.TempDir(), Host: "127.0.0.1", Port: 6667, Channel: "#agents"})
	if m.Paths.State == "" || m.Paths.Home != m.Home {
		t.Fatalf("unexpected manager paths: %+v", m.Paths)
	}
}

func TestNickTakenErrorMessageHasNoWrapperPrefix(t *testing.T) {
	err := error(nickTakenError{nick: "misky"})
	if !strings.Contains(err.Error(), `the nick "misky" is already in use`) {
		t.Fatalf("unexpected message: %s", err)
	}
	var taken nickTakenError
	if !errors.As(err, &taken) || taken.nick != "misky" {
		t.Fatalf("nick collision was not recognisable through errors.As: %v", err)
	}
	if errors.As(errors.New("some other failure"), &taken) {
		t.Fatal("an unrelated error was treated as a nick collision")
	}
}
