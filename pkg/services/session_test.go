//go:build darwin || linux

package services

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/config"
	"github.com/stretchr/testify/require"
)

func testSession() common.ChatSession {
	return common.ChatSession{Nick: "agent-a", Host: "127.0.0.1", Port: 6667, HomeChannel: common.ChannelMembership{Name: "#lobby"}, Channels: []common.ChannelMembership{{Name: "#lobby"}}, Owner: common.OwnerBinding{PID: 1, StartFingerprint: "test"}}
}

func TestChatServiceUsesConfiguredChatHome(t *testing.T) {
	m := NewChatService(config.ChatConfig{Home: t.TempDir(), Host: "127.0.0.1", Port: 6667, Channel: "#agents"})
	require.NotEmpty(t, m.Paths.State)
	require.Equal(t, m.Home, m.Paths.Home)
}

func TestNickTakenErrorMessageHasNoWrapperPrefix(t *testing.T) {
	err := error(nickTakenError{nick: "misky"})
	require.Contains(t, err.Error(), `the nick "misky" is already in use`)
	var taken nickTakenError
	require.ErrorAs(t, err, &taken, "nick collision was not recognisable through errors.As")
	require.Equal(t, "misky", taken.nick)
	require.False(t, errors.As(errors.New("some other failure"), &taken), "an unrelated error was treated as a nick collision")
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
	require.Error(t, m.Start(context.Background(), "agent-a", "tester", false), "expected the start to fail")
	_, statErr := os.Stat(m.StatePath)
	require.True(t, os.IsNotExist(statErr), "state survived a failed start: %v", statErr)
	require.Equal(t, "no session", m.Health(context.Background(), true).Failure,
		"home did not return to the unstarted state")
}
