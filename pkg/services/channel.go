//go:build darwin || linux

package services

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/alswl/chatta/integrations/ii"
	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/dal"
)

func (m *ChatService) Join(channel string) error {
	st, err := m.Ensure()
	if err != nil {
		return err
	}
	target := dal.NormalizeChannel(channel)
	for _, c := range st.Channels {
		if c.Name == target {
			return nil
		}
	}
	if err := ii.WriteFIFO(filepath.Join(m.Paths.Conversations, st.Host, "in"), "/j "+target, 1); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if m.confirmMembership(st, target) {
			st.Channels = append(st.Channels, common.ChannelMembership{Name: target, Kind: "custom", JoinedAt: time.Now(), Confirmed: true})
			return dal.SaveState(m.StatePath, st)
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("the server did not confirm membership in %s", target)
}

func (m *ChatService) Part(channel, reason string) error {
	st, err := m.Ensure()
	if err != nil {
		return err
	}
	if err := dal.CanPart(channel, st.HomeChannel.Name); err != nil {
		return err
	}
	target := dal.NormalizeChannel(channel)
	kept := st.Channels[:0]
	for _, c := range st.Channels {
		if c.Name != target {
			kept = append(kept, c)
		}
	}
	if err := ii.WriteFIFO(filepath.Join(m.Paths.Conversations, st.Host, target, "in"), "/l "+strings.TrimSpace(reason), 1); err != nil {
		return err
	}
	st.Channels = kept
	return dal.SaveState(m.StatePath, st)
}
