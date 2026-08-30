//go:build darwin || linux

package chat

import (
	"os"
	"testing"
)

func TestProcessStartAndAlive(t *testing.T) {
	fingerprint := ProcessStart(os.Getpid())
	if fingerprint == "" {
		t.Fatal("ProcessStart returned an empty fingerprint")
	}
	if !ProcessAlive(OwnerBinding{PID: os.Getpid(), StartFingerprint: fingerprint}) {
		t.Fatal("current process should be alive")
	}
	if ProcessAlive(OwnerBinding{PID: os.Getpid(), StartFingerprint: "different"}) {
		t.Fatal("mismatched fingerprint must not be alive")
	}
}

func TestInvocationSessionIDPrefersCallingRuntime(t *testing.T) {
	t.Setenv("CODEX_SESSION_ID", "reader-session")
	if got := InvocationSessionID("owner-session"); got != "reader-session" {
		t.Fatalf("got %q", got)
	}
}
