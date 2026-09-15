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

func (m *ChatService) Send(channel, text string) error {
	st, err := m.Ensure()
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
		resp, err := daemon.Request(m.Paths.ControlSock, daemon.ControlRequest{Op: "privmsg", Target: target, Text: part})
		if err != nil {
			return fmt.Errorf("send: %w", err)
		}
		if !resp.OK {
			return fmt.Errorf("send: %s", resp.Error)
		}
	}
	return nil
}

func (m *ChatService) DM(nick, text string) error {
	st, err := m.Ensure()
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
		resp, err := daemon.Request(m.Paths.ControlSock, daemon.ControlRequest{Op: "privmsg", Target: nick, Text: part})
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

func (m *ChatService) Poll(replay bool) ([]string, error) {
	st, err := m.Ensure()
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
	result := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		if msg.Nick == st.Nick {
			continue
		}
		result = append(result, renderMessage(msg))
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

// Watch streams incoming messages until ctx is cancelled, the owning
// process exits, or the client cannot be recovered.
func (m *ChatService) Watch(ctx context.Context, emit func(string)) error {
	st, err := m.Ensure()
	if err != nil {
		return err
	}
	watchOwner, watchOwnerErr := m.findOwner()
	offset := fileSize(m.Paths.Messages)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	check := time.NewTicker(30 * time.Second)
	defer check.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-check.C:
			if _, err := m.Ensure(); err != nil {
				return err
			}
		case <-ticker.C:
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

func (m *ChatService) Who(channel string) ([]string, error) {
	st, err := m.Ensure()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("not in %s — run: chatta chat join %s", target, target)
	}
	resp, err := daemon.Request(m.Paths.ControlSock, daemon.ControlRequest{Op: "names", Target: target})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("no NAMES reply — run: chatta chat health")
	}
	result := make([]string, 0, len(resp.Members))
	for _, n := range resp.Members {
		trimmed := strings.TrimLeft(n, "@+")
		if trimmed == st.Nick {
			trimmed += " (you)"
		}
		result = append(result, trimmed)
	}
	return result, nil
}
