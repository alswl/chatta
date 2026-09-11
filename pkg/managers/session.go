//go:build darwin || linux

package managers

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
	"syscall"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/dal"
)

func (m *Manager) Start(nick, role string, takeover bool) error {
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
	if err := dal.ReapStrayII(m.Paths.Conversations); err != nil {
		return fmt.Errorf("reap stale ii client: %w", err)
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
	serverOut := filepath.Join(m.Paths.Conversations, session.Host, "out")
	// A nick is not released the instant its previous client dies: the server
	// holds it for a few seconds, and an ii that gets rejected does not
	// re-register on its own. Reconnecting under one's own nick therefore
	// needs a fresh client, not a longer wait -- so a collision is retried
	// with a new supervisor rather than reported straight to the caller.
	var lastErr error
	for attempt := 0; attempt < nickAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(nickRetryDelay)
		}
		err := m.startOnce(&session, serverOut, nick)
		if err == nil {
			return nil
		}
		lastErr = err
		var taken nickTakenError
		if !errors.As(err, &taken) {
			return err
		}
	}
	return lastErr
}

// nickAttempts and nickRetryDelay bound the wait for the server to release a
// nick this client held moments ago. Measured: a retry a few seconds later
// succeeds every time, while waiting inside one attempt never does.
const (
	nickAttempts   = 3
	nickRetryDelay = 3 * time.Second
)

// nickTakenError carries the nick so the retry loop can recognise the case
// without the caller ever seeing a wrapped sentinel in the message.
type nickTakenError struct{ nick string }

func (e nickTakenError) Error() string {
	return fmt.Sprintf("the nick %q is already in use on this server; choose a distinct nick if it belongs to another agent", e.nick)
}

func (m *Manager) startOnce(base *common.ChatSession, serverOut, nick string) error {
	session := *base
	serverOffset := fileSize(serverOut)
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
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if nicknameTaken(serverOut, serverOffset, nick) {
			_ = dal.StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
			return nickTakenError{nick: nick}
		}
		if report := m.Health(true); report.Owner && report.Supervisor && report.ClientReader && report.ServerLink && report.Membership {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	_ = dal.StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
	return fmt.Errorf("chat client did not become ready; inspect %s", m.Paths.Log)
}

func (m *Manager) spawnSupervisor() (int, error) {
	if err := os.MkdirAll(m.Paths.Home, 0700); err != nil {
		return 0, err
	}
	executable := m.Executable
	if executable == "" {
		executable = os.Args[0]
	}
	cmd := exec.Command(executable, "chat", "--home", m.Home, "--host", m.Host, "--port", fmt.Sprint(m.Port), "--channel", m.Channel, "--ii", m.II, "_supervise")
	cmd.Stdout, cmd.Stderr = mustOpenLog(m.Paths.Log)
	cmd.Stdin = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func nicknameTaken(path string, offset int64, nick string) bool {
	lines, _, _ := dal.Tail(path, offset)
	for _, line := range lines {
		if strings.Contains(line, nick+" Nickname already in use") || (strings.Contains(line, nick) && strings.Contains(strings.ToLower(line), "nickname already in use")) {
			return true
		}
	}
	return false
}

// mustOpenLog opens the supervisor's ii log for the outer supervisor
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
// lifecycle events (start, ii exit, owner death, shutdown) at the same
// path used for ii's raw output. If that path can't be opened, it falls
// back to a discoverable temp file rather than a detached process's stderr,
// which is discarded — logging the fallback itself so the failure is not
// silent.
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

func (m *Manager) supervise() error {
	lock, err := dal.LockHome(m.Paths.Lock)
	if err != nil {
		return fmt.Errorf("another supervisor owns this client: %w", err)
	}
	defer func() { _ = lock.Close() }()
	log, logFile := openSupervisorLog(m.Paths.Log)
	defer func() { _ = logFile.Close() }()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
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
		client := &dal.II{Paths: m.Paths}
		if err := client.Start(st, m.II); err != nil {
			log.Error("failed to start ii", "error", err)
			return err
		}
		for _, channel := range st.Channels {
			if err := dal.WriteFIFO(filepath.Join(m.Paths.Conversations, st.Host, "in"), "/j "+channel.Name, 14); err != nil {
				_ = client.Cmd.Process.Signal(syscall.SIGTERM)
				_ = client.Close()
				log.Error("failed to join channel after starting ii", "channel", channel.Name, "error", err)
				return fmt.Errorf("join %s after starting ii: %w", channel.Name, err)
			}
		}
		done := make(chan error, 1)
		go func() { done <- client.Cmd.Wait() }()
		ticker := time.NewTicker(time.Second)
		terminated := false
		running := true
		stopCh := ctx.Done()
		for running {
			select {
			case <-stopCh:
				stopCh = nil
				log.Info("stop signal received")
				terminated = true
				_ = client.Cmd.Process.Signal(syscall.SIGTERM)
			case waitErr := <-done:
				if waitErr != nil {
					log.Warn("ii exited", "error", waitErr)
				}
				running = false
			case <-ticker.C:
				if !dal.ProcessAlive(st.Owner) {
					log.Info("owner process ended; stopping supervisor")
					terminated = true
					_ = client.Cmd.Process.Signal(syscall.SIGTERM)
				}
			}
		}
		ticker.Stop()
		_ = client.Close()
		if terminated || !dal.ProcessAlive(st.Owner) {
			return nil
		}
		time.Sleep(time.Second)
	}
}

// RunSupervisor is called by the hidden CLI command.
func (m *Manager) RunSupervisor() error { return m.supervise() }
