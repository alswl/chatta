package dal

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/alswl/chatta/pkg/common"
)

// AppendMessage appends one StoredMessage to path as a single JSON line,
// written with one write call so a concurrent reader never observes a
// partial line.
func AppendMessage(path string, msg common.StoredMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(append(b, '\n'))
	return err
}

// ReadMessages tails path from offset, returning every message that
// parses. A line that fails to parse is skipped rather than aborting the
// read, so one bad line cannot make an inbox unreadable.
func ReadMessages(path string, offset int64) ([]common.StoredMessage, int64, error) {
	lines, next, err := Tail(path, offset)
	if err != nil {
		return nil, offset, err
	}
	out := make([]common.StoredMessage, 0, len(lines))
	for _, line := range lines {
		var msg common.StoredMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		out = append(out, msg)
	}
	return out, next, nil
}
