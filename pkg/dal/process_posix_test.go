//go:build darwin || linux

package dal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockHomeExclusive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "supervisor.lock")
	first, err := LockHome(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := LockHome(path)
	if err == nil {
		second.Close()
		t.Fatal("second lock unexpectedly succeeded")
	}
}

func TestIsSupervisorRejectsUnrelatedProcess(t *testing.T) {
	if IsSupervisor(os.Getpid()) {
		t.Fatal("test process must not be treated as supervisor")
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
