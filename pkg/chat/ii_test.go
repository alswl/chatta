//go:build darwin || linux

package chat

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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

func TestParseRawIIRCReplies(t *testing.T) {
	channel, names, ok := ParseNames("1700000000 :smoke.local 353 misky = #agents :@misky pola")
	if !ok || channel != "#agents" || len(names) != 2 {
		t.Fatalf("unexpected raw NAMES response: %q %#v %v", channel, names, ok)
	}
	_, nick, text, ok := ParseLine("1700000000 :pola!~pola@127.0.0.1 PRIVMSG #agents :hello")
	if !ok || nick != "pola" || text != "hello" {
		t.Fatalf("unexpected raw PRIVMSG: %q %q %v", nick, text, ok)
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

func TestFIFOReaderObservesOpenFIFOWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in")
	if err := unix.Mkfifo(path, 0600); err != nil {
		t.Fatal(err)
	}
	if FIFOReader(path) {
		t.Fatal("FIFO without an open reader is unexpectedly healthy")
	}
	reader := exec.Command("sh", "-c", "exec 3<> \"$1\"; sleep 10", "sh", path)
	if err := reader.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Process.Kill(); _ = reader.Wait() })
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if FIFOReader(path) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("open FIFO was not observed")
}

func TestLsofHasPathRequiresAnExactNameRecord(t *testing.T) {
	path := "/private/tmp/client/irc/server/#agents/in"
	output := []byte("p123\nfcwd\nn/private/tmp/client\nfn\nn" + path + "\n")
	if !lsofHasPath(output, path) {
		t.Fatal("expected matching lsof name record")
	}
	if lsofHasPath(output, path+"-backup") {
		t.Fatal("prefix match must not make a different FIFO healthy")
	}
}
