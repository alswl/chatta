//go:build darwin || linux

package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (m *Manager) Stop(force bool) error {
	st, err := LoadState(m.StatePath)
	if err != nil {
		return err
	}
	if !force && ProcessAlive(st.Owner) {
		current, ownerErr := FindAgentOwner()
		if ownerErr != nil || !sameOwner(st.Owner, current) {
			return fmt.Errorf("this client belongs to another live agent; use --force after confirmation")
		}
	}
	server := filepath.Join(m.Paths.Conversations, st.Host)
	_ = WriteFIFO(filepath.Join(server, "in"), "/q leaving", 0)
	if st.SupervisorPID > 0 {
		if err := StopVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint); err != nil {
			return err
		}
	}
	if err := ReapStrayII(m.Paths.Conversations); err != nil {
		return err
	}
	st.SupervisorPID = 0
	st.SupervisorStartFingerprint = ""
	return SaveState(m.StatePath, st)
}

func (m *Manager) Survey() ([]ClientSurvey, error) {
	root := filepath.Dir(m.Home)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	rows := make([]ClientSurvey, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(root, e.Name(), "state.json")
		st, err := LoadState(path)
		if err != nil {
			continue
		}
		status := "down"
		if IsVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint) {
			status = "alive"
		}
		eligibility := "live owner"
		if !ProcessAlive(st.Owner) {
			eligibility = "owner ended"
		}
		rows = append(rows, ClientSurvey{ClientHome: filepath.Dir(path), SessionSummary: st.Nick, ClientProcessState: status, CleanupEligibility: eligibility})
	}
	return rows, nil
}

func (m *Manager) GC(dryRun, prune bool) (string, error) {
	rows, err := m.Survey()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, row := range rows {
		b.WriteString(fmt.Sprintf("%s: %s (%s)\n", row.SessionSummary, row.ClientProcessState, row.CleanupEligibility))
		if dryRun || row.CleanupEligibility == "live owner" {
			continue
		}
		st, err := LoadState(filepath.Join(row.ClientHome, "state.json"))
		if err != nil {
			continue
		}
		if st.SupervisorPID > 0 {
			if err := StopVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint); err != nil {
				return "", err
			}
		}
		if err := ReapStrayII(filepath.Join(row.ClientHome, "irc")); err != nil {
			return "", err
		}
		if prune {
			if err := os.RemoveAll(row.ClientHome); err != nil {
				return "", err
			}
		}
	}
	return b.String(), nil
}

func (m *Manager) SaveSurvey(path string, rows []ClientSurvey) error {
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
