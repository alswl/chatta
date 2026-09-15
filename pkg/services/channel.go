//go:build darwin || linux

package services

import (
	"fmt"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/daemon"
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
	resp, err := daemon.Request(m.Paths.ControlSock, daemon.ControlRequest{Op: "join", Target: target})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("the server did not confirm membership in %s", target)
	}
	st.Channels = append(st.Channels, common.ChannelMembership{Name: target, Kind: "custom", JoinedAt: time.Now(), Confirmed: true})
	return dal.SaveState(m.StatePath, st)
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
	if _, err := daemon.Request(m.Paths.ControlSock, daemon.ControlRequest{Op: "part", Target: target, Reason: reason}); err != nil {
		return err
	}
	st.Channels = kept
	return dal.SaveState(m.StatePath, st)
}
