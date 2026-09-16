//go:build darwin || linux

package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/daemon"
	"github.com/alswl/chatta/pkg/dal"
	"github.com/alswl/chatta/pkg/dal/irc"
)

func (m *ChatService) Start(ctx context.Context, nick, role string, takeover bool) error {
	owner, err := m.findOwner()
	if err != nil {
		return err
	}
	if nick == "" {
		return errors.New("nick is required")
	}
	if role == "" {
		role = "agent"
	}
	if m.StatePath == "" {
		return errors.New("chat manager has no state path")
	}
	if old, err := dal.LoadState(m.StatePath); err == nil {
		if !takeover && dal.ProcessAlive(old.Owner) && !dal.SameOwner(old.Owner, owner) {
			return fmt.Errorf("this client belongs to another live agent (nick %s); use --takeover only after confirmation", old.Nick)
		}
		if old.SupervisorPID > 0 {
			_ = dal.StopVerifiedSupervisor(old.SupervisorPID, old.SupervisorStartFingerprint)
		}
	}

	normalized := dal.NormalizeChannel(m.Channel)
	session := common.ChatSession{
		SchemaVersion: common.StateSchemaVersion,
		Nick:          nick, Role: role, Host: m.Host, Port: m.Port,
		HomeChannel: common.ChannelMembership{Name: normalized, Kind: "lobby", Confirmed: false},
		Channels:    []common.ChannelMembership{{Name: normalized, Kind: "lobby", Confirmed: false}},
		Owner:       owner, SessionID: dal.OwnerSessionID(owner), StartedAt: time.Now(),
	}
	if err := dal.SaveState(m.StatePath, session); err != nil {
		return err
	}
	m.State = session
	// A nick is not released the instant its previous client dies: the server
	// holds it for a few seconds. Reconnecting under one's own nick therefore
	// needs a fresh attempt, not a longer wait -- so a collision is retried
	// with a new supervisor rather than reported straight to the caller.
	var lastErr error
	for attempt := 0; attempt < nickAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(nickRetryDelay)
		}
		err := m.startOnce(ctx, &session, nick)
		if err == nil {
			return nil
		}
		lastErr = err
		var taken nickTakenError
		if !errors.As(err, &taken) {
			break
		}
	}
	// A start that failed must not leave the home looking half-started:
	// state naming an owner with no supervisor behind it would make every
	// later command try to recover a session that never existed (US1.4).
	m.discardSession()
	return lastErr
}

// discardSession returns the home to its unstarted state, keeping the
// supervisor log because that is where the failure is explained.
func (m *ChatService) discardSession() {
	_ = os.Remove(m.StatePath)
	_ = os.Remove(m.Paths.Lock)
	m.State = common.ChatSession{}
}

// nickAttempts and nickRetryDelay bound the wait for the server to release a
// nick this client held moments ago. Measured: a retry a few seconds later
// succeeds every time, while waiting inside one attempt never does.
const (
	nickAttempts   = 3
	nickRetryDelay = 3 * time.Second
)

// livenessProbe and livenessDeadline bound how long a connection may stay
// silent before it is probed and then abandoned. Both sit well inside the
// 30s self-heal cycle the chat skill documents, so an unusable link is
// reported as unhealthy rather than waited on indefinitely (FR-007).
const (
	livenessProbe    = 15 * time.Second
	livenessDeadline = 35 * time.Second
)

// nickTakenError carries the nick so the retry loop can recognise the case
// without the caller ever seeing a wrapped sentinel in the message.
type nickTakenError struct {
	nick string
	// detail is the server's own wording, preferred over the generic text
	// so a rejection is never reported as the wrong kind of failure.
	detail string
}

func (e nickTakenError) Error() string {
	if e.detail != "" {
		return e.detail
	}
	return fmt.Sprintf("the nick %q is already in use on this server; choose a distinct nick if it belongs to another agent", e.nick)
}

