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
// trailing newline.
func emitJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// emitJSONList writes a slice as a JSON array, never as `null`. A failing
// command hands back the nil slice its error path returned, and `null` is the
// one output shape that breaks a consumer rather than merely disappointing it:
// `jq '.[]'` iterates an empty array quietly and errors on null, so the reason
// for the failure -- which is on stderr -- reaches the user buried under a jq
// complaint about a type.
func emitJSONList[T any](cmd *cobra.Command, rows []T) error {
	if rows == nil {
		rows = []T{}
	}
	return emitJSON(cmd, rows)
}
