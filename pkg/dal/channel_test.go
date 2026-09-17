package dal

import (
	"testing"

	"github.com/alswl/chatta/pkg/common"

	"github.com/stretchr/testify/require"
)

func TestNormalizeChannel(t *testing.T) {
	require.Equal(t, "#project-alpha", NormalizeChannel(" Project/Alpha "))
	require.Equal(t, "#lobby", NormalizeChannel("#LOBBY"))
}
func TestChannelOrderingAndHomeProtection(t *testing.T) {
	cs := []common.ChannelMembership{{Name: "#lobby"}, {Name: "#work"}}
	require.NoError(t, ValidateChannels(cs, "#lobby"))
	require.Error(t, CanPart("#lobby", "#lobby"), "home must be protected")
	require.Error(t,
		ValidateChannels([]common.ChannelMembership{{Name: "#work"}, {Name: "#lobby"}}, "#lobby"),
		"home must be first")
}
