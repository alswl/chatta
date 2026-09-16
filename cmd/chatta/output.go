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
// trailing newline. A successful command with nothing to report passes an
// empty slice and prints `[]`; a failing one passes the nil it got back and
// prints `null`, so a consumer has to check the exit code before indexing.
// contracts/cli-commands.md records that asymmetry.
func emitJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
