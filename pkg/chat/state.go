package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func ValidateSession(s ChatSession) error {
	if s.SchemaVersion == 0 {
		s.SchemaVersion = StateSchemaVersion
	}
	if s.SchemaVersion != StateSchemaVersion {
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
func LoadState(path string) (ChatSession, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return ChatSession{}, e
	}
	var s ChatSession
	if e = json.Unmarshal(b, &s); e != nil {
		return s, fmt.Errorf("malformed state: %w", e)
	}
	if e = ValidateSession(s); e != nil {
		return s, e
	}
	return s, nil
}
func SaveState(path string, s ChatSession) error {
	s.SchemaVersion = StateSchemaVersion
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
	defer os.Remove(tmp)
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
func LoadCursors(path string) (MessageCursor, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return MessageCursor{Offsets: map[string]int64{}}, e
	}
	var c MessageCursor
	if e = json.Unmarshal(b, &c); e != nil {
		return MessageCursor{Offsets: map[string]int64{}}, fmt.Errorf("malformed cursors: %w", e)
	}
	if c.Offsets == nil {
		c.Offsets = map[string]int64{}
	}
	return c, nil
}
func SaveCursors(path string, c MessageCursor) error {
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
	defer os.Remove(tmp)
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
