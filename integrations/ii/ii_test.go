//go:build darwin || linux

package ii

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/dal"
	"golang.org/x/sys/unix"
)

func TestStartReportsActionableMissingTransport(t *testing.T) {
	client := &Client{Paths: dal.ResolvePaths(filepath.Join(t.TempDir(), "client"))}
	err := client.Start(common.ChatSession{Host: "127.0.0.1", Port: 6667, Nick: "test"}, filepath.Join(t.TempDir(), "missing-ii"))
	if err == nil || !strings.Contains(err.Error(), "chat transport") || !strings.Contains(err.Error(), "install ii") {
		t.Fatalf("expected actionable missing transport error, got %v", err)
	}
}

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

func TestLsofHasPathRequiresExactNameRecord(t *testing.T) {
	path := "/private/tmp/client/irc/server/#agents/in"
	output := []byte("p123\nfcwd\nn/private/tmp/client\nfn\nn" + path + "\n")
	if !lsofHasPath(output, path) || lsofHasPath(output, path+"-backup") {
		t.Fatal("lsof path matching is not exact")
	}
}

func TestLsofExecutableResolvesOnThisPlatform(t *testing.T) {
	if _, err := os.Stat(lsofExecutable()); err != nil {
		t.Fatalf("lsof executable is not available: %v", err)
	}
}

func TestIsIIForHomeRequiresExactClientHome(t *testing.T) {
	if !isIIForHome("/opt/homebrew/bin/ii -s 127.0.0.1 -i /tmp/a/irc", "/tmp/a/irc") {
		t.Fatal("expected ii client home match")
	}
	if isIIForHome("/opt/homebrew/bin/ii -i /tmp/ab/irc", "/tmp/a/irc") {
		t.Fatal("must not match a different client home")
	}
}
