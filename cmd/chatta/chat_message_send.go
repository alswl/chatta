package main

import "github.com/spf13/cobra"

func newSendCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.ExactArgs(1), Short: short, Hidden: hidden, RunE: sendMessage}
	cmd.Flags().StringP("channel", "c", "", "channel to send to")
	return cmd
}

func sendMessage(cmd *cobra.Command, args []string) error {
	channel, _ := cmd.Flags().GetString("channel")
	return newChatService().Send(cmd.Context(), channel, args[0])
}

func init() {
	messageCmd.AddCommand(newSendCmd("send <text>", "Send a message to a joined channel", false))
}
