package main

import (
	"github.com/alswl/chatta/pkg/services"
	"github.com/spf13/cobra"
)

var (
	chatHome, chatHost, chatChannel string
	chatPort                        int
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Coordinate agent sessions over a local chat bus",
	Long:  "Coordinate agent sessions by managing their session, channels, messages, inbox, and local clients.",
}

func init() {
	flags := chatCmd.PersistentFlags()
	flags.StringVar(&chatHome, "home", "", "client home path")
	flags.StringVar(&chatHost, "host", "", "chat server host")
	flags.IntVar(&chatPort, "port", 0, "chat server port")
	flags.StringVar(&chatChannel, "channel", "", "home channel")
	rootCmd.AddCommand(chatCmd)
}

func newChatService() *services.ChatService { return services.NewChatService(appConfig.Chat) }

func mustBool(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}
