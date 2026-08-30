//go:build darwin || linux

package chat

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alswl/chatta/pkg/config"
)

type Manager struct {
	Home, StatePath, Host, Channel, II string
	Port                               int
	Paths                              Paths
	State                              ChatSession
	OwnerLookup                        func() (OwnerBinding, error)
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
		if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
			root = strings.TrimSpace(string(out))
		}
		home = WorktreeHome("", filepath.Clean(root))
	}
	paths := ResolvePaths(home)
	return &Manager{Home: home, StatePath: paths.State, Host: host, Port: port, Channel: channel, II: cfg.II, Paths: paths, OwnerLookup: FindAgentOwner}
}

func (m *Manager) findOwner() (OwnerBinding, error) {
	if m.OwnerLookup != nil {
		return m.OwnerLookup()
	}
	return FindAgentOwner()
}
