//go:build darwin || linux

package irc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// TypedError carries a machine-readable failure code alongside a message,
// so callers can map it onto the control-socket failure codes without
// substring-matching server text.
type TypedError struct {
	Code string
	Msg  string
}

func (e *TypedError) Error() string { return e.Msg }

const (
	CodeNickInUse     = "nick_in_use"
	CodeNoSuchNick    = "no_such_nick"
	CodeJoinFailed    = "join_failed"
	CodeTimeout       = "timeout"
	CodeNotRegistered = "not_registered"
)

// Conn is a single connection to an IRC server: it owns the socket, the
// read loop, and pending request/reply correlation. All exported methods
// are safe to call concurrently.
type Conn struct {
	nick string

	writeMu sync.Mutex
	conn    net.Conn

	mu         sync.Mutex
	registered bool
	channels   map[string]bool
	lastPong   int64

	pending map[string]*pendingRequest

	// onMessage, when set, is called for every PRIVMSG received after
	// registration (from the read loop goroutine).
	onMessage func(kind, target, nick, text string)
	// onDisconnect is called once when the read loop exits.
	onDisconnect func(err error)
}

type pendingRequest struct {
	done chan struct{}
	err  error
	// namesCollected accumulates NAMES replies for a "names" request.
	namesCollected []string
}

// Dial connects to addr, registers with nick/user, and blocks until the
// server sends numeric 001 or an error occurs (dial failure, 433).
func Dial(addr, nick, user string, timeout time.Duration) (*Conn, error) {
	raw, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	c := &Conn{
		conn:     raw,
		nick:     nick,
		channels: map[string]bool{},
		pending:  map[string]*pendingRequest{},
	}
	regDone := make(chan error, 1)
	c.mu.Lock()
	c.pending["__register__"] = &pendingRequest{done: make(chan struct{})}
	c.mu.Unlock()
	go c.readLoop()
	if err := c.send(FormatLine("NICK", nick)); err != nil {
		_ = raw.Close()
		return nil, err
	}
	if err := c.send(FormatLine("USER", user, "0", "*", user)); err != nil {
		_ = raw.Close()
		return nil, err
	}
	go func() {
		c.mu.Lock()
		req := c.pending["__register__"]
		c.mu.Unlock()
		<-req.done
		regDone <- req.err
	}()
	select {
	case err := <-regDone:
		if err != nil {
			_ = raw.Close()
			return nil, err
		}
		return c, nil
	case <-time.After(timeout):
		_ = raw.Close()
		return nil, &TypedError{Code: CodeTimeout, Msg: "registration timed out"}
	}
}

// OnMessage sets the callback invoked for every received PRIVMSG.
func (c *Conn) OnMessage(f func(kind, target, nick, text string)) { c.onMessage = f }

// OnDisconnect sets the callback invoked once the read loop exits.
func (c *Conn) OnDisconnect(f func(err error)) { c.onDisconnect = f }

func (c *Conn) send(line string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := fmt.Fprintf(c.conn, "%s\r\n", line)
	return err
}

// Nick returns the currently registered nick.
func (c *Conn) Nick() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nick
}

// Registered reports whether the server has sent numeric 001.
func (c *Conn) Registered() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.registered
}

// Channels returns the channels the server has confirmed membership in.
func (c *Conn) Channels() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.channels))
	for name := range c.channels {
		out = append(out, name)
	}
	return out
}

// LastPong returns the Unix timestamp of the last PONG sent, or 0 if none.
func (c *Conn) LastPong() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastPong
}

// Close closes the underlying connection.
func (c *Conn) Close() error { return c.conn.Close() }

// Join joins a channel and waits for the server to confirm membership via
// the JOIN echo plus numeric 366, or resolve a typed join error.
func (c *Conn) Join(channel string, timeout time.Duration) error {
	if !c.Registered() {
		return &TypedError{Code: CodeNotRegistered, Msg: "not registered"}
	}
	req := c.registerPending("join:" + channel)
	if err := c.send(FormatLine("JOIN", channel)); err != nil {
		c.resolvePending("join:"+channel, err)
		return err
	}
	return c.wait(req, timeout)
}

// Part leaves a channel.
func (c *Conn) Part(channel, reason string, timeout time.Duration) error {
	if err := c.send(FormatLine("PART", channel, reason)); err != nil {
		return err
	}
	c.mu.Lock()
	delete(c.channels, channel)
	c.mu.Unlock()
	return nil
}

// Names requests the member list for a channel, collecting 353 replies
// until 366, and returns the raw names (with @/+ prefixes intact).
func (c *Conn) Names(channel string, timeout time.Duration) ([]string, error) {
	if !c.Registered() {
		return nil, &TypedError{Code: CodeNotRegistered, Msg: "not registered"}
	}
	req := c.registerPending("names:" + channel)
	if err := c.send(FormatLine("NAMES", channel)); err != nil {
		c.resolvePending("names:"+channel, err)
		return nil, err
	}
	if err := c.wait(req, timeout); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return req.namesCollected, nil
}

// Privmsg sends a message and returns once the write completes.
func (c *Conn) Privmsg(target, text string) error {
	return c.send(FormatLine("PRIVMSG", target, text))
}

