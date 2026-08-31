// Package managers holds per-entity business logic for a chat session: it
// orchestrates the dal layer's persistence and OS primitives but never
// touches them directly (state files, FIFOs, processes) except through dal.
package managers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/config"
	"github.com/alswl/chatta/pkg/dal"
)

type Manager struct {
	Home, StatePath, Host, Channel, II string
	Port                               int
	Paths                              dal.Paths
	State                              common.ChatSession
	OwnerLookup                        func() (common.OwnerBinding, error)
	Executable                         string
}

func NewManager(cfg config.ChatConfig) *Manager {
	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Port
	if port == 0 {
		port = 6667
	}
	channel := cfg.Channel
	if channel == "" {
		channel = "#agents"
	}
	home := cfg.Home
	if home == "" {
		cwd, _ := os.Getwd()
		root := cwd
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output(); err == nil {
			root = strings.TrimSpace(string(out))
		}
		home = dal.WorktreeHome("", filepath.Clean(root))
	}
	paths := dal.ResolvePaths(home)
	return &Manager{Home: home, StatePath: paths.State, Host: host, Port: port, Channel: channel, II: cfg.II, Paths: paths, OwnerLookup: dal.FindAgentOwner}
}

func (m *Manager) findOwner() (common.OwnerBinding, error) {
	if m.OwnerLookup != nil {
		return m.OwnerLookup()
	}
	return dal.FindAgentOwner()
}
