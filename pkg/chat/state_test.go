package chat

import (
	"os"
	"path/filepath"
	"testing"
)

func testSession() ChatSession {
	return ChatSession{Nick: "agent-a", Host: "127.0.0.1", Port: 6667, HomeChannel: ChannelMembership{Name: "#lobby"}, Channels: []ChannelMembership{{Name: "#lobby"}}, Owner: OwnerBinding{PID: 1, StartFingerprint: "test"}}
}

func TestIncompleteStateRejected(t *testing.T) {
	s := testSession()
	s.Owner = OwnerBinding{}
	if err := ValidateSession(s); err == nil {
		t.Fatal("state without owner binding accepted")
	}
}
func TestStateAtomicRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	if e := SaveState(p, testSession()); e != nil {
		t.Fatal(e)
	}
	s, e := LoadState(p)
	if e != nil || s.Nick != "agent-a" || s.SchemaVersion != StateSchemaVersion {
		t.Fatalf("%+v %v", s, e)
	}
}
func TestMalformedStateRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	if e := os.WriteFile(p, []byte("{"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := LoadState(p); e == nil {
		t.Fatal("malformed state accepted")
	}
}
func TestCursorPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cursors.json")
	c := MessageCursor{InvokerKey: "session-a", Offsets: map[string]int64{"#lobby": 42}}
	if e := SaveCursors(p, c); e != nil {
		t.Fatal(e)
	}
	got, e := LoadCursors(p)
	if e != nil || got.Offsets["#lobby"] != 42 {
		t.Fatalf("%+v %v", got, e)
	}
}
