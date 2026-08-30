package main

import "testing"

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
