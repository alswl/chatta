//go:build integration && (darwin || linux)

package chat_test

import (
	"os/exec"
	"testing"
)

// The real-daemon scenarios are added with the chat lifecycle implementation.
// Keeping the integration tag on this package lets normal unit tests run
// without ii or ngircd installed.
func TestIntegrationPrerequisites(t *testing.T) {
	if _, err := exec.LookPath("ii"); err != nil {
		t.Skip("integration scenarios require ii")
	}
	if _, err := exec.LookPath("ngircd"); err != nil {
		t.Skip("integration scenarios require ngircd")
	}
	t.Log("ii and ngircd are available; run the lifecycle smoke scenarios in an isolated environment")
}