// PrivmsgDM sends a direct message and waits up to timeout for the server
// to reject it with a 401 (no such nick). If no rejection arrives within
// the window, the message is assumed delivered.
func (c *Conn) PrivmsgDM(target, text string, timeout time.Duration) error {
	key := "privmsg:" + target
	req := c.registerPending(key)
	if err := c.Privmsg(target, text); err != nil {
		c.resolvePending(key, err)
		return err
	}
	select {
	case <-req.done:
		return req.err
	case <-time.After(timeout):
		c.resolvePending(key, nil)
		return nil
	}
}

// Quit sends QUIT.
func (c *Conn) Quit(reason string) error {
	return c.send(FormatLine("QUIT", reason))
}

func (c *Conn) registerPending(key string) *pendingRequest {
	req := &pendingRequest{done: make(chan struct{})}
	c.mu.Lock()
	c.pending[key] = req
	c.mu.Unlock()
	return req
}

func (c *Conn) resolvePending(key string, err error) {
	c.mu.Lock()
	req, ok := c.pending[key]
	if ok {
		delete(c.pending, key)
	}
	c.mu.Unlock()
	if ok {
		req.err = err
		close(req.done)
	}
}

func (c *Conn) wait(req *pendingRequest, timeout time.Duration) error {
	select {
	case <-req.done:
		return req.err
	case <-time.After(timeout):
		return &TypedError{Code: CodeTimeout, Msg: "request timed out"}
	}
}

func (c *Conn) readLoop() {
	scanner := bufio.NewScanner(c.conn)
	scanner.Buffer(make([]byte, 0, 4096), 8192)
	for scanner.Scan() {
		msg, ok := ParseLine(scanner.Text())
		if !ok {
			continue
		}
		c.handle(msg)
	}
	err := scanner.Err()
	if c.onDisconnect != nil {
		c.onDisconnect(err)
	}
}

func (c *Conn) handle(msg Message) {
	switch msg.Command {
	case "PING":
		payload := ""
		if len(msg.Params) > 0 {
			payload = msg.Params[0]
		}
		_ = c.send(FormatLine("PONG", payload))
		c.mu.Lock()
		c.lastPong = time.Now().Unix()
		c.mu.Unlock()
	case RPL_WELCOME:
		c.mu.Lock()
		c.registered = true
		if len(msg.Params) > 0 {
			c.nick = msg.Params[0]
		}
		c.mu.Unlock()
		c.resolvePending("__register__", nil)
	case ERR_NICKNAMEINUSE:
		c.resolvePending("__register__", &TypedError{Code: CodeNickInUse, Msg: fmt.Sprintf("the nick %q is already in use on this server; choose a distinct nick if it belongs to another agent", attemptedNick(msg))})
	case ERR_ERRONEUSNICKNAME:
		reason := ""
		if len(msg.Params) > 0 {
			reason = msg.Params[len(msg.Params)-1]
		}
		c.resolvePending("__register__", &TypedError{Code: CodeNickInUse, Msg: fmt.Sprintf("the nick %q was rejected by the server: %s", attemptedNick(msg), reason)})
	case "JOIN":
		nick := Nick(msg.Prefix)
		if nick != c.Nick() || len(msg.Params) == 0 {
			return
		}
		channel := msg.Params[0]
		c.mu.Lock()
		c.channels[channel] = true
		c.mu.Unlock()
	case RPL_ENDOFNAMES:
		if len(msg.Params) < 2 {
			return
		}
		channel := msg.Params[1]
		c.resolvePending("names:"+channel, nil)
		c.resolvePending("join:"+channel, nil)
	case RPL_NAMREPLY:
		if len(msg.Params) < 3 {
			return
		}
		channel := msg.Params[2]
		c.mu.Lock()
		req, ok := c.pending["names:"+channel]
		c.mu.Unlock()
		if ok {
			req.namesCollected = append(req.namesCollected, strings.Fields(msg.Params[len(msg.Params)-1])...)
		}
	case ERR_NOSUCHNICK:
		target := ""
		if len(msg.Params) > 1 {
			target = msg.Params[1]
		}
		c.resolvePending("privmsg:"+target, &TypedError{Code: CodeNoSuchNick, Msg: fmt.Sprintf("direct message target %q is absent", target)})
	case ERR_NOSUCHCHANNEL, ERR_INVITEONLYCHAN, ERR_BANNEDFROMCHAN, ERR_BADCHANNELKEY:
		if len(msg.Params) < 2 {
			return
		}
		channel := msg.Params[1]
		c.resolvePending("join:"+channel, &TypedError{Code: CodeJoinFailed, Msg: fmt.Sprintf("the server did not confirm membership in %s", channel)})
	case "PRIVMSG":
		if len(msg.Params) < 2 {
			return
		}
		target, text := msg.Params[0], msg.Params[len(msg.Params)-1]
		nick := Nick(msg.Prefix)
		kind := "direct"
		if strings.HasPrefix(target, "#") {
			kind = "channel"
		} else {
			target = nick
		}
		if c.onMessage != nil {
			c.onMessage(kind, target, nick, text)
		}
	}
}

// attemptedNick extracts the rejected nick from a 433 reply, whose params
// are ["*", "<nick>", "<reason>"].
func attemptedNick(msg Message) string {
	if len(msg.Params) < 2 {
		return ""
	}
	return msg.Params[len(msg.Params)-2]
}
