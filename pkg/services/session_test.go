//go:build darwin || linux

package services

import (
	"errors"
	"os"
	"strconv"
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

// US1.4: a failed start must not leave the home looking half-started, or
// every later command tries to recover a session that never existed. This
// drives the fast failure -- the supervisor cannot be spawned at all --
// which shares one cleanup path with the slow one, a readiness deadline
// that expires because the server is unreachable.
func TestFailedStartLeavesNoHalfStartedSession(t *testing.T) {
	t.Setenv("CHATTA_CHAT_TEST_OWNER_PID", strconv.Itoa(os.Getpid()))
	home := t.TempDir()
	m := NewChatService(config.ChatConfig{Home: home, Host: "127.0.0.1", Port: 1, Channel: "#agents"})
	m.Executable = "/nonexistent/chatta"
	if err := m.Start("agent-a", "tester", false); err == nil {
		t.Fatal("expected the start to fail")
	}
	if _, err := os.Stat(m.StatePath); !os.IsNotExist(err) {
		t.Fatalf("state survived a failed start: %v", err)
	}
	if report := m.Health(true); report.Failure != "no session" {
		t.Fatalf("home did not return to the unstarted state: %+v", report)
	}
}
