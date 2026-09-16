package main

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/stretchr/testify/require"
)

func TestChatCommandExposesLifecycleCommands(t *testing.T) {
	for _, name := range []string{"start", "health", "stop"} {
		require.NotEmpty(t, chatCmd.Commands(), "chat command has no children")
		found := false
		for _, child := range chatCmd.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		require.True(t, found, "missing chat %s command", name)
	}
	require.True(t, supervisorCmd.Hidden, "supervisor command must be hidden")
}

func TestIIFlagIsDeprecated(t *testing.T) {
	flag := chatCmd.PersistentFlags().Lookup("ii")
	require.NotNil(t, flag)
	require.Contains(t, flag.Usage, "deprecated", "ii flag is not marked deprecated in its help text")
	// pflag's own deprecation warning would be a second stderr line on top
	// of the one config emits; the contract allows exactly one (FR-010).
	require.Empty(t, flag.Deprecated, "ii flag must not carry pflag's deprecation notice")
}

func TestChatCommandExposesDomainCommandGroups(t *testing.T) {
	groups := map[string][]string{
		"session": {"start", "status", "stop"},
		"channel": {"join", "leave", "members"},
		"message": {"send", "direct"},
		"inbox":   {"read", "watch"},
		"client":  {"list", "gc"},
	}
	for name, commands := range groups {
		var groupFound *cobra.Command
		found := false
		for _, child := range chatCmd.Commands() {
			if child.Name() == name {
				found = true
				groupFound = child
			}
		}
		require.True(t, found, "missing chat %s command group", name)
		for _, command := range commands {
			_, _, err := groupFound.Find([]string{command})
			require.NoError(t, err, "missing chat %s %s command", name, command)
		}
	}
	for _, alias := range []string{"start", "health", "clients"} {
		cmd, _, err := chatCmd.Find([]string{alias})
		require.NoError(t, err, "missing flat compatibility command %s", alias)
		require.True(t, cmd.Hidden,
			"flat compatibility command %s must stay hidden from the primary command tree", alias)
	}
}

// contracts/cli-commands.md freezes `session status [--deep]`; the flag was
// accepted by the service layer but never wired to the command.
func TestSessionStatusAcceptsDeepFlag(t *testing.T) {
	for _, path := range [][]string{{"session", "status"}, {"health"}} {
		cmd, _, err := chatCmd.Find(path)
		require.NoError(t, err, "%v", path)
		require.NotNil(t, cmd.Flags().Lookup("deep"), "%v does not expose --deep", path)
	}
}