func (m *ChatService) startOnce(ctx context.Context, base *common.ChatSession, nick string) error {
	session := *base
	pid, err := m.spawnSupervisor()
	if err != nil {
		return err
	}
	session.SupervisorPID = pid
	session.SupervisorStartFingerprint = dal.ProcessStart(pid)
	if session.SupervisorStartFingerprint == "" {
		return fmt.Errorf("could not establish supervisor process identity")
	}
	if err := dal.SaveState(m.StatePath, session); err != nil {
		_ = dal.StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
		return err
	}
	m.State = session
	// The supervisor knows exactly why it cannot serve -- a refused dial, a
	// rejected nick, a control socket it could not bind. Startup used to
	// discard all of that and report a generic timeout, so keep hold of the
	// last specific reason and hand it to the caller (FR-008).
	deadline := time.Now().Add(20 * time.Second)
	detail := ""
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			_ = dal.StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
			return err
		}
		resp, reqErr := daemon.Request(ctx, m.Paths.ControlSock, daemon.ControlRequest{Op: "status"})
		if reqErr == nil && !resp.OK {
			if resp.Error != "" {
				detail = resp.Error
			}
			switch resp.Code {
			case daemon.CodeNickInUse:
				_ = dal.StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
				return nickTakenError{nick: nick, detail: resp.Error}
			case daemon.CodeNickInvalid:
				// Retrying cannot help: the nick itself has to change.
				_ = dal.StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
				return errors.New(detail)
			}
		}
		if report := m.Health(ctx, true); report.Owner && report.Supervisor && report.JoinedChannels && report.ServerLink && report.Membership {
			return nil
		}
		// A supervisor that died before it could listen never gets to answer
		// on the control socket, so its reason exists only in its log.
		if !dal.IsVerifiedSupervisor(pid, session.SupervisorStartFingerprint) {
			if reason := supervisorLogReason(m.Paths.Log); reason != "" {
				return fmt.Errorf("the chat supervisor exited during startup: %s", reason)
			}
			return fmt.Errorf("the chat supervisor exited during startup; inspect %s", m.Paths.Log)
		}
		time.Sleep(250 * time.Millisecond)
	}
	_ = dal.StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
	if detail != "" {
		return fmt.Errorf("chat client did not become ready: %s", detail)
	}
	return fmt.Errorf("chat client did not become ready; inspect %s", m.Paths.Log)
}

// supervisorLogReason returns the last error the supervisor logged, so a
// failure it could only write to its log still reaches the user.
func supervisorLogReason(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); strings.Contains(line, "level=ERROR") {
			return line
		}
	}
	return ""
}

func (m *ChatService) spawnSupervisor() (int, error) {
	if err := os.MkdirAll(m.Paths.Home, 0700); err != nil {
		return 0, err
	}
	executable := m.Executable
	if executable == "" {
		executable = os.Args[0]
	}
	cmd := exec.Command(executable, "chat", "--home", m.Home, "--host", m.Host, "--port", fmt.Sprint(m.Port), "--channel", m.Channel, "_supervise")
	cmd.Stdout, cmd.Stderr = mustOpenLog(m.Paths.Log)
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

// mustOpenLog opens the supervisor's own log for the outer supervisor
// process's own stdout/stderr. If it cannot be opened, it says so on
// stderr instead of silently discarding output that would otherwise
// vanish once the process detaches from its launching terminal.
func mustOpenLog(path string) (*os.File, *os.File) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "chatta: could not open supervisor log %s: %v\n", path, err)
		return os.Stderr, os.Stderr
	}
	return f, f
}

// openSupervisorLog opens a structured logger for the supervisor's own
// lifecycle events (start, connection loss, owner death, shutdown) at the
// same path used for its own stdout/stderr. If that path can't be opened,
// it falls back to a discoverable temp file rather than a detached
// process's stderr, which is discarded -- logging the fallback itself so
// the failure is not silent.
func openSupervisorLog(path string) (*slog.Logger, *os.File) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err == nil {
		return slog.New(slog.NewTextHandler(f, nil)), f
	}
	fallback := filepath.Join(os.TempDir(), fmt.Sprintf("chatta-supervisor-%d.log", os.Getpid()))
	if f2, ferr := os.OpenFile(fallback, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); ferr == nil {
		logger := slog.New(slog.NewTextHandler(f2, nil))
		logger.Warn("could not open configured supervisor log; using fallback", "path", path, "fallback", fallback, "error", err)
		return logger, f2
	}
	return slog.New(slog.NewTextHandler(os.Stderr, nil)), os.Stderr
}

// controlHandler serves the supervisor's control socket, dispatching each
// request onto the currently held irc.Conn (nil while disconnected).
type controlHandler struct {
	m   *ChatService
	log *slog.Logger

	mu      sync.Mutex
	conn    *irc.Conn
	failure error
	// generation counts established connections. A watcher that sees it
	// change knows the link was interrupted, even if the reconnect was
	// quick enough that it never observed the gap itself.
	generation int64
}

func (h *controlHandler) setConn(c *irc.Conn) {
	h.mu.Lock()
	h.conn = c
	if c != nil {
		h.failure = nil
		h.generation++
	}
	h.mu.Unlock()
}

func (h *controlHandler) setFailure(err error) {
	h.mu.Lock()
	h.conn = nil
	h.failure = err
	h.mu.Unlock()
}

