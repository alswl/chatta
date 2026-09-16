package main

import "github.com/spf13/cobra"

func newStopCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: stopSession}
	cmd.Flags().Bool("force", false, "stop a client owned by another session")
	return cmd
}

func stopSession(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	return newChatService().Stop(cmd.Context(), force)
}

func init() {
	sessionCmd.AddCommand(newStopCmd("stop", "Stop the owned client", false))
}
