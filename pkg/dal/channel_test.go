package dal

import (
	"testing"

	"github.com/alswl/chatta/pkg/common"
)

func TestNormalizeChannel(t *testing.T) {
	if got := NormalizeChannel(" Project/Alpha "); got != "#project-alpha" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeChannel("#LOBBY"); got != "#lobby" {
		t.Fatalf("got %q", got)
	}
}
func TestChannelOrderingAndHomeProtection(t *testing.T) {
	cs := []common.ChannelMembership{{Name: "#lobby"}, {Name: "#work"}}
	if e := ValidateChannels(cs, "#lobby"); e != nil {
		t.Fatal(e)
	}
	if e := CanPart("#lobby", "#lobby"); e == nil {
		t.Fatal("home must be protected")
	}
	if e := ValidateChannels([]common.ChannelMembership{{Name: "#work"}, {Name: "#lobby"}}, "#lobby"); e == nil {
		t.Fatal("home must be first")
	}
}