func (h *controlHandler) getConn() *irc.Conn {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.conn
}

func (h *controlHandler) Handle(req daemon.ControlRequest) daemon.ControlResponse {
	switch req.Op {
	case "status":
		return h.status()
	case "join":
		return h.join(req.Target)
	case "part":
		return h.part(req.Target, req.Reason)
	case "names":
		return h.names(req.Target)
	case "privmsg":
		return h.privmsg(req.Target, req.Text)
	case "quit":
		if conn := h.getConn(); conn != nil {
			_ = conn.Quit(req.Reason)
		}
		return daemon.ControlResponse{OK: true}
	default:
		return daemon.ControlResponse{OK: false, Code: daemon.CodeBadRequest, Error: "unknown op: " + req.Op}
	}
}

func (h *controlHandler) status() daemon.ControlResponse {
	conn := h.getConn()
	if conn == nil {
		h.mu.Lock()
		failure := h.failure
		h.mu.Unlock()
		code, errMsg := daemon.CodeNotConnected, "not connected"
		if failure != nil {
			errMsg = failure.Error()
			var typed *irc.TypedError
			if errors.As(failure, &typed) {
				switch typed.Code {
				case irc.CodeNickInUse:
					code = daemon.CodeNickInUse
				case irc.CodeNickInvalid:
					code = daemon.CodeNickInvalid
				}
			}
		}
		return daemon.ControlResponse{OK: false, Code: code, Error: errMsg}
	}
	h.mu.Lock()
	generation := h.generation
	h.mu.Unlock()
	return daemon.ControlResponse{OK: true, Status: &common.TransportStatus{
		Connected: true, Registered: conn.Registered(), Nick: conn.Nick(),
		Channels: conn.Channels(), LastPong: conn.LastPong(), Generation: generation,
	}}
}

func (h *controlHandler) join(target string) daemon.ControlResponse {
	conn := h.getConn()
	if conn == nil {
		return daemon.ControlResponse{OK: false, Code: daemon.CodeNotConnected, Error: "not connected"}
	}
	if err := conn.Join(target, 10*time.Second); err != nil {
		return mapTransportErr(err, daemon.CodeJoinFailed)
	}
	return daemon.ControlResponse{OK: true}
}

func (h *controlHandler) part(target, reason string) daemon.ControlResponse {
	conn := h.getConn()
	if conn == nil {
		return daemon.ControlResponse{OK: false, Code: daemon.CodeNotConnected, Error: "not connected"}
	}
	if err := conn.Part(target, reason, 10*time.Second); err != nil {
		return mapTransportErr(err, daemon.CodeJoinFailed)
	}
	return daemon.ControlResponse{OK: true}
}

func (h *controlHandler) names(target string) daemon.ControlResponse {
	conn := h.getConn()
	if conn == nil {
		return daemon.ControlResponse{OK: false, Code: daemon.CodeNotConnected, Error: "not connected"}
	}
	members, err := conn.Names(target, 10*time.Second)
	if err != nil {
		return mapTransportErr(err, daemon.CodeJoinFailed)
	}
	return daemon.ControlResponse{OK: true, Members: members}
}

func (h *controlHandler) privmsg(target, text string) daemon.ControlResponse {
	conn := h.getConn()
	if conn == nil {
		return daemon.ControlResponse{OK: false, Code: daemon.CodeNotConnected, Error: "not connected"}
	}
	if isChannel(target) {
		if err := conn.Privmsg(target, text); err != nil {
			return daemon.ControlResponse{OK: false, Code: daemon.CodeNotConnected, Error: err.Error()}
		}
		return daemon.ControlResponse{OK: true}
	}
	if err := conn.PrivmsgDM(target, text, time.Second); err != nil {
		return mapTransportErr(err, daemon.CodeNoSuchNick)
	}
	return daemon.ControlResponse{OK: true}
}

func isChannel(target string) bool { return len(target) > 0 && target[0] == '#' }

func mapTransportErr(err error, fallback string) daemon.ControlResponse {
	var typed *irc.TypedError
	if errors.As(err, &typed) {
		code := fallback
		switch typed.Code {
		case irc.CodeNotRegistered:
			code = daemon.CodeNotRegistered
		case irc.CodeTimeout:
			code = daemon.CodeTimeout
		case irc.CodeNoSuchNick:
			code = daemon.CodeNoSuchNick
		case irc.CodeJoinFailed:
			code = daemon.CodeJoinFailed
		case irc.CodeNickInUse:
			code = daemon.CodeNickInUse
		}
		return daemon.ControlResponse{OK: false, Code: code, Error: typed.Msg}
	}
	return daemon.ControlResponse{OK: false, Code: fallback, Error: err.Error()}
}

