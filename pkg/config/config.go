package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Config contains process-wide CLI configuration.
type Config struct {
	Verbose bool
	Chat    ChatConfig
}

// ChatConfig contains settings for the local agent-chat client.
type ChatConfig struct {
	Home    string
	Host    string
	Port    int
	Channel string
	II      string
}

// Options controls how configuration is loaded.
type Options struct {
	File                                                             string
	Verbose                                                          bool
	VerboseSet                                                       bool
	Chat                                                             ChatConfig
	ChatHomeSet, ChatHostSet, ChatPortSet, ChatChannelSet, ChatIISet bool
}

// Load applies the precedence: explicit options/flags, environment, config file, defaults.
func Load(options Options) (Config, error) {
	v := viper.New()
	v.SetDefault("verbose", false)
	v.SetDefault("chat.home", "")
	v.SetDefault("chat.host", "127.0.0.1")
	v.SetDefault("chat.port", 6667)
	v.SetDefault("chat.channel", "#agents")
	v.SetDefault("chat.ii", "ii")
	v.SetEnvPrefix("CHATTA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	if options.File != "" {
		v.SetConfigFile(options.File)
	} else if dir, err := os.UserConfigDir(); err == nil {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(filepath.Join(dir, "chatta"))
	} else {
		return Config{}, fmt.Errorf("resolve user config directory: %w", err)
	}

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if options.File != "" || !errors.As(err, &notFound) {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	chat := ChatConfig{Home: v.GetString("chat.home"), Host: v.GetString("chat.host"), Port: v.GetInt("chat.port"), Channel: v.GetString("chat.channel"), II: v.GetString("chat.ii")}
	v.AutomaticEnv()
	if options.VerboseSet {
		v.Set("verbose", options.Verbose)
	}
	if err := applyLegacyChatEnvironment(&chat, v); err != nil {
		return Config{}, err
	}
	if err := applyChatEnvironment(&chat, "CHATTA_CHAT_"); err != nil {
		return Config{}, err
	}
	if options.ChatHomeSet {
		chat.Home = options.Chat.Home
	}
	if options.ChatHostSet {
		chat.Host = options.Chat.Host
	}
	if options.ChatPortSet {
		chat.Port = options.Chat.Port
	}
	if options.ChatChannelSet {
		chat.Channel = options.Chat.Channel
	}
	if options.ChatIISet {
		chat.II = options.Chat.II
	}

	return Config{Verbose: v.GetBool("verbose"), Chat: chat}, nil
}

func applyChatEnvironment(chat *ChatConfig, prefix string) error {
	if value, ok := os.LookupEnv(prefix + "HOME"); ok && value != "" {
		chat.Home = value
	}
	if value, ok := os.LookupEnv(prefix + "HOST"); ok && value != "" {
		chat.Host = value
	}
	if value, ok := os.LookupEnv(prefix + "CHANNEL"); ok && value != "" {
		chat.Channel = value
	}
	if value, ok := os.LookupEnv(prefix + "II"); ok && value != "" {
		chat.II = value
	}
	if value, ok := os.LookupEnv(prefix + "PORT"); ok && value != "" {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid %sPORT %q", prefix, value)
		}
		chat.Port = port
	}
	return nil
}

func applyLegacyChatEnvironment(chat *ChatConfig, v *viper.Viper) error {
	if value, ok := os.LookupEnv("AGENT_CHAT_HOME"); ok && value != "" && !v.InConfig("chat.home") {
		chat.Home = value
	}
	if value, ok := os.LookupEnv("AGENT_CHAT_HOST"); ok && value != "" && !v.InConfig("chat.host") {
		chat.Host = value
	}
	if value, ok := os.LookupEnv("AGENT_CHAT_CHANNEL"); ok && value != "" && !v.InConfig("chat.channel") {
		chat.Channel = value
	}
	if value, ok := os.LookupEnv("AGENT_CHAT_II"); ok && value != "" && !v.InConfig("chat.ii") {
		chat.II = value
	}
	if value, ok := os.LookupEnv("AGENT_CHAT_PORT"); ok && value != "" && !v.InConfig("chat.port") {
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid AGENT_CHAT_PORT %q", value)
		}
		chat.Port = port
	}
	return nil
}
