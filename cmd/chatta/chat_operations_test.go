package main

import "testing"

func TestChatOperationsCommandsExist(t *testing.T) {
	for _, name := range []string{"watch", "clients", "gc"} {
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
}
