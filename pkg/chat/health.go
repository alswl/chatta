//go:build darwin || linux

package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var timeReply = regexp.MustCompile(`^\d+ (?:\S+ (?:Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday) |:\S+ 391 \S+ \S+ :(?:Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday) )`)

func (m *Manager) Health(deep bool) HealthReport {
	st := m.State
	if loaded, err := LoadState(m.StatePath); err == nil {
		st = loaded
	}
	r := HealthReport{Owner: ProcessAlive(st.Owner), Supervisor: IsVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint)}
	if st.Host == "" {
		r.Failure = "no session"
		return r
	}
	if !r.Owner {
		r.Failure = "owner has exited"
		return r
	}
	if !r.Supervisor {
		r.Failure = "supervisor is not running"
		return r
	}
	server := filepath.Join(m.Paths.Conversations, st.Host)
	for _, channel := range st.Channels {
		if !FIFOReader(filepath.Join(server, channel.Name, "in")) {
			r.Failure = "channel not joined: " + channel.Name
			return r
		}
	}
	r.JoinedChannels = true
	if len(st.Channels) > 0 {
		r.ClientReader = FIFOReader(filepath.Join(server, st.Channels[0].Name, "in"))
	}
	if !r.ClientReader {
		r.Failure = "ii is not reading a channel FIFO"
		return r
	}
	if !deep {
		r.ServerLink, r.Membership = true, true
		return r
	}
	out := filepath.Join(server, "out")
	before := fileSize(out)
	if err := WriteFIFO(filepath.Join(server, "in"), "/TIME", 1); err != nil {
		r.Failure = "server input unavailable: " + err.Error()
		return r
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		lines, _, _ := Tail(out, before)
		for _, line := range lines {
			if timeReply.MatchString(line) {
				r.ServerLink = true
				break
			}
		}
		if r.ServerLink {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !r.ServerLink {
		r.Failure = "no reply to /TIME"
		return r
	}
	r.Membership = m.confirmMembership(st, st.HomeChannel.Name)
	if !r.Membership {
		r.Failure = "server did not confirm membership in " + st.HomeChannel.Name
	}
	return r
}

func (m *Manager) confirmMembership(st ChatSession, target string) bool {
	server := filepath.Join(m.Paths.Conversations, st.Host)
	out := filepath.Join(server, "out")
	offset := fileSize(out)
	if err := WriteFIFO(filepath.Join(server, "in"), "/NAMES "+target, 1); err != nil {
		return false
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		lines, _, _ := Tail(out, offset)
		for _, line := range lines {
			channel, names, ok := ParseNames(line)
			if !ok || channel != target {
				continue
			}
			for _, name := range names {
				if strings.TrimLeft(name, "@+") == st.Nick {
					return true
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (m *Manager) Ensure() (ChatSession, error) {
	st, err := LoadState(m.StatePath)
	if err != nil {
		return ChatSession{}, fmt.Errorf("no session — run: chatta chat start <nick> [role]")
	}
	if !ProcessAlive(st.Owner) {
		return st, fmt.Errorf("the agent session that owns this client has exited")
	}
	m.State = st
	m.Paths = ResolvePaths(m.Home)
	if !IsVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint) || !m.Health(false).ClientReader {
		if st.SupervisorPID > 0 {
			_ = StopVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint)
		}
		if err := ReapStrayII(m.Paths.Conversations); err != nil {
			return st, fmt.Errorf("reap stale ii client: %w", err)
		}
		pid, err := m.spawnSupervisor()
		if err != nil {
			return st, err
		}
		st.SupervisorPID = pid
		st.SupervisorStartFingerprint = ProcessStart(pid)
		if st.SupervisorStartFingerprint == "" {
			return st, fmt.Errorf("could not establish supervisor process identity")
		}
		if err := SaveState(m.StatePath, st); err != nil {
			return st, err
		}
		m.State = st
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		r := m.Health(false)
		if r.Owner && r.Supervisor && r.ClientReader && m.confirmRememberedMembership(st) {
			return st, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return st, fmt.Errorf("chat client recovery timed out")
}

func (m *Manager) confirmRememberedMembership(st ChatSession) bool {
	for _, channel := range st.Channels {
		if !m.confirmMembership(st, channel.Name) {
			return false
		}
	}
	return true
}

func (m *Manager) Join(channel string) error {
	st, err := m.Ensure()
	if err != nil {
		return err
	}
	target := NormalizeChannel(channel)
	for _, c := range st.Channels {
		if c.Name == target {
			return nil
		}
	}
	if err := WriteFIFO(filepath.Join(m.Paths.Conversations, st.Host, "in"), "/j "+target, 1); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if m.confirmMembership(st, target) {
			st.Channels = append(st.Channels, ChannelMembership{Name: target, Kind: "custom", JoinedAt: time.Now(), Confirmed: true})
			return SaveState(m.StatePath, st)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("the server did not confirm membership in %s", target)
}

func (m *Manager) Part(channel, reason string) error {
	st, err := m.Ensure()
	if err != nil {
		return err
	}
	if err := CanPart(channel, st.HomeChannel.Name); err != nil {
		return err
	}
	target := NormalizeChannel(channel)
	kept := st.Channels[:0]
	for _, c := range st.Channels {
		if c.Name != target {
			kept = append(kept, c)
		}
	}
	if err := WriteFIFO(filepath.Join(m.Paths.Conversations, st.Host, target, "in"), "/l "+strings.TrimSpace(reason), 1); err != nil {
		return err
	}
	st.Channels = kept
	return SaveState(m.StatePath, st)
}
