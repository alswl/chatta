//go:build darwin || linux

package chat

import (
	"os"
	"strconv"
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

func TestFindAgentOwnerUsesValidatedExplicitTestOwner(t *testing.T) {
	t.Setenv(testOwnerPIDEnv, strconv.Itoa(os.Getpid()))
	owner, err := FindAgentOwner()
	if err != nil {
		t.Fatal(err)
	}
	if owner.PID != os.Getpid() || owner.Runtime != "verification-fixture" || !ProcessAlive(owner) {
		t.Fatalf("unexpected fixture owner: %+v", owner)
	}
}

func TestFindAgentOwnerRejectsInvalidExplicitTestOwner(t *testing.T) {
	t.Setenv(testOwnerPIDEnv, "not-a-pid")
	if _, err := FindAgentOwner(); err == nil {
		t.Fatal("invalid fixture owner unexpectedly succeeded")
	}
}
