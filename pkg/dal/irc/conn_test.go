//go:build darwin || linux

package irc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// scriptedServer accepts one connection and runs script against it: script
// maps a received command word to a canned reply (or a function producing
// one). Unmatched commands are ignored.
type scriptedServer struct {
	ln       net.Listener
	handlers map[string]func(conn net.Conn, params []string)
}

func newScriptedServer(t *testing.T) *scriptedServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &scriptedServer{ln: ln, handlers: map[string]func(conn net.Conn, params []string){}}
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *scriptedServer) on(command string, handler func(conn net.Conn, params []string)) {
	s.handlers[strings.ToUpper(command)] = handler
}

func (s *scriptedServer) serveOne(t *testing.T) {
	t.Helper()
	go func() {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			msg, ok := ParseLine(scanner.Text())
			if !ok {
				continue
			}
			if h, found := s.handlers[msg.Command]; found {
				h(conn, msg.Params)
			}
		}
	}()
}

func writeLine(conn net.Conn, line string) {
	_, _ = fmt.Fprintf(conn, "%s\r\n", line)
}

func TestConnSuccessfulRegistration(t *testing.T) {
	s := newScriptedServer(t)
	s.on("NICK", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 001 agent-a :welcome")
	})
	s.serveOne(t)
	c, err := Dial(s.ln.Addr().String(), "agent-a", "agent-a", 2*time.Second)
	require.NoError(t, err, "dial")
	defer c.Close()
	require.True(t, c.Registered(), "registration did not complete")
	require.Equal(t, "agent-a", c.Nick())
}

func TestConnNicknameInUseOnRegistration(t *testing.T) {
	s := newScriptedServer(t)
	s.on("NICK", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 433 * agent-a :Nickname is already in use")
	})
	s.serveOne(t)
	_, err := Dial(s.ln.Addr().String(), "agent-a", "agent-a", 2*time.Second)
	require.Error(t, err, "expected nick-in-use error")
	var typed *TypedError
	require.True(t, asTypedError(err, &typed), "expected a typed error, got %v", err)
	require.Equal(t, CodeNickInUse, typed.Code)
}

func TestConnJoinConfirmedBy366(t *testing.T) {
	s := newScriptedServer(t)
	s.on("NICK", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 001 agent-a :welcome")
	})
	s.on("JOIN", func(conn net.Conn, params []string) {
		channel := params[0]
		writeLine(conn, ":agent-a!u@h JOIN "+channel)
		writeLine(conn, ":srv 353 agent-a = "+channel+" :agent-a")
		writeLine(conn, ":srv 366 agent-a "+channel+" :End of names")
	})
	s.serveOne(t)
	c, err := Dial(s.ln.Addr().String(), "agent-a", "agent-a", 2*time.Second)
	require.NoError(t, err, "dial")
	defer c.Close()
	require.NoError(t, c.Join("#agents", 2*time.Second), "join")
	require.Equal(t, []string{"#agents"}, c.Channels())
}

func TestConnNoSuchNickOnAbsentDMTarget(t *testing.T) {
	s := newScriptedServer(t)
	s.on("NICK", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 001 agent-a :welcome")
	})
	s.on("PRIVMSG", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 401 agent-a ghost :No such nick/channel")
	})
	s.serveOne(t)
	c, err := Dial(s.ln.Addr().String(), "agent-a", "agent-a", 2*time.Second)
	require.NoError(t, err, "dial")
	defer c.Close()
	req := c.registerPending("privmsg:ghost")
	require.NoError(t, c.Privmsg("ghost", "hello"), "privmsg")
	err = c.wait(req, 2*time.Second)
	require.Error(t, err, "expected no_such_nick error")
	var typed *TypedError
	require.True(t, asTypedError(err, &typed), "expected a typed error, got %v", err)
	require.Equal(t, CodeNoSuchNick, typed.Code)
}

func TestConnPingPong(t *testing.T) {
	s := newScriptedServer(t)
	s.on("NICK", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 001 agent-a :welcome")
	})
	pongReceived := make(chan struct{}, 1)
	s.on("PONG", func(conn net.Conn, params []string) {
		pongReceived <- struct{}{}
	})
	s.serveOne(t)
	c, err := Dial(s.ln.Addr().String(), "agent-a", "agent-a", 2*time.Second)
	require.NoError(t, err, "dial")
	defer c.Close()
	require.NoError(t, writeToConn(c, "PING :12345"))
	select {
	case <-pongReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("no PONG observed after PING")
	}
}

// writeToConn simulates the server sending a line to the client. The
// unexported read loop isn't reachable from a scripted server connection in
// these tests, so this calls Conn.handle directly instead.
func writeToConn(c *Conn, line string) error {
	msg, ok := ParseLine(line)
	if !ok {
		return fmt.Errorf("bad line")
	}
	c.handle(msg)
	return nil
}

func asTypedError(err error, target **TypedError) bool {
	if e, ok := err.(*TypedError); ok {
		*target = e
		return true
	}
	return false
}

func TestConnErroneousNicknameIsNotReportedAsInUse(t *testing.T) {
	s := newScriptedServer(t)
	s.on("NICK", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 432 * toolongnick :Nickname too long, max. 9 characters")
	})
	s.serveOne(t)
	_, err := Dial(s.ln.Addr().String(), "toolongnick", "toolongnick", 2*time.Second)
	require.Error(t, err, "expected an invalid-nick error")
	var typed *TypedError
	require.True(t, asTypedError(err, &typed), "expected a typed error, got %v", err)
	require.Equal(t, CodeNickInvalid, typed.Code, "432 must not be reported as nick_in_use")
	require.Contains(t, err.Error(), "Nickname too long", "the server's own reason must survive")
}

func TestConnLastActivityAdvancesWithServerTraffic(t *testing.T) {
	s := newScriptedServer(t)
	s.on("NICK", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 001 agent-a :welcome")
	})
	notice := make(chan struct{}, 1)
	s.on("PING", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv NOTICE agent-a :still here")
		notice <- struct{}{}
	})
	s.serveOne(t)
	c, err := Dial(s.ln.Addr().String(), "agent-a", "agent-a", 2*time.Second)
	require.NoError(t, err, "dial")
	defer c.Close()
	before := c.LastActivity()
	require.False(t, before.IsZero(), "a freshly dialled connection must count as active")
	require.NoError(t, c.Ping())
	select {
	case <-notice:
	case <-time.After(2 * time.Second):
		t.Fatal("server never saw the liveness probe")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.LastActivity().After(before) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("LastActivity did not advance after the server replied")
}

func TestConnDisconnectCallbackFiresWhenServerGoesAway(t *testing.T) {
	s := newScriptedServer(t)
	s.on("NICK", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 001 agent-a :welcome")
	})
	s.serveOne(t)
	c, err := Dial(s.ln.Addr().String(), "agent-a", "agent-a", 2*time.Second)
	require.NoError(t, err, "dial")
	gone := make(chan error, 1)
	c.OnDisconnect(func(err error) { gone <- err })
	_ = c.Close()
	select {
	case <-gone:
	case <-time.After(2 * time.Second):
		t.Fatal("a closed connection must report the disconnect")
	}
}
