// Package services contains CLI-facing business use cases: one method per
// `chatta chat` verb, plus the state/process orchestration that used to live
// in pkg/managers. It is the entry point cobra commands call.
package services

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

type ChatService struct {
	Home, StatePath, Host, Channel string
	Port                           int
	Paths                          dal.Paths
	State                          common.ChatSession
	OwnerLookup                    func() (common.OwnerBinding, error)
	Executable                     string
}

func NewChatService(cfg config.ChatConfig) *ChatService {
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
	return &ChatService{Home: home, StatePath: paths.State, Host: host, Port: port, Channel: channel, Paths: paths, OwnerLookup: dal.FindAgentOwner}
}

func (s *ChatService) findOwner() (common.OwnerBinding, error) {
	if s.OwnerLookup != nil {
		return s.OwnerLookup()
	}
	return dal.FindAgentOwner()
}
