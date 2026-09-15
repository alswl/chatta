package dal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alswl/chatta/pkg/common"
)

func TestAppendThenTailOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	msgs := []common.StoredMessage{
		{TS: 1, Kind: "channel", Target: "#agents", Nick: "agent-a", Text: "one"},
		{TS: 2, Kind: "direct", Target: "agent-b", Nick: "agent-a", Text: "two"},
		{TS: 3, Kind: "channel", Target: "#agents", Nick: "agent-b", Text: "three"},
	}
	for _, m := range msgs {
		if err := AppendMessage(path, m); err != nil {
			t.Fatal(err)
		}
	}
	got, next, err := ReadMessages(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}
	for i, m := range msgs {
		if got[i] != m {
			t.Fatalf("message %d: got %+v, want %+v", i, got[i], m)
		}
	}
	if next <= 0 {
		t.Fatalf("unexpected next offset: %d", next)
	}
}

func TestReadMessagesOffsetContinuityAcrossTwoReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	if err := AppendMessage(path, common.StoredMessage{TS: 1, Kind: "channel", Target: "#agents", Nick: "a", Text: "one"}); err != nil {
		t.Fatal(err)
	}
	first, offset, err := ReadMessages(path, 0)
	if err != nil || len(first) != 1 {
		t.Fatalf("first read: %+v %v", first, err)
	}
	if err := AppendMessage(path, common.StoredMessage{TS: 2, Kind: "channel", Target: "#agents", Nick: "a", Text: "two"}); err != nil {
		t.Fatal(err)
	}
	second, _, err := ReadMessages(path, offset)
	if err != nil || len(second) != 1 || second[0].Text != "two" {
		t.Fatalf("second read: %+v %v", second, err)
	}
}

func TestReadMessagesSkipsCorruptLineWithoutAborting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	content := `{"ts":1,"kind":"channel","target":"#agents","nick":"a","text":"good-one"}
not json at all
{"ts":2,"kind":"channel","target":"#agents","nick":"a","text":"good-two"}
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got, _, err := ReadMessages(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "good-one" || got[1].Text != "good-two" {
		t.Fatalf("unexpected messages: %+v", got)
	}
}

func TestNonASCIITextPreservedByteForByte(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	want := "你好，世界 — こんにちは"
	if err := AppendMessage(path, common.StoredMessage{TS: 1, Kind: "channel", Target: "#agents", Nick: "a", Text: want}); err != nil {
		t.Fatal(err)
	}
	got, _, err := ReadMessages(path, 0)
	if err != nil || len(got) != 1 || got[0].Text != want {
		t.Fatalf("got %+v, want text %q (%v)", got, want, err)
	}
}
