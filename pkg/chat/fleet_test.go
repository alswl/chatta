//go:build darwin || linux

package chat

import (
	"testing"

	"github.com/alswl/chatta/pkg/config"
)

func TestSurveyEmptyHome(t *testing.T) {
	m := NewManager(config.ChatConfig{Home: t.TempDir()})
	rows, err := m.Survey()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("unexpected client rows: %+v", rows)
	}
}

func TestSurveyFindsSiblingClientHomes(t *testing.T) {
	root := t.TempDir()
	current := root + "/current"
	other := root + "/other"
	for _, home := range []string{current, other} {
		state := testSession()
		state.Nick = home[len(root)+1:]
		if err := SaveState(ResolvePaths(home).State, state); err != nil {
			t.Fatal(err)
		}
	}
	m := NewManager(config.ChatConfig{Home: current})
	rows, err := m.Survey()
	if err != nil || len(rows) != 2 {
		t.Fatalf("expected two homes, got %+v (%v)", rows, err)
	}
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
		if got := withinRoot(tc.path, root); got != tc.want {
			t.Errorf("withinRoot(%q, %q) = %t, want %t", tc.path, root, got, tc.want)
		}
	}
}
