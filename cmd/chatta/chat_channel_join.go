package main

import "github.com/spf13/cobra"

func newJoinCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.ExactArgs(1), Short: short, Hidden: hidden, RunE: joinChannel}
}

func joinChannel(cmd *cobra.Command, args []string) error {
	return newChatService().Join(cmd.Context(), args[0])
}

func init() {
	channelCmd.AddCommand(newJoinCmd("join <channel>", "Join and remember a channel", false))
}
