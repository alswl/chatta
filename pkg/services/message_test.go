//go:build darwin || linux

package services

import (
	"strings"
	"testing"

	"github.com/alswl/chatta/pkg/common"
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

func TestRenderMessageLabelsDirectMessages(t *testing.T) {
	line := renderMessage(common.StoredMessage{TS: 1700000000, Kind: "direct", Target: "pola", Nick: "pola", Text: "[ASK] hello"})
	if !strings.Contains(line, "(DM)") || !strings.Contains(line, "<pola>") {
		t.Fatalf("unexpected rendering: %s", line)
	}
}

func TestRenderMessageLabelsChannelMessages(t *testing.T) {
	line := renderMessage(common.StoredMessage{TS: 1700000000, Kind: "channel", Target: "#agents", Nick: "pola", Text: "hi"})
	if !strings.Contains(line, "(#agents)") || !strings.Contains(line, "<pola>") {
		t.Fatalf("unexpected rendering: %s", line)
	}
}

func TestSplitUTF8RespectsByteLimit(t *testing.T) {
	parts := splitUTF8("你好世界", 7)
	if len(parts) != 2 || len([]byte(parts[0])) > 7 || len([]byte(parts[1])) > 7 {
		t.Fatalf("unexpected chunks: %#v", parts)
	}
}
