package dal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

type MessageRecord struct {
	Message common.StoredMessage
	Offset  int64
}

type InvalidMessageRecordError struct {
	Offset int64
	Err    error
}

func (e *InvalidMessageRecordError) Error() string {
	return fmt.Sprintf("invalid message JSONL record at byte %d: %v", e.Offset, e.Err)
}

func (e *InvalidMessageRecordError) Unwrap() error { return e.Err }

func ReadCompleteMessageRecords(path string, offset int64) ([]MessageRecord, int64, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, offset, nil
	}
	if err != nil {
		return nil, offset, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, offset, err
	}
	if info.Size() < offset {
		offset = 0
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, offset, err
	}
	end := len(data)
	if end > 0 && data[end-1] != '\n' {
		lastNL := bytes.LastIndexByte(data, '\n')
		if lastNL < 0 {
			return nil, offset, nil
		}
		end = lastNL + 1
	}
	var records []MessageRecord
	pos := 0
	for pos < end {
		rel := bytes.IndexByte(data[pos:end], '\n')
		if rel < 0 {
			break
		}
		lineEnd := pos + rel + 1
		var msg common.StoredMessage
		if err := json.Unmarshal(data[pos:lineEnd-1], &msg); err != nil {
			badOffset := offset + int64(pos)
			return records, badOffset, &InvalidMessageRecordError{Offset: badOffset, Err: err}
		}
		records = append(records, MessageRecord{Message: msg, Offset: offset + int64(lineEnd)})
		pos = lineEnd
	}
	return records, offset + int64(end), nil
}
