package main

import "testing"

func TestChatMessagingCommandsExist(t *testing.T) {
	for _, name := range []string{"join", "part", "send", "dm", "poll", "who"} {
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
