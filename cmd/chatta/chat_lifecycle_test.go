package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestChatCommandExposesLifecycleCommands(t *testing.T) {
	for _, name := range []string{"start", "health", "stop"} {
		if chatCmd.Commands() == nil {
			t.Fatal("chat command has no children")
		}
		found := false
		for _, child := range chatCmd.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing chat %s command", name)
		}
	}
	if supervisorCmd.Hidden != true {
		t.Fatal("supervisor command must be hidden")
	}
}

func TestIIFlagIsDeprecated(t *testing.T) {
	flag := chatCmd.PersistentFlags().Lookup("ii")
	if flag == nil || !strings.Contains(flag.Usage, "deprecated") {
		t.Fatalf("ii flag is not marked deprecated in its help text: %#v", flag)
	}
	// pflag's own deprecation warning would be a second stderr line on top
	// of the one config emits; the contract allows exactly one (FR-010).
	if flag.Deprecated != "" {
		t.Fatalf("ii flag must not carry pflag's deprecation notice: %q", flag.Deprecated)
	}
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
		if !found {
			t.Fatalf("missing chat %s command group", name)
		}
		for _, command := range commands {
			if _, _, err := groupFound.Find([]string{command}); err != nil {
				t.Fatalf("missing chat %s %s command: %v", name, command, err)
			}
		}
	}
	if !startCmd.Hidden || !healthCmd.Hidden || !clientsCmd.Hidden {
		t.Fatal("flat compatibility commands must stay hidden from the primary command tree")
	}
}

// contracts/cli-commands.md freezes `session status [--deep]`; the flag was
// accepted by the service layer but never wired to the command.
func TestSessionStatusAcceptsDeepFlag(t *testing.T) {
	for _, path := range [][]string{{"session", "status"}, {"health"}} {
		cmd, _, err := chatCmd.Find(path)
		if err != nil {
			t.Fatalf("%v: %v", path, err)
		}
		if cmd.Flags().Lookup("deep") == nil {
			t.Fatalf("%v does not expose --deep", path)
		}
	}
}
