package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newGCCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: collectGarbage}
	cmd.Flags().Bool("dry-run", false, "report actions without changing anything")
	cmd.Flags().Bool("prune", false, "remove confirmed-dead client directories")
	return withJSON(cmd)
}

func collectGarbage(cmd *cobra.Command, _ []string) error {
	dry, _ := cmd.Flags().GetBool("dry-run")
	prune, _ := cmd.Flags().GetBool("prune")
	if wantsJSON(cmd) {
		rows, err := newChatService().GCReport(cmd.Context(), dry, prune)
		if jsonErr := emitJSONList(cmd, rows); jsonErr != nil {
			return jsonErr
		}
		return err
	}
	out, err := newChatService().GC(cmd.Context(), dry, prune)
	if _, writeErr := fmt.Fprint(cmd.OutOrStdout(), out); writeErr != nil {
		return writeErr
	}
	return err
}

func init() {
	clientCmd.AddCommand(newGCCmd("gc", "Clean up abandoned clients", false))
}
