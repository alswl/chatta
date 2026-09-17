//go:build darwin || linux

package services

import (
	"strings"
	"testing"

	"github.com/alswl/chatta/pkg/common"
	"github.com/stretchr/testify/require"
)

func TestSplitUTF8PreservesTextAndLimit(t *testing.T) {
	input := strings.Repeat("你", 200) + " end"
	parts := splitUTF8(input, 400)
	var got strings.Builder
	for _, part := range parts {
		require.LessOrEqual(t, len([]byte(part)), 400, "chunk too large")
		got.WriteString(part)
	}
	require.Equal(t, input, got.String(), "UTF-8 splitter changed message content")
}

func TestRenderMessageLabelsDirectMessages(t *testing.T) {
	line := renderMessage(common.StoredMessage{TS: 1700000000, Kind: "direct", Target: "pola", Nick: "pola", Text: "[ASK] hello"})
	require.Contains(t, line, "(DM)")
	require.Contains(t, line, "<pola>")
}

func TestRenderMessageLabelsChannelMessages(t *testing.T) {
	line := renderMessage(common.StoredMessage{TS: 1700000000, Kind: "channel", Target: "#agents", Nick: "pola", Text: "hi"})
	require.Contains(t, line, "(#agents)")
	require.Contains(t, line, "<pola>")
}

func TestSplitUTF8RespectsByteLimit(t *testing.T) {
	parts := splitUTF8("你好世界", 7)
	require.Len(t, parts, 2)
	for _, part := range parts {
		require.LessOrEqual(t, len([]byte(part)), 7, "chunk too large")
	}
}
