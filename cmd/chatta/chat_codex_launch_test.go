//go:build darwin || linux

package main

import (
	"bytes"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCodexLaunchRouting(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		interactive bool
	}{
		{"version", []string{"--version"}, false}, {"help", []string{"--help"}, false},
		{"exec", []string{"exec", "--json", "hello"}, false},
		{"options then exec", []string{"-m", "gpt-6-luna", "-c", "x=exec", "exec", "hello"}, false},
		{"value named help", []string{"-c", "--help", "hello"}, true},
		{"resume", []string{"resume", "thread"}, true}, {"resume help", []string{"resume", "--help"}, false},
		{"prompt named command after separator", []string{"--", "exec"}, true},
		{"quoted prompt", []string{"a; $(false)\nline"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.interactive, codexLaunchInteractive(tc.args)) })
	}
}

func TestCodexLaunchRemoteRespectsArgumentBoundaries(t *testing.T) {
	endpoint, exists := codexLaunchRemote([]string{"-c", "--remote", "hello"})
	require.False(t, exists)
	require.Empty(t, endpoint)
	endpoint, exists = codexLaunchRemote([]string{"resume", "thread", "--remote", "unix:///tmp/a.sock"})
	require.True(t, exists)
	require.Equal(t, "unix:///tmp/a.sock", endpoint)
	endpoint, exists = codexLaunchRemote([]string{"--", "--remote", "prompt"})
	require.False(t, exists)
	require.Empty(t, endpoint)
}

func TestCodexLaunchWrapperHelpDoesNotRequireCodex(t *testing.T) {
	t.Setenv("CHATTA_CODEX_COMMAND", "/nonexistent/codex")
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "launch", RunE: launchCodex}
	cmd.SetOut(&out)
	require.NoError(t, launchCodex(cmd, []string{"--help"}))
	require.Contains(t, out.String(), "Usage:")
}
