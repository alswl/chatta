package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatOperationsCommandsExist(t *testing.T) {
	for _, name := range []string{"watch", "clients", "gc"} {
		found := false
		for _, child := range chatCmd.Commands() {
			if child.Name() == name {
				found = true
			}
		}
		require.True(t, found, "missing chat %s command", name)
	}
}
