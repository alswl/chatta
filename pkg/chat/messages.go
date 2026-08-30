//go:build darwin || linux

package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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

func (m *Manager) Send(channel, text string) error {
	st, err := m.Ensure()
	if err != nil {
		return err
	}
	target := st.HomeChannel.Name
	if channel != "" {
		target = NormalizeChannel(channel)
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
	if !m.confirmMembership(st, target) {
		return fmt.Errorf("server did not confirm membership in %s", target)
	}
	fifo := filepath.Join(m.Paths.Conversations, st.Host, target, "in")
	for _, part := range parts {
		if err := WriteFIFO(fifo, part, 1); err != nil {
			return fmt.Errorf("send: %w", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

func (m *Manager) DM(nick, text string) error {
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
	server := filepath.Join(m.Paths.Conversations, st.Host)
	query := filepath.Join(server, nick, "in")
	serverOut := filepath.Join(server, "out")
	offset := fileSize(serverOut)
	firstUnsent := 0
	if _, err := os.Stat(query); os.IsNotExist(err) {
		if err := WriteFIFO(filepath.Join(server, "in"), "/j "+nick+" "+parts[0], 1); err != nil {
			return err
		}
		firstUnsent = 1
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(query); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if _, err := os.Stat(query); err != nil {
			return fmt.Errorf("direct message target %q did not open query FIFO: %w", nick, err)
		}
	}
	for _, part := range parts[firstUnsent:] {
		if err := WriteFIFO(query, part, 1); err != nil {
			return err
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := waitForAbsentNick(serverOut, offset, nick); err != nil {
		return err
	}
	return nil
}

func waitForAbsentNick(path string, offset int64, nick string) error {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		lines, _, _ := Tail(path, offset)
		for _, line := range lines {
			if strings.Contains(line, nick) && strings.Contains(strings.ToLower(line), "no such nick") {
				return fmt.Errorf("direct message target %q is absent", nick)
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

func (m *Manager) Poll(replay bool) ([]string, error) {
	st, err := m.Ensure()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(m.Paths.Conversations, st.Host)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	cursorPath := CursorPath(m.Home, InvocationSessionID(st.SessionID))
	cursor, _ := LoadCursors(cursorPath)
	if replay {
		cursor.Offsets = map[string]int64{}
	}
	var result []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		out := filepath.Join(root, name, "out")
		lines, next, err := Tail(out, cursor.Offsets[name])
		if err != nil {
			return nil, err
		}
		for _, line := range lines {
			_, nick, _, ok := ParseLine(line)
			if ok && nick == st.Nick {
				continue
			}
			result = append(result, renderLine(line, name))
		}
		cursor.Offsets[name] = next
	}
	cursor.InvokerKey = st.SessionID
	return result, SaveCursors(cursorPath, cursor)
}

func renderLine(line, source string) string {
	ts, nick, text, ok := ParseLine(line)
	if !ok {
		return line
	}
	stamp := time.Unix(ts, 0).Local().Format("2006-01-02 15:04:05")
	if nick == "" {
		return fmt.Sprintf("%s %s%s", stamp, source, text)
	}
	label := source
	if !strings.HasPrefix(source, "#") {
		label = "DM"
	}
	return fmt.Sprintf("%s (%s) <%s> %s", stamp, label, nick, text)
}

func (m *Manager) Watch(done <-chan struct{}, emit func(string)) error {
	st, err := m.Ensure()
	if err != nil {
		return err
	}
	watchOwner, watchOwnerErr := m.findOwner()
	root := filepath.Join(m.Paths.Conversations, st.Host)
	offsets := map[string]int64{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			offsets[entry.Name()] = fileSize(filepath.Join(root, entry.Name(), "out"))
		}
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	check := time.NewTicker(30 * time.Second)
	defer check.Stop()
	for {
		select {
		case <-done:
			return nil
		case <-check.C:
			if _, err := m.Ensure(); err != nil {
				return err
			}
		case <-ticker.C:
		}
		if watchOwnerErr == nil && !ProcessAlive(watchOwner) {
			return nil
		}
		entries, err = os.ReadDir(root)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			lines, next, _ := Tail(filepath.Join(root, name, "out"), offsets[name])
			for _, line := range lines {
				_, nick, _, ok := ParseLine(line)
				if ok && nick == st.Nick {
					continue
				}
				emit(renderLine(line, name))
			}
			offsets[name] = next
		}
	}
}

func (m *Manager) Who(channel string) ([]string, error) {
	st, err := m.Ensure()
	if err != nil {
		return nil, err
	}
	target := st.HomeChannel.Name
	if channel != "" {
		target = NormalizeChannel(channel)
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
	server := filepath.Join(m.Paths.Conversations, st.Host)
	out := filepath.Join(server, "out")
	offset := fileSize(out)
	if err := WriteFIFO(filepath.Join(server, "in"), "/NAMES "+target, 1); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		lines, _, _ := Tail(out, offset)
		for _, line := range lines {
			name, names, ok := ParseNames(line)
			if !ok || name != target {
				continue
			}
			result := make([]string, 0, len(names))
			for _, n := range names {
				n = strings.TrimLeft(n, "@+")
				if n == st.Nick {
					n += " (you)"
				}
				result = append(result, n)
			}
			for _, member := range names {
				if strings.TrimLeft(member, "@+") == st.Nick {
					return result, nil
				}
			}
			return nil, fmt.Errorf("server did not confirm membership in %s", target)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("no NAMES reply — run: chatta chat health")
}
