//go:build darwin || linux

package services

import (
	"context"
	"testing"

	"github.com/alswl/chatta/pkg/config"
	"github.com/alswl/chatta/pkg/dal"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func TestSurveyEmptyHome(t *testing.T) {
	m := NewChatService(config.ChatConfig{Home: t.TempDir()})
	rows, err := m.Survey(context.Background())
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestSurveyFindsSiblingClientHomes(t *testing.T) {
	root := t.TempDir()
	current := root + "/current"
	other := root + "/other"
	for _, home := range []string{current, other} {
		state := testSession()
		state.Nick = home[len(root)+1:]
		require.NoError(t, dal.SaveState(dal.ResolvePaths(home).State, state))
	}
	m := NewChatService(config.ChatConfig{Home: current})
	rows, err := m.Survey(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestWithinRootRejectsSiblingPrefix(t *testing.T) {
	root := t.TempDir() + "/clients"
	cases := []struct {
		path string
		want bool
	}{
		{root, true},
		{root + "/agent-one", true},
		{root + "-backup/agent-one", false},
		{t.TempDir(), false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, withinRoot(tc.path, root), "withinRoot(%q, %q)", tc.path, root)
	}
}
