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
