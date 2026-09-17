package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newStartCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.RangeArgs(1, 2), Short: short, Hidden: hidden, RunE: startSession}
	cmd.Flags().Bool("takeover", false, "replace a client after user confirmation")
	return cmd
}

func startSession(cmd *cobra.Command, args []string) error {
	role := "agent"
	if len(args) == 2 {
		role = args[1]
	}
	if err := newChatService().Start(cmd.Context(), args[0], role, mustBool(cmd, "takeover")); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "chat session started")
	return nil
}

func init() {
	sessionCmd.AddCommand(newStartCmd("start <nick> [role]", "Start an owner-bound chat session", false))
}
