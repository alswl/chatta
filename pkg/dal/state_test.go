package dal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alswl/chatta/pkg/common"
)

func testSession() common.ChatSession {
	return common.ChatSession{Nick: "agent-a", Host: "127.0.0.1", Port: 6667, HomeChannel: common.ChannelMembership{Name: "#lobby"}, Channels: []common.ChannelMembership{{Name: "#lobby"}}, Owner: common.OwnerBinding{PID: 1, StartFingerprint: "test"}}
}

func TestIncompleteStateRejected(t *testing.T) {
	s := testSession()
	s.Owner = common.OwnerBinding{}
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
	if e != nil || s.Nick != "agent-a" || s.SchemaVersion != common.StateSchemaVersion {
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
func TestPreUpgradeSchemaVersionRejectedNamingUpgrade(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	s := testSession()
	s.SchemaVersion = 1
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadState(p)
	if err == nil {
		t.Fatal("schema_version:1 state was accepted after the v2 upgrade")
	}
	if !strings.Contains(err.Error(), "unsupported state schema version") {
		t.Fatalf("error does not name the schema mismatch: %v", err)
	}
}

func TestCursorPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cursors.json")
	c := common.MessageCursor{InvokerKey: "session-a", Offset: 42}
	if e := SaveCursors(p, c); e != nil {
		t.Fatal(e)
	}
	got, e := LoadCursors(p)
	if e != nil || got.Offset != 42 {
		t.Fatalf("%+v %v", got, e)
	}
}
