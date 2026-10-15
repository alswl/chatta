package dal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alswl/chatta/pkg/common"
	"github.com/stretchr/testify/require"
)

func TestCodexBindingPersistsIndependentCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "codex-binding.json")
	want := common.CodexBinding{BindingID: "binding-1", SessionID: "session-a", Endpoint: "unix:///tmp/app.sock", ThreadID: "thread-a", Enabled: true, Offset: 37}
	require.NoError(t, SaveCodexBinding(path, want))
	got, err := LoadCodexBinding(path)
	require.NoError(t, err)
	require.Equal(t, want, got)
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestReadCompleteMessageRecordsStopsAtInvalidCompleteRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	require.NoError(t, AppendMessage(path, common.StoredMessage{EventID: "one", Source: "irc", Text: "hello"}))
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	badOffset := int64(len(contents))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	require.NoError(t, err)
	_, err = f.Write([]byte("{invalid-json}\n{" + `"event_id":"later"}` + "\n"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	records, next, err := ReadCompleteMessageRecords(path, 0)
	var invalid *InvalidMessageRecordError
	require.ErrorAs(t, err, &invalid)
	require.Equal(t, badOffset, next)
	require.Equal(t, badOffset, invalid.Offset)
	require.Len(t, records, 1)
	require.Equal(t, "one", records[0].Message.EventID)

	// Retrying from the returned cursor must encounter the same bad record.
	records, next, err = ReadCompleteMessageRecords(path, next)
	require.ErrorAs(t, err, &invalid)
	require.Equal(t, badOffset, next)
	require.Empty(t, records)
}

func TestReadCompleteMessageRecordsLeavesPartialLinePending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	require.NoError(t, AppendMessage(path, common.StoredMessage{EventID: "one", Source: "irc", Text: "hello"}))
	complete, err := os.ReadFile(path)
	require.NoError(t, err)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	require.NoError(t, err)
	_, err = f.Write([]byte(`{"event_id":"two"`))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	records, offset, err := ReadCompleteMessageRecords(path, 0)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "one", records[0].Message.EventID)
	require.Equal(t, int64(len(complete)), offset)
}
