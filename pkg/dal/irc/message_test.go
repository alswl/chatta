//go:build darwin || linux

package irc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLine(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		want   Message
		wantOK bool
	}{
		{
			name:   "trailing param with embedded colons",
			line:   ":agent-a!u@h PRIVMSG #agents :hello: world :again",
			want:   Message{Prefix: "agent-a!u@h", Command: "PRIVMSG", Params: []string{"#agents", "hello: world :again"}, Trailing: true},
			wantOK: true,
		},
		{
			name:   "params with no trailing",
			line:   "JOIN #agents",
			want:   Message{Command: "JOIN", Params: []string{"#agents"}},
			wantOK: true,
		},
		{
			name:   "no params",
			line:   "PING",
			want:   Message{Command: "PING"},
			wantOK: true,
		},
		{
			name:   "512-byte line",
			line:   ":server.example 001 agent-a :" + strings.Repeat("x", 470),
			want:   Message{Prefix: "server.example", Command: "001", Params: []string{"agent-a", strings.Repeat("x", 470)}, Trailing: true},
			wantOK: true,
		},
		{
			name:   "non-ascii message body",
			line:   ":agent-a!u@h PRIVMSG #agents :你好，世界",
			want:   Message{Prefix: "agent-a!u@h", Command: "PRIVMSG", Params: []string{"#agents", "你好，世界"}, Trailing: true},
			wantOK: true,
		},
		{
			name:   "malformed: empty",
			line:   "",
			wantOK: false,
		},
		{
			name:   "malformed: prefix with no command",
			line:   ":agent-a!u@h",
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseLine(tc.line)
			require.Equal(t, tc.wantOK, ok)
			if !ok {
				return
			}
			require.Equal(t, tc.want.Prefix, got.Prefix)
			require.Equal(t, tc.want.Command, got.Command)
			require.Equal(t, tc.want.Trailing, got.Trailing)
			// Len before elementwise: the fixtures leave Params unset where
			// a line carries none, and a nil slice and an empty one are the
			// same thing to every caller here.
			require.Len(t, got.Params, len(tc.want.Params))
			for i := range got.Params {
				require.Equal(t, tc.want.Params[i], got.Params[i], "param %d", i)
			}
		})
	}
}

func TestFormatLineRoundTrips(t *testing.T) {
	line := FormatLine("PRIVMSG", "#agents", "hello world")
	require.Equal(t, "PRIVMSG #agents :hello world", line)
	msg, ok := ParseLine(line)
	require.True(t, ok, "round trip failed")
	require.Equal(t, "PRIVMSG", msg.Command)
	require.Equal(t, []string{"#agents", "hello world"}, msg.Params)
}

func TestNick(t *testing.T) {
	require.Equal(t, "agent-a", Nick("agent-a!u@h"))
	require.Equal(t, "server.example", Nick("server.example"))
}
