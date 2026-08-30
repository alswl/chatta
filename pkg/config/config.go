package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config contains process-wide CLI configuration.
type Config struct {
	Verbose bool
}

// Options controls how configuration is loaded.
type Options struct {
	File       string
	Verbose    bool
	VerboseSet bool
}

// Load applies the precedence: explicit options/flags, environment, config file, defaults.
func Load(options Options) (Config, error) {
	v := viper.New()
	v.SetDefault("verbose", false)
	v.SetEnvPrefix("CHATTA")
	v.AutomaticEnv()

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

	if options.VerboseSet {
		v.Set("verbose", options.Verbose)
	}

	return Config{Verbose: v.GetBool("verbose")}, nil
}
