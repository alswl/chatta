//go:build darwin || linux

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/daemon"
	"github.com/alswl/chatta/pkg/dal"
)

func (m *ChatService) Stop(ctx context.Context, force bool) error {
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
	_, _ = daemon.Request(ctx, m.Paths.ControlSock, daemon.ControlRequest{Op: "quit", Reason: "leaving"})
	if st.SupervisorPID > 0 {
		if err := dal.StopVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint); err != nil {
			return err
		}
	}
	st.SupervisorPID = 0
	st.SupervisorStartFingerprint = ""
	return dal.SaveState(m.StatePath, st)
}

func (m *ChatService) Survey(ctx context.Context) ([]common.ClientSurvey, error) {
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
		paths := dal.ResolvePaths(home)
		if resp, err := daemon.Request(ctx, paths.ControlSock, daemon.ControlRequest{Op: "status"}); err == nil && resp.OK && resp.Status != nil && resp.Status.Connected {
			clientState = "alive"
		}
		eligibility := "live owner"
		if ownerState != "alive" {
			eligibility = "owner ended"
		}
		rows = append(rows, common.ClientSurvey{ClientHome: filepath.Dir(path), SessionSummary: st.Nick, OwnerState: ownerState, SupervisorState: supervisorState, ClientProcessState: clientState, CleanupEligibility: eligibility})
	}
	return rows, nil
}

func (m *ChatService) clientHomes() []string {
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

// GC renders what GCReport returns. Both perform the cleanup, so a caller
// picks one or the other.
func (m *ChatService) GC(ctx context.Context, dryRun, prune bool) (string, error) {
	rows, err := m.GCReport(ctx, dryRun, prune)
	var b strings.Builder
	for _, row := range rows {
		_, _ = fmt.Fprintf(&b, "%s: %s (%s)\n", row.SessionSummary, row.ClientProcessState, row.CleanupEligibility)
	}
	return b.String(), err
}

// GCReport performs the cleanup and returns the clients it considered, in
// the order it considered them.
func (m *ChatService) GCReport(ctx context.Context, dryRun, prune bool) ([]common.ClientSurvey, error) {
	rows, err := m.Survey(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if dryRun {
			continue
		}
		if row.CleanupEligibility == "live owner" {
			continue
		}
		st, err := dal.LoadState(filepath.Join(row.ClientHome, "state.json"))
		if err != nil {
			continue
		}
		if st.SupervisorPID > 0 {
			if err := dal.StopVerifiedSupervisor(st.SupervisorPID, st.SupervisorStartFingerprint); err != nil {
				return nil, err
			}
		}
		if prune {
			if err := os.RemoveAll(row.ClientHome); err != nil {
				return nil, err
			}
		}
	}
	return rows, nil
}

func (m *ChatService) SaveSurvey(path string, rows []common.ClientSurvey) error {
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
