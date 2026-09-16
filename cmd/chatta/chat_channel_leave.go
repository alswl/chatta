package main

import "github.com/spf13/cobra"

func newLeaveCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.RangeArgs(1, 2), Short: short, Hidden: hidden, RunE: leaveChannel}
}

func leaveChannel(cmd *cobra.Command, args []string) error {
	reason := "done here"
	if len(args) == 2 {
		reason = args[1]
	}
	return newChatService().Part(cmd.Context(), args[0], reason)
}

func init() {
	channelCmd.AddCommand(newLeaveCmd("leave <channel> [reason]", "Leave a remembered channel", false))
}