// supervise is the resident process's run loop: it owns the IRC connection
// and the control socket for as long as the owning agent session is alive,
// reconnecting and rejoining channels whenever the connection drops.
func (m *ChatService) supervise() error {
	lock, err := dal.LockHome(m.Paths.Lock)
	if err != nil {
		return fmt.Errorf("another supervisor owns this client: %w", err)
	}
	defer func() { _ = lock.Close() }()
	log, logFile := openSupervisorLog(m.Paths.Log)
	defer func() { _ = logFile.Close() }()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	srv, err := daemon.Listen(m.Paths.ControlSock)
	if err != nil {
		log.Error("failed to listen on control socket", "error", err)
		return err
	}
	defer func() { _ = srv.Close() }()

	h := &controlHandler{m: m, log: log}
	go func() {
		if err := srv.Serve(ctx, h); err != nil {
			log.Error("control socket server exited", "error", err)
		}
	}()

	log.Info("supervisor starting", "home", m.Home, "nick", m.State.Nick)
	for {
		st, err := dal.LoadState(m.StatePath)
		if err != nil {
			log.Error("failed to load state", "error", err)
			return err
		}
		if !dal.ProcessAlive(st.Owner) {
			log.Info("owner has exited; stopping supervisor")
			return nil
		}

		conn, err := irc.Dial(fmt.Sprintf("%s:%d", st.Host, st.Port), st.Nick, st.Nick, 10*time.Second)
		if err != nil {
			log.Warn("failed to connect", "error", err)
			h.setFailure(err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
				continue
			}
		}
		conn.OnMessage(func(kind, target, nick, text string) {
			if err := dal.AppendMessage(m.Paths.Messages, common.StoredMessage{TS: time.Now().Unix(), Kind: kind, Target: target, Nick: nick, Text: text}); err != nil {
				log.Error("failed to append message", "error", err)
			}
		})
		disconnected := make(chan error, 1)
		conn.OnDisconnect(func(err error) { disconnected <- err })
		h.setConn(conn)

		for _, channel := range st.Channels {
			if err := conn.Join(channel.Name, 10*time.Second); err != nil {
				log.Error("failed to join channel", "channel", channel.Name, "error", err)
			}
		}

		ticker := time.NewTicker(time.Second)
		running := true
		for running {
			select {
			case <-ctx.Done():
				log.Info("stop signal received")
				_ = conn.Quit("leaving")
				_ = conn.Close()
				ticker.Stop()
				h.setConn(nil)
				return nil
			case waitErr := <-disconnected:
				if waitErr != nil {
					log.Warn("connection lost", "error", waitErr)
				}
				running = false
			case <-ticker.C:
				if !dal.ProcessAlive(st.Owner) {
					log.Info("owner process ended; stopping supervisor")
					announceDeparture(conn, st.Nick, log)
					_ = conn.Quit("owner exited")
					_ = conn.Close()
					ticker.Stop()
					h.setConn(nil)
					return nil
				}
				// A frozen or half-open server leaves the socket established,
				// so the read loop never ends and nothing else would notice.
				// Silence is the only symptom: probe it, then give up on it.
				switch idle := time.Since(conn.LastActivity()); {
				case idle > livenessDeadline:
					log.Warn("no traffic from server; dropping the connection", "idle", idle.Round(time.Second))
					h.setFailure(fmt.Errorf("no response from %s:%d for %s", st.Host, st.Port, idle.Round(time.Second)))
					_ = conn.Close()
				case idle > livenessProbe:
					_ = conn.Ping()
				}
			}
		}
		ticker.Stop()
		_ = conn.Close()
		h.setConn(nil)
		time.Sleep(time.Second)
	}
}

// RunSupervisor is called by the hidden CLI command.
func (m *ChatService) RunSupervisor() error { return m.supervise() }

// announceDeparture sends the goodbye the agent runtime no longer can: an
// agent ending its own session says one first, but one whose runtime exits
// under it never gets the chance, leaving peers to address someone who
// stopped reading. It has to be a PRIVMSG -- a bare QUIT carries no tag and
// so never reaches a peer's inbox -- and it must not delay the QUIT that
// follows, so nothing here waits for confirmation.
func announceDeparture(conn *irc.Conn, nick string, log *slog.Logger) {
	text := fmt.Sprintf("[STATUS] %s -> all: my session ended; signing off.", nick)
	for _, channel := range conn.Channels() {
		if err := conn.Privmsg(channel, text); err != nil {
			log.Warn("failed to announce departure", "channel", channel, "error", err)
		}
	}
}
