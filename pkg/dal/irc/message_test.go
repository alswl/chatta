//go:build darwin || linux

package irc

import (
	"strings"
	"testing"
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
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Prefix != tc.want.Prefix || got.Command != tc.want.Command || got.Trailing != tc.want.Trailing || len(got.Params) != len(tc.want.Params) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			for i := range got.Params {
				if got.Params[i] != tc.want.Params[i] {
					t.Fatalf("param %d: got %q, want %q", i, got.Params[i], tc.want.Params[i])
				}
			}
		})
	}
}

func TestFormatLineRoundTrips(t *testing.T) {
	line := FormatLine("PRIVMSG", "#agents", "hello world")
	if line != "PRIVMSG #agents :hello world" {
		t.Fatalf("unexpected line: %q", line)
	}
	msg, ok := ParseLine(line)
	if !ok || msg.Command != "PRIVMSG" || msg.Params[0] != "#agents" || msg.Params[1] != "hello world" {
		t.Fatalf("round trip failed: %+v (%v)", msg, ok)
	}
}

func TestNick(t *testing.T) {
	if got := Nick("agent-a!u@h"); got != "agent-a" {
		t.Fatalf("got %q", got)
	}
	if got := Nick("server.example"); got != "server.example" {
		t.Fatalf("got %q", got)
	}
}
