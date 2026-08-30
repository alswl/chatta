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
