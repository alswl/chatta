package dal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alswl/chatta/pkg/common"
)

func ValidateSession(s common.ChatSession) error {
	if s.SchemaVersion == 0 {
		s.SchemaVersion = common.StateSchemaVersion
	}
	if s.SchemaVersion != common.StateSchemaVersion {
		return fmt.Errorf("unsupported state schema version %d", s.SchemaVersion)
	}
	if s.Nick == "" || s.Host == "" || s.Port < 1 || s.Port > 65535 || s.Owner.PID <= 0 || s.Owner.StartFingerprint == "" || s.HomeChannel.Name == "" || len(s.Channels) == 0 {
		return errors.New("invalid chat session")
	}
	if s.SupervisorPID > 0 && s.SupervisorStartFingerprint == "" {
		return errors.New("invalid supervisor identity")
	}
	return ValidateChannels(s.Channels, s.HomeChannel.Name)
}
func LoadState(path string) (common.ChatSession, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return common.ChatSession{}, e
	}
	var s common.ChatSession
	if e = json.Unmarshal(b, &s); e != nil {
		return s, fmt.Errorf("malformed state: %w", e)
	}
	if e = ValidateSession(s); e != nil {
		return s, e
	}
	return s, nil
}
func SaveState(path string, s common.ChatSession) error {
	s.SchemaVersion = common.StateSchemaVersion
	if e := ValidateSession(s); e != nil {
		return e
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	b, e := json.MarshalIndent(s, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".state-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, e = f.Write(append(b, '\n')); e == nil {
		e = f.Chmod(0600)
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(tmp, path)
	}
	return e
}
func LoadCursors(path string) (common.MessageCursor, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return common.MessageCursor{Offsets: map[string]int64{}}, e
	}
	var c common.MessageCursor
	if e = json.Unmarshal(b, &c); e != nil {
		return common.MessageCursor{Offsets: map[string]int64{}}, fmt.Errorf("malformed cursors: %w", e)
	}
	if c.Offsets == nil {
		c.Offsets = map[string]int64{}
	}
	return c, nil
}
func SaveCursors(path string, c common.MessageCursor) error {
	if c.Offsets == nil {
		c.Offsets = map[string]int64{}
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".cursor-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, e = f.Write(append(b, '\n')); e == nil {
		e = f.Chmod(0600)
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(tmp, path)
	}
	return e
}
