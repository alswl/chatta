package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newReadCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: readInbox}
	cmd.Flags().Bool("all", false, "replay available history")
	return withJSON(cmd)
}

func readInbox(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	if wantsJSON(cmd) {
		msgs, err := newChatService().PollMessages(cmd.Context(), all)
		if jsonErr := emitJSON(cmd, msgs); jsonErr != nil {
			return jsonErr
		}
		return err
	}
	lines, err := newChatService().Poll(cmd.Context(), all)
	for _, line := range lines {
		if _, writeErr := fmt.Fprintln(cmd.OutOrStdout(), line); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func init() {
	inboxCmd.AddCommand(newReadCmd("read", "Print incoming messages", false))
}
