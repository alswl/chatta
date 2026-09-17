package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newListCmd(use, short string, hidden bool) *cobra.Command {
	return withJSON(&cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: listClients})
}

func listClients(cmd *cobra.Command, _ []string) error {
	rows, err := newChatService().Survey(cmd.Context())
	if wantsJSON(cmd) {
		if jsonErr := emitJSONList(cmd, rows); jsonErr != nil {
			return jsonErr
		}
		return err
	}
	for _, row := range rows {
		if _, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "%s nick=%s owner=%s supervisor=%s client=%s cleanup=%s\n", row.ClientHome, row.SessionSummary, row.OwnerState, row.SupervisorState, row.ClientProcessState, row.CleanupEligibility); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func init() {
	clientCmd.AddCommand(newListCmd("list", "Survey local chat clients", false))
}
