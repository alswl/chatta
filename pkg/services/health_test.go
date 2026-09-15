//go:build darwin || linux

package services

import (
	"testing"

	"github.com/alswl/chatta/pkg/config"
)

func TestHealthWithoutSessionNamesFailure(t *testing.T) {
	m := NewChatService(config.ChatConfig{Home: t.TempDir()})
	r := m.Health(true)
	if r.Failure == "" || r.Owner || r.Supervisor {
		t.Fatalf("unexpected health report: %+v", r)
	}
}

func TestTimeReplyAcceptsFormattedAndRawIIRCLines(t *testing.T) {
	for _, line := range []string{
		"1700000000 agentchat.local Thursday August 27 2026",
		"1700000000 :agentchat.local 391 misky agentchat.local :Thursday August 27 2026",
	} {
		if !timeReply.MatchString(line) {
			t.Errorf("TIME reply was not recognized: %q", line)
		}
	}
}
