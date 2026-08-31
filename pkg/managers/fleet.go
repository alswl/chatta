//go:build darwin || linux

package managers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/dal"
)

func (m *Manager) Stop(force bool) error {
	st, err := dal.LoadState(m.StatePath)
	if err != nil {
		return err
	}
	if !force && dal.ProcessAlive(st.Owner) {
		current, ownerErr := dal.FindAgentOwner()
		if ownerErr != nil || !dal.SameOwner(st.Owner, current) {
			return fmt.Errorf("this client belongs to another live agent; use --force after confirmation")
		}
	}
	server := filepath.Join(m.Paths.Conversations, st.Host)
	_ = dal.WriteFIFO(filepath.Join(server, "in"), "/q leaving", 0)
	if st.SupervisorPID > 0 {
		if err := dal.StopVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint); err != nil {
			return err
		}
	}
	if err := dal.ReapStrayII(m.Paths.Conversations); err != nil {
		return err
	}
	st.SupervisorPID = 0
	st.SupervisorStartFingerprint = ""
	return dal.SaveState(m.StatePath, st)
}

func (m *Manager) Survey() ([]common.ClientSurvey, error) {
	rows := make([]common.ClientSurvey, 0)
	for _, home := range m.clientHomes() {
		path := filepath.Join(home, "state.json")
		st, err := dal.LoadState(path)
		if err != nil {
			continue
		}
		ownerState := "ended"
		if dal.ProcessAlive(st.Owner) {
			ownerState = "alive"
		}
		supervisorState := "down"
		if dal.IsVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint) {
			supervisorState = "alive"
		}
		clientState := "down"
		if dal.FIFOReader(filepath.Join(home, "irc", st.Host, st.HomeChannel.Name, "in")) {
			clientState = "alive"
		}
		iiPIDs, _ := dal.IIPIDs(filepath.Join(home, "irc"))
		if clientState == "down" && len(iiPIDs) > 0 {
			clientState = "stray"
		}
		eligibility := "live owner"
		if ownerState != "alive" {
			eligibility = "owner ended"
		}
		rows = append(rows, common.ClientSurvey{ClientHome: filepath.Dir(path), SessionSummary: st.Nick, OwnerState: ownerState, SupervisorState: supervisorState, ClientProcessState: clientState, IIProcessCount: len(iiPIDs), CleanupEligibility: eligibility})
	}
	return rows, nil
}

func (m *Manager) clientHomes() []string {
	roots := []string{m.Home, filepath.Dir(m.Home)}
	if userHome, err := os.UserHomeDir(); err == nil {
		agentRoot := filepath.Join(userHome, ".irc-agent")
		if withinRoot(m.Home, agentRoot) {
			roots = append(roots, agentRoot, filepath.Join(agentRoot, "clients"))
		}
	}
	homes := make(map[string]struct{})
	for _, root := range roots {
		if root == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "state.json")); err == nil {
			homes[root] = struct{}{}
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			home := filepath.Join(root, entry.Name())
			if _, err := os.Stat(filepath.Join(home, "state.json")); err == nil {
				homes[home] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(homes))
	for home := range homes {
		result = append(result, home)
	}
	return result
}

func withinRoot(path, root string) bool {
	path, root = filepath.Clean(path), filepath.Clean(root)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

func (m *Manager) GC(dryRun, prune bool) (string, error) {
	rows, err := m.Survey()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, row := range rows {
		_, _ = fmt.Fprintf(&b, "%s: %s, ii=%d (%s)\n", row.SessionSummary, row.ClientProcessState, row.IIProcessCount, row.CleanupEligibility)
		orphanII := row.SupervisorState != "alive" && row.IIProcessCount > 0
		if dryRun {
			if orphanII {
				b.WriteString("  would reap orphan ii process(es)\n")
			}
			continue
		}
		st, err := dal.LoadState(filepath.Join(row.ClientHome, "state.json"))
		if err != nil {
			continue
		}
		if orphanII {
			if err := dal.ReapStrayII(filepath.Join(row.ClientHome, "irc")); err != nil {
				return "", err
			}
			b.WriteString("  reaped orphan ii process(es)\n")
		}
		if row.CleanupEligibility == "live owner" {
			continue
		}
		if st.SupervisorPID > 0 {
			if err := dal.StopVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint); err != nil {
				return "", err
			}
		}
		if !orphanII {
			if err := dal.ReapStrayII(filepath.Join(row.ClientHome, "irc")); err != nil {
				return "", err
			}
		}
		if prune {
			if err := os.RemoveAll(row.ClientHome); err != nil {
				return "", err
			}
		}
	}
	return b.String(), nil
}

func (m *Manager) SaveSurvey(path string, rows []common.ClientSurvey) error {
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
