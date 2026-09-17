package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatMessagingCommandsExist(t *testing.T) {
	for _, name := range []string{"join", "part", "send", "dm", "poll", "who"} {
		found := false
		for _, child := range chatCmd.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		require.True(t, found, "missing chat %s command", name)
	}
}
