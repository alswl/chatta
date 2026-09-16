package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHealthCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: checkHealth}
	cmd.Flags().Bool("deep", true, "probe the server link, not just the local client")
	return withJSON(cmd)
}

func checkHealth(cmd *cobra.Command, _ []string) error {
	r := newChatService().Health(cmd.Context(), mustBool(cmd, "deep"))
	if wantsJSON(cmd) {
		if err := emitJSON(cmd, r); err != nil {
			return err
		}
		if r.Failure != "" {
			return fmt.Errorf("health: %s", r.Failure)
		}
		return nil
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "owner=%t supervisor=%t channels=%t connected=%t server=%t membership=%t\n", r.Owner, r.Supervisor, r.JoinedChannels, r.Connected, r.ServerLink, r.Membership); err != nil {
		return err
	}
	if r.Failure != "" {
		return fmt.Errorf("health: %s", r.Failure)
	}
	return nil
}

func init() {
	sessionCmd.AddCommand(newHealthCmd("status", "Check owner, client, and server health", false))
}
