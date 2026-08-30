//go:build darwin || linux

package chat

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestParseLineAndNames(t *testing.T) {
	ts, nick, text, ok := ParseLine("1700000000 <pola> [ASK] hello")
	if !ok || ts != 1700000000 || nick != "pola" || text != "[ASK] hello" {
		t.Fatalf("unexpected parsed message: %d %q %q %v", ts, nick, text, ok)
	}
	channel, names, ok := ParseNames("1700000000 = #agents @misky pola")
	if !ok || channel != "#agents" || len(names) != 2 {
		t.Fatalf("unexpected NAMES response: %q %#v %v", channel, names, ok)
	}
}

func TestWriteFIFOFailsWithoutReader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFIFO(path, "hello", 0); err == nil {
		t.Fatal("write to FIFO without reader unexpectedly succeeded")
	}
}
