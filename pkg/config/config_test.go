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

func TestDeprecatedIIInputsAreAcceptedAndReported(t *testing.T) {
	t.Setenv("CHATTA_CHAT_II", "/nonexistent/ii")
	config, err := Load(Options{DeprecatedIISet: true})
	require.NoError(t, err)
	require.True(t, config.DeprecatedIISet)
	require.Equal(t, "127.0.0.1", config.Chat.Host)

	t.Setenv("CHATTA_CHAT_II", "")
	t.Setenv("AGENT_CHAT_II", "whatever")
	config, err = Load(Options{})
	require.NoError(t, err)
	require.True(t, config.DeprecatedIISet)
}
