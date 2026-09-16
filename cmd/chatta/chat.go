package main

import (
	"github.com/alswl/chatta/pkg/services"
	"github.com/spf13/cobra"
)

var (
	chatHome, chatHost, chatChannel, chatII string
	chatPort                                int
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
	// Not pflag's MarkDeprecated: that prints its own warning, and the
	// contract allows exactly one deprecation line on stderr (FR-010),
	// which config emits for the flag and both env aliases alike.
	flags.StringVar(&chatII, "ii", "", "deprecated: ignored — chatta no longer shells out to ii")
	rootCmd.AddCommand(chatCmd)
}

func newChatService() *services.ChatService { return services.NewChatService(appConfig.Chat) }

func mustBool(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}
