//go:build darwin || linux

package chat

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
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
	if old, err := LoadState(m.StatePath); err == nil {
		if !takeover && ProcessAlive(old.Owner) && !sameOwner(old.Owner, owner) {
			return fmt.Errorf("this client belongs to another live agent (nick %s); use --takeover only after confirmation", old.Nick)
		}
		if old.SupervisorPID > 0 {
			_ = StopVerifiedSupervisor(old.SupervisorPID, old.SupervisorStartFingerprint)
		}
	}
	if err := ReapStrayII(m.Paths.Conversations); err != nil {
		return fmt.Errorf("reap stale ii client: %w", err)
	}

	normalized := NormalizeChannel(m.Channel)
	session := ChatSession{
		SchemaVersion: StateSchemaVersion,
		Nick:          nick, Role: role, Host: m.Host, Port: m.Port,
		HomeChannel: ChannelMembership{Name: normalized, Kind: "lobby", Confirmed: false},
		Channels:    []ChannelMembership{{Name: normalized, Kind: "lobby", Confirmed: false}},
		Owner:       owner, SessionID: ownerSessionID(owner), StartedAt: time.Now(),
	}
	if err := SaveState(m.StatePath, session); err != nil {
		return err
	}
	m.State = session
	serverOut := filepath.Join(m.Paths.Conversations, session.Host, "out")
	serverOffset := fileSize(serverOut)
	pid, err := m.spawnSupervisor()
	if err != nil {
		return err
	}
	session.SupervisorPID = pid
	session.SupervisorStartFingerprint = ProcessStart(pid)
	if session.SupervisorStartFingerprint == "" {
		return fmt.Errorf("could not establish supervisor process identity")
	}
	if err := SaveState(m.StatePath, session); err != nil {
		_ = StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
		return err
	}
	m.State = session
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if nicknameTaken(serverOut, serverOffset, nick) {
			_ = StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
			return fmt.Errorf("the nick %q is already in use on this server; choose a distinct nick", nick)
		}
		if report := m.Health(true); report.Owner && report.Supervisor && report.ClientReader && report.ServerLink && report.Membership {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	_ = StopVerifiedSupervisor(pid, session.SupervisorStartFingerprint)
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
	lines, _, _ := Tail(path, offset)
	for _, line := range lines {
		if strings.Contains(line, nick+" Nickname already in use") || (strings.Contains(line, nick) && strings.Contains(strings.ToLower(line), "nickname already in use")) {
			return true
		}
	}
	return false
}

func mustOpenLog(path string) (*os.File, *os.File) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return os.Stderr, os.Stderr
	}
	return f, f
}

func (m *Manager) supervise() error {
	lock, err := LockHome(m.Paths.Lock)
	if err != nil {
		return fmt.Errorf("another supervisor owns this client: %w", err)
	}
	defer func() { _ = lock.Close() }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(stop)
	for {
		st, err := LoadState(m.StatePath)
		if err != nil {
			return err
		}
		if !ProcessAlive(st.Owner) {
			return nil
		}
		client := &II{Paths: m.Paths}
		if err := client.Start(st, m.II); err != nil {
			return err
		}
		for _, channel := range st.Channels {
			if err := WriteFIFO(filepath.Join(m.Paths.Conversations, st.Host, "in"), "/j "+channel.Name, 14); err != nil {
				_ = client.Cmd.Process.Signal(syscall.SIGTERM)
				_ = client.Close()
				return fmt.Errorf("join %s after starting ii: %w", channel.Name, err)
			}
		}
		done := make(chan error, 1)
		go func() { done <- client.Cmd.Wait() }()
		ticker := time.NewTicker(time.Second)
		terminated := false
		running := true
		for running {
			select {
			case <-stop:
				terminated = true
				_ = client.Cmd.Process.Signal(syscall.SIGTERM)
			case waitErr := <-done:
				if waitErr != nil {
					_, _ = fmt.Fprintf(client.Log, "ii exited: %v\n", waitErr)
				}
				running = false
			case <-ticker.C:
				if !ProcessAlive(st.Owner) {
					terminated = true
					_ = client.Cmd.Process.Signal(syscall.SIGTERM)
				}
			}
		}
		ticker.Stop()
		_ = client.Close()
		if terminated || !ProcessAlive(st.Owner) {
			return nil
		}
		time.Sleep(time.Second)
	}
}

// RunSupervisor is called by the hidden CLI command.
func (m *Manager) RunSupervisor() error { return m.supervise() }
