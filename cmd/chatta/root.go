package main

import (
	"fmt"
	"os"

	"github.com/alswl/chatta/pkg/config"
	"github.com/spf13/cobra"
)

var (
	configFile string
	verbose    bool

	appConfig config.Config
)

var rootCmd = &cobra.Command{
	Use:           "chatta",
	Short:         "A command-line tool for Chatta",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.AddCommand(versionCmd)
	rootCmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		var err error
		appConfig, err = config.Load(config.Options{
			File:       configFile,
			Verbose:    verbose,
			VerboseSet: rootCmd.PersistentFlags().Changed("verbose"),
			Chat: config.ChatConfig{
				Home: chatHome, Host: chatHost, Port: chatPort,
				Channel: chatChannel, II: chatII,
			},
			ChatHomeSet:    chatCmd.PersistentFlags().Changed("home"),
			ChatHostSet:    chatCmd.PersistentFlags().Changed("host"),
			ChatPortSet:    chatCmd.PersistentFlags().Changed("port"),
			ChatChannelSet: chatCmd.PersistentFlags().Changed("channel"),
			ChatIISet:      chatCmd.PersistentFlags().Changed("ii"),
		})
		return err
	}
}

func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
