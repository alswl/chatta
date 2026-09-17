package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newWatchCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: watchInbox}
}

func watchInbox(cmd *cobra.Command, _ []string) error {
	return newChatService().Watch(cmd.Context(), func(line string) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
	})
}

func init() {
	inboxCmd.AddCommand(newWatchCmd("watch", "Stream incoming messages", false))
}
