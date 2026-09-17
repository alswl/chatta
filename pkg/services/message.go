//go:build darwin || linux

package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/daemon"
	"github.com/alswl/chatta/pkg/dal"
)

func splitUTF8(text string, limit int) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		current := strings.Builder{}
		size := 0
		for _, r := range line {
			piece := string(r)
			n := len([]byte(piece))
			if size+n > limit && current.Len() > 0 {
				out = append(out, current.String())
				current.Reset()
				size = 0
			}
			current.WriteString(piece)
			size += n
		}
		if current.Len() > 0 {
			out = append(out, current.String())
		}
	}
	return out
}

func (m *ChatService) Send(ctx context.Context, channel, text string) error {
	st, err := m.Ensure(ctx)
	if err != nil {
		return err
	}
	target := st.HomeChannel.Name
	if channel != "" {
		target = dal.NormalizeChannel(channel)
	}
	found := false
	for _, c := range st.Channels {
		if c.Name == target {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("not in %s — run: chatta chat join %s", target, target)
	}
	parts := splitUTF8(text, 400)
	if len(parts) == 0 {
		return fmt.Errorf("message must contain non-whitespace text")
	}
	for _, part := range parts {
		resp, err := daemon.Request(ctx, m.Paths.ControlSock, daemon.ControlRequest{Op: "privmsg", Target: target, Text: part})
		if err != nil {
			return fmt.Errorf("send: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("send: %s", resp.Error)
		}
	}
	return nil
}

func (m *ChatService) DM(ctx context.Context, nick, text string) error {
	st, err := m.Ensure(ctx)
	if err != nil {
		return err
	}
	if strings.HasPrefix(nick, "#") || nick == st.Nick {
		return fmt.Errorf("dm takes another nick")
	}
	parts := splitUTF8(text, 400)
	if len(parts) == 0 {
		return fmt.Errorf("message must contain non-whitespace text")
	}
	for _, part := range parts {
		resp, err := daemon.Request(ctx, m.Paths.ControlSock, daemon.ControlRequest{Op: "privmsg", Target: nick, Text: part})
		if err != nil {
			return err
		}
		if !resp.OK {
			if resp.Code == daemon.CodeNoSuchNick {
				return fmt.Errorf("direct message target %q is absent", nick)
			}
			return fmt.Errorf("%s", resp.Error)
		}
	}
	return nil
}

// Poll renders what PollMessages returns. Both consume the cursor, so a
// caller picks one or the other -- never both for a single invocation.
func (m *ChatService) Poll(ctx context.Context, replay bool) ([]string, error) {
	msgs, err := m.PollMessages(ctx, replay)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, renderMessage(msg))
	}
	return result, nil
}

// PollMessages advances the cursor and returns the messages it passed, as
// stored, for callers that want the data rather than the rendering.
func (m *ChatService) PollMessages(ctx context.Context, replay bool) ([]common.StoredMessage, error) {
	st, err := m.Ensure(ctx)
	if err != nil {
		return nil, err
	}
	cursorPath := dal.CursorPath(m.Home, dal.InvocationSessionID(st.SessionID))
	cursor, _ := dal.LoadCursors(cursorPath)
	if replay {
		cursor.Offset = 0
	}
	msgs, next, err := dal.ReadMessages(m.Paths.Messages, cursor.Offset)
	if err != nil {
		return nil, err
	}
	result := make([]common.StoredMessage, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Nick == st.Nick {
			continue
		}
		result = append(result, msg)
	}
	cursor.Offset = next
	cursor.InvokerKey = st.SessionID
	return result, dal.SaveCursors(cursorPath, cursor)
}

func renderMessage(msg common.StoredMessage) string {
	stamp := time.Unix(msg.TS, 0).Local().Format("2006-01-02 15:04:05")
	label := msg.Target
	if msg.Kind == "direct" {
		label = "DM"
	}
	return fmt.Sprintf("%s (%s) <%s> %s", stamp, label, msg.Nick, msg.Text)
}

