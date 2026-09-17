package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMembersCmd(use, short string, hidden bool) *cobra.Command {
	return withJSON(&cobra.Command{Use: use, Args: cobra.MaximumNArgs(1), Short: short, Hidden: hidden, RunE: channelMembers})
}

func channelMembers(cmd *cobra.Command, args []string) error {
	target := ""
	if len(args) == 1 {
		target = args[0]
	}
	if wantsJSON(cmd) {
		members, err := newChatService().Members(cmd.Context(), target)
		if err != nil {
			return err
		}
		return emitJSON(cmd, members)
	}
	lines, err := newChatService().Who(cmd.Context(), target)
	for _, line := range lines {
		if _, writeErr := fmt.Fprintln(cmd.OutOrStdout(), line); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func init() {
	channelCmd.AddCommand(newMembersCmd("members [channel]", "List channel members", false))
}
