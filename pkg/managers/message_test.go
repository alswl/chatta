//go:build darwin || linux

package managers

import (
	"os"
	"strings"
	"testing"
)

func TestSplitUTF8PreservesTextAndLimit(t *testing.T) {
	input := strings.Repeat("你", 200) + " end"
	parts := splitUTF8(input, 400)
	var got strings.Builder
	for _, part := range parts {
		if len([]byte(part)) > 400 {
			t.Fatalf("chunk too large: %d", len([]byte(part)))
		}
		got.WriteString(part)
	}
	if got.String() != input {
		t.Fatal("UTF-8 splitter changed message content")
	}
}

func TestWaitForAbsentNickDetectsServerReply(t *testing.T) {
	path := t.TempDir() + "/out"
	if err := os.WriteFile(path, []byte("1700000000 <server> pola No such nick or channel name\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := waitForAbsentNick(path, 0, "pola"); err == nil {
		t.Fatal("absent nick reply was ignored")
	}
}

func TestRenderLineLabelsDirectMessages(t *testing.T) {
	line := renderLine("1700000000 <pola> [ASK] hello", "pola")
	if !strings.Contains(line, "(DM)") || !strings.Contains(line, "<pola>") {
		t.Fatalf("unexpected rendering: %s", line)
	}
}

func TestSplitUTF8RespectsByteLimit(t *testing.T) {
	parts := splitUTF8("你好世界", 7)
	if len(parts) != 2 || len([]byte(parts[0])) > 7 || len([]byte(parts[1])) > 7 {
		t.Fatalf("unexpected chunks: %#v", parts)
	}
}
