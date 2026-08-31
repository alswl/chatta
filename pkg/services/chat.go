// Package services contains CLI-facing business use cases: one method per
// `chatta chat` verb. It is the entry point cobra commands call, and it
// orchestrates a single managers.Manager per invocation.
package services

import (
	"context"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/config"
	"github.com/alswl/chatta/pkg/managers"
)

type ChatService struct {
	manager *managers.Manager
}

func NewChatService(cfg config.ChatConfig) *ChatService {
	return &ChatService{manager: managers.NewManager(cfg)}
}

func (s *ChatService) StartSession(nick, role string, takeover bool) error {
	return s.manager.Start(nick, role, takeover)
}

func (s *ChatService) SessionHealth(deep bool) common.HealthReport {
	return s.manager.Health(deep)
}

func (s *ChatService) StopSession(force bool) error {
	return s.manager.Stop(force)
}

func (s *ChatService) JoinChannel(channel string) error {
	return s.manager.Join(channel)
}

func (s *ChatService) LeaveChannel(channel, reason string) error {
	return s.manager.Part(channel, reason)
}

func (s *ChatService) ChannelMembers(channel string) ([]string, error) {
	return s.manager.Who(channel)
}

func (s *ChatService) SendMessage(channel, text string) error {
	return s.manager.Send(channel, text)
}

func (s *ChatService) SendDirectMessage(nick, text string) error {
	return s.manager.DM(nick, text)
}

func (s *ChatService) ReadInbox(replay bool) ([]string, error) {
	return s.manager.Poll(replay)
}

func (s *ChatService) WatchInbox(ctx context.Context, emit func(string)) error {
	return s.manager.Watch(ctx, emit)
}

func (s *ChatService) ListClients() ([]common.ClientSurvey, error) {
	return s.manager.Survey()
}

func (s *ChatService) CollectGarbage(dryRun, prune bool) (string, error) {
	return s.manager.GC(dryRun, prune)
}

func (s *ChatService) RunSupervisor() error {
	return s.manager.RunSupervisor()
}
