package main

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// withJSON registers the machine-readable alternative to a command's default
// human output. The default form is already plain and greppable, so --plain
// would be a synonym for it and is not offered.
func withJSON(cmd *cobra.Command) *cobra.Command {
	cmd.Flags().Bool("json", false, "emit JSON instead of the human-readable form")
	return cmd
}

func wantsJSON(cmd *cobra.Command) bool { return mustBool(cmd, "json") }

// emitJSON writes v to the command's stdout as one indented document with a
// trailing newline. Collections print as `[]` rather than `null` when empty,
// so a consumer can index the result without a nil check.
func emitJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
