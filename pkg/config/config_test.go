package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChatConfigurationPrecedence(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(file, []byte("chat:\n  host: from-file\n  port: 7000\n"), 0o600))
	t.Setenv("AGENT_CHAT_HOST", "from-legacy")
	config, err := Load(Options{File: file})
	require.NoError(t, err)
	require.Equal(t, "from-file", config.Chat.Host, "config file must override legacy")
	t.Setenv("CHATTA_CHAT_HOST", "from-primary")
	config, err = Load(Options{File: file})
	require.NoError(t, err)
	require.Equal(t, "from-primary", config.Chat.Host, "primary environment must override config")
	config, err = Load(Options{File: file, Chat: ChatConfig{Host: "from-flag"}, ChatHostSet: true})
	require.NoError(t, err)
	require.Equal(t, "from-flag", config.Chat.Host, "flag must override environment")
}

func TestDeprecatedIISettingsAreAcceptedAndIgnored(t *testing.T) {
	baseline, err := Load(Options{})
	require.NoError(t, err)

	t.Setenv("CHATTA_CHAT_II", "/custom/ii")
	viaPrimaryEnv, err := Load(Options{})
	require.NoError(t, err, "CHATTA_CHAT_II must be accepted without error")
	require.Equal(t, baseline.Chat, viaPrimaryEnv.Chat, "CHATTA_CHAT_II must not affect resolved configuration")
	os.Unsetenv("CHATTA_CHAT_II")

	t.Setenv("AGENT_CHAT_II", "/custom/ii")
	viaLegacyEnv, err := Load(Options{})
	require.NoError(t, err, "AGENT_CHAT_II must be accepted without error")
	require.Equal(t, baseline.Chat, viaLegacyEnv.Chat, "AGENT_CHAT_II must not affect resolved configuration")
	os.Unsetenv("AGENT_CHAT_II")

	viaFlag, err := Load(Options{Chat: ChatConfig{II: "/custom/ii"}, ChatIISet: true})
	require.NoError(t, err, "--ii must be accepted without error")
	require.Equal(t, baseline.Chat, viaFlag.Chat, "--ii must not affect resolved configuration")
}
