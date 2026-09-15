//go:build darwin || linux

package irc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
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
	if err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if !c.Registered() || c.Nick() != "agent-a" {
		t.Fatalf("registration did not complete: registered=%v nick=%s", c.Registered(), c.Nick())
	}
}

func TestConnNicknameInUseOnRegistration(t *testing.T) {
	s := newScriptedServer(t)
	s.on("NICK", func(conn net.Conn, params []string) {
		writeLine(conn, ":srv 433 * agent-a :Nickname is already in use")
	})
	s.serveOne(t)
	_, err := Dial(s.ln.Addr().String(), "agent-a", "agent-a", 2*time.Second)
	if err == nil {
		t.Fatal("expected nick-in-use error")
	}
	var typed *TypedError
	if !asTypedError(err, &typed) || typed.Code != CodeNickInUse {
		t.Fatalf("expected nick_in_use typed error, got %v", err)
	}
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
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if err := c.Join("#agents", 2*time.Second); err != nil {
		t.Fatalf("join: %v", err)
	}
	channels := c.Channels()
	if len(channels) != 1 || channels[0] != "#agents" {
		t.Fatalf("unexpected channels: %v", channels)
	}
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
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	req := c.registerPending("privmsg:ghost")
	if err := c.Privmsg("ghost", "hello"); err != nil {
		t.Fatalf("privmsg: %v", err)
	}
	err = c.wait(req, 2*time.Second)
	if err == nil {
		t.Fatal("expected no_such_nick error")
	}
	var typed *TypedError
	if !asTypedError(err, &typed) || typed.Code != CodeNoSuchNick {
		t.Fatalf("expected no_such_nick typed error, got %v", err)
	}
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
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if err := writeToConn(c, "PING :12345"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pongReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("no PONG observed after PING")
	}
}

// writeToConn simulates the server sending a line to the client by writing
// directly on the connection the client dialed with, via its handle method
// through the read loop path is not directly testable from outside, so this
// test drives handle() indirectly by writing on a raw connection to the
// scripted server which never sees it. Instead, exercise Conn.handle directly.
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