// renderNotice formats a transport event for the same stream as messages,
// using the `-!-` marker the inbox already carries joins and parts with.
func renderNotice(text string) string {
	return fmt.Sprintf("%s -!- %s", time.Now().Format("2006-01-02 15:04:05"), text)
}

// transportGeneration reports the supervisor's connection counter and
// whether it currently holds a usable connection at all.
func (m *ChatService) transportGeneration(ctx context.Context) (int64, bool) {
	resp, err := daemon.Request(ctx, m.Paths.ControlSock, daemon.ControlRequest{Op: "status"})
	if err != nil || !resp.OK || resp.Status == nil || !resp.Status.Registered {
		return 0, false
	}
	return resp.Status.Generation, true
}

// Watch streams incoming messages until ctx is cancelled, the owning
// process exits, or the client cannot be recovered.
func (m *ChatService) Watch(ctx context.Context, emit func(string)) error {
	st, err := m.Ensure(ctx)
	if err != nil {
		return err
	}
	watchOwner, watchOwnerErr := m.findOwner()
	offset := fileSize(m.Paths.Messages)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	check := time.NewTicker(30 * time.Second)
	defer check.Stop()
	// A watcher that goes quiet looks the same whether the channel is idle
	// or the link is gone, so every interruption is announced -- including
	// one that is repaired too quickly to be observed while it is down.
	generation, live := m.transportGeneration(ctx)
	down := !live
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-check.C:
			if _, err := m.Ensure(ctx); err != nil {
				return err
			}
		case <-ticker.C:
		}
		switch current, ok := m.transportGeneration(ctx); {
		case !ok:
			if !down {
				down = true
				emit(renderNotice(fmt.Sprintf("connection to %s:%d lost; reconnecting", st.Host, st.Port)))
			}
		case down || current != generation:
			down, generation = false, current
			emit(renderNotice(fmt.Sprintf("connection to %s:%d re-established", st.Host, st.Port)))
		}
		if watchOwnerErr == nil && !dal.ProcessAlive(watchOwner) {
			return nil
		}
		msgs, next, err := dal.ReadMessages(m.Paths.Messages, offset)
		if err != nil {
			return err
		}
		for _, msg := range msgs {
			if msg.Nick == st.Nick {
				continue
			}
			emit(renderMessage(msg))
		}
		offset = next
	}
}

// Who renders what Members returns.
func (m *ChatService) Who(ctx context.Context, channel string) ([]string, error) {
	members, err := m.Members(ctx, channel)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(members.Members))
	for _, member := range members.Members {
		line := member.Nick
		if member.You {
			line += " (you)"
		}
		result = append(result, line)
	}
	return result, nil
}

// Members reports the channel's occupants, flagging the caller rather than
// suffixing its nick.
func (m *ChatService) Members(ctx context.Context, channel string) (common.ChannelMembers, error) {
	st, err := m.Ensure(ctx)
	if err != nil {
		return common.ChannelMembers{}, err
	}
	target := st.HomeChannel.Name
	if channel != "" {
		target = dal.NormalizeChannel(channel)
	}
	joined := false
	for _, membership := range st.Channels {
		if membership.Name == target {
			joined = true
			break
		}
	}
	if !joined {
		return common.ChannelMembers{}, fmt.Errorf("not in %s — run: chatta chat join %s", target, target)
	}
	resp, err := daemon.Request(ctx, m.Paths.ControlSock, daemon.ControlRequest{Op: "names", Target: target})
	if err != nil {
		return common.ChannelMembers{}, err
	}
	if !resp.OK {
		return common.ChannelMembers{}, fmt.Errorf("no NAMES reply — run: chatta chat health")
	}
	result := common.ChannelMembers{Channel: target, Members: make([]common.ChannelMember, 0, len(resp.Members))}
	for _, n := range resp.Members {
		trimmed := strings.TrimLeft(n, "@+")
		result.Members = append(result.Members, common.ChannelMember{Nick: trimmed, You: trimmed == st.Nick})
	}
	return result, nil
}
