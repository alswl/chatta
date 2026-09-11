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

func TestIIFlagIsAdvancedChattaTransportConfiguration(t *testing.T) {
	flag := chatCmd.PersistentFlags().Lookup("ii")
	if flag == nil || !strings.Contains(flag.Usage, "Chatta-managed") {
		t.Fatalf("ii flag does not describe Chatta ownership: %#v", flag)
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
