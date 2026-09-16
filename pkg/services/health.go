//go:build darwin || linux

package services

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/daemon"
	"github.com/alswl/chatta/pkg/dal"
)

func (m *ChatService) Health(deep bool) common.HealthReport {
	st := m.State
	if loaded, err := dal.LoadState(m.StatePath); err == nil {
		st = loaded
	} else if migration := preUpgradeMigrationMessage(m.Home, err); migration != "" {
		return common.HealthReport{Failure: migration}
	}
	r := common.HealthReport{Owner: dal.ProcessAlive(st.Owner), Supervisor: dal.IsVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint)}
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
	resp, err := daemon.Request(m.Paths.ControlSock, daemon.ControlRequest{Op: "status"})
	if err != nil || !resp.OK || resp.Status == nil {
		r.Failure = "transport not connected"
		if resp.Error != "" {
			r.Failure = resp.Error
		}
		return r
	}
	r.Connected = resp.Status.Connected
	confirmed := map[string]bool{}
	for _, c := range resp.Status.Channels {
		confirmed[c] = true
	}
	for _, channel := range st.Channels {
		if !confirmed[channel.Name] {
			r.Failure = "channel not joined: " + channel.Name
			return r
		}
	}
	r.JoinedChannels = true
	if !deep {
		r.ServerLink, r.Membership = true, true
		return r
	}
	r.ServerLink = resp.Status.Registered
	if !r.ServerLink {
		r.Failure = "not registered with server"
		return r
	}
	r.Membership = r.JoinedChannels
	if !r.Membership {
		r.Failure = "server did not confirm membership in " + st.HomeChannel.Name
	}
	return r
}

// preUpgradeMigrationMessage detects a pre-upgrade home: state failed to
// load with a schema-version error, or the home still has the retired
// irc/ conversation tree with no control.sock (FR-013).
func preUpgradeMigrationMessage(home string, loadErr error) string {
	const migration = "this client home was created by an older Chatta and cannot be reused;\nrun: chatta chat session stop --force && chatta chat session start <nick>"
	if loadErr != nil && strings.Contains(loadErr.Error(), "unsupported state schema version") {
		return migration
	}
	if _, err := os.Stat(home + "/irc"); err == nil {
		if _, err := os.Stat(home + "/control.sock"); os.IsNotExist(err) {
			return migration
		}
	}
	return ""
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (m *ChatService) Ensure() (common.ChatSession, error) {
	st, err := dal.LoadState(m.StatePath)
	if err != nil {
		if migration := preUpgradeMigrationMessage(m.Home, err); migration != "" {
			return common.ChatSession{}, fmt.Errorf("%s", migration)
		}
		return common.ChatSession{}, fmt.Errorf("no session — run: chatta chat start <nick> [role]")
	}
	if !dal.ProcessAlive(st.Owner) {
		return st, fmt.Errorf("the agent session that owns this client has exited")
	}
	m.State = st
	m.Paths = dal.ResolvePaths(m.Home)
	needsRecovery := !dal.IsVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint)
	if !needsRecovery {
		resp, err := daemon.Request(m.Paths.ControlSock, daemon.ControlRequest{Op: "status"})
		needsRecovery = err != nil || !resp.OK
	}
	if needsRecovery {
		if st.SupervisorPID > 0 {
			_ = dal.StopVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint)
		}
		pid, err := m.spawnSupervisor()
		if err != nil {
			return st, err
		}
		st.SupervisorPID = pid
		st.SupervisorStartFingerprint = dal.ProcessStart(pid)
		if st.SupervisorStartFingerprint == "" {
			return st, fmt.Errorf("could not establish supervisor process identity")
		}
		if err := dal.SaveState(m.StatePath, st); err != nil {
			return st, err
		}
		m.State = st
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := daemon.Request(m.Paths.ControlSock, daemon.ControlRequest{Op: "status"})
		if err == nil && resp.OK && resp.Status != nil && resp.Status.Registered && dal.IsVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint) {
			return st, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return st, fmt.Errorf("chat client recovery timed out")
}
