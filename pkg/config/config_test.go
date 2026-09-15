package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChatConfigurationPrecedence(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(file, []byte("chat:\n  host: from-file\n  port: 7000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_CHAT_HOST", "from-legacy")
	config, err := Load(Options{File: file})
	if err != nil || config.Chat.Host != "from-file" {
		t.Fatalf("config file must override legacy: %+v %v", config.Chat, err)
	}
	t.Setenv("CHATTA_CHAT_HOST", "from-primary")
	config, err = Load(Options{File: file})
	if err != nil || config.Chat.Host != "from-primary" {
		t.Fatalf("primary environment must override config: %+v %v", config.Chat, err)
	}
	config, err = Load(Options{File: file, Chat: ChatConfig{Host: "from-flag"}, ChatHostSet: true})
	if err != nil || config.Chat.Host != "from-flag" {
		t.Fatalf("flag must override environment: %+v %v", config.Chat, err)
	}
}

func TestDeprecatedIISettingsAreAcceptedAndIgnored(t *testing.T) {
	baseline, err := Load(Options{})
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CHATTA_CHAT_II", "/custom/ii")
	viaPrimaryEnv, err := Load(Options{})
	if err != nil {
		t.Fatalf("CHATTA_CHAT_II must be accepted without error: %v", err)
	}
	if viaPrimaryEnv.Chat != baseline.Chat {
		t.Fatalf("CHATTA_CHAT_II must not affect resolved configuration: %+v", viaPrimaryEnv.Chat)
	}
	os.Unsetenv("CHATTA_CHAT_II")

	t.Setenv("AGENT_CHAT_II", "/custom/ii")
	viaLegacyEnv, err := Load(Options{})
	if err != nil {
		t.Fatalf("AGENT_CHAT_II must be accepted without error: %v", err)
	}
	if viaLegacyEnv.Chat != baseline.Chat {
		t.Fatalf("AGENT_CHAT_II must not affect resolved configuration: %+v", viaLegacyEnv.Chat)
	}
	os.Unsetenv("AGENT_CHAT_II")

	viaFlag, err := Load(Options{Chat: ChatConfig{II: "/custom/ii"}, ChatIISet: true})
	if err != nil {
		t.Fatalf("--ii must be accepted without error: %v", err)
	}
	if viaFlag.Chat != baseline.Chat {
		t.Fatalf("--ii must not affect resolved configuration: %+v", viaFlag.Chat)
	}
}
