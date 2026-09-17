package dal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alswl/chatta/pkg/common"

	"github.com/stretchr/testify/require"
)

func TestAppendThenTailOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	msgs := []common.StoredMessage{
		{TS: 1, Kind: "channel", Target: "#agents", Nick: "agent-a", Text: "one"},
		{TS: 2, Kind: "direct", Target: "agent-b", Nick: "agent-a", Text: "two"},
		{TS: 3, Kind: "channel", Target: "#agents", Nick: "agent-b", Text: "three"},
	}
	for _, m := range msgs {
		require.NoError(t, AppendMessage(path, m))
	}
	got, next, err := ReadMessages(path, 0)
	require.NoError(t, err)
	require.Equal(t, msgs, got)
	require.Positive(t, next, "unexpected next offset")
}

func TestReadMessagesOffsetContinuityAcrossTwoReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	require.NoError(t, AppendMessage(path, common.StoredMessage{TS: 1, Kind: "channel", Target: "#agents", Nick: "a", Text: "one"}))
	first, offset, err := ReadMessages(path, 0)
	require.NoError(t, err, "first read")
	require.Len(t, first, 1, "first read")
	require.NoError(t, AppendMessage(path, common.StoredMessage{TS: 2, Kind: "channel", Target: "#agents", Nick: "a", Text: "two"}))
	second, _, err := ReadMessages(path, offset)
	require.NoError(t, err, "second read")
	require.Len(t, second, 1, "second read")
	require.Equal(t, "two", second[0].Text)
}

func TestReadMessagesSkipsCorruptLineWithoutAborting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	content := `{"ts":1,"kind":"channel","target":"#agents","nick":"a","text":"good-one"}
not json at all
{"ts":2,"kind":"channel","target":"#agents","nick":"a","text":"good-two"}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	got, _, err := ReadMessages(path, 0)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "good-one", got[0].Text)
	require.Equal(t, "good-two", got[1].Text)
}

func TestNonASCIITextPreservedByteForByte(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	want := "你好，世界 — こんにちは"
	require.NoError(t, AppendMessage(path, common.StoredMessage{TS: 1, Kind: "channel", Target: "#agents", Nick: "a", Text: want}))
	got, _, err := ReadMessages(path, 0)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, want, got[0].Text)
}
