package dal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alswl/chatta/pkg/common"

	"github.com/stretchr/testify/require"
)

func testSession() common.ChatSession {
	return common.ChatSession{Nick: "agent-a", Host: "127.0.0.1", Port: 6667, HomeChannel: common.ChannelMembership{Name: "#lobby"}, Channels: []common.ChannelMembership{{Name: "#lobby"}}, Owner: common.OwnerBinding{PID: 1, StartFingerprint: "test"}}
}

func TestIncompleteStateRejected(t *testing.T) {
	s := testSession()
	s.Owner = common.OwnerBinding{}
	require.Error(t, ValidateSession(s), "state without owner binding accepted")
}
func TestStateAtomicRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, SaveState(p, testSession()))
	s, e := LoadState(p)
	require.NoError(t, e)
	require.Equal(t, "agent-a", s.Nick)
	require.Equal(t, common.StateSchemaVersion, s.SchemaVersion)
}
func TestMalformedStateRejected(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(p, []byte("{"), 0600))
	_, e := LoadState(p)
	require.Error(t, e, "malformed state accepted")
}
func TestPreUpgradeSchemaVersionRejectedNamingUpgrade(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	s := testSession()
	s.SchemaVersion = 1
	b, err := json.Marshal(s)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(p, b, 0600))
	_, err = LoadState(p)
	require.Error(t, err, "schema_version:1 state was accepted after the v2 upgrade")
	require.Contains(t, err.Error(), "unsupported state schema version",
		"error does not name the schema mismatch")
}

func TestCursorPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cursors.json")
	c := common.MessageCursor{InvokerKey: "session-a", Offset: 42}
	require.NoError(t, SaveCursors(p, c))
	got, e := LoadCursors(p)
	require.NoError(t, e)
	require.Equal(t, int64(42), got.Offset)
}
