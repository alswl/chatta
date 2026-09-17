package main

import "github.com/spf13/cobra"

func newDirectCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.ExactArgs(2), Short: short, Hidden: hidden, RunE: sendDirectMessage}
}

func sendDirectMessage(cmd *cobra.Command, args []string) error {
	return newChatService().DM(cmd.Context(), args[0], args[1])
}

func init() {
	messageCmd.AddCommand(newDirectCmd("direct <nick> <text>", "Send a direct message", false))
}
