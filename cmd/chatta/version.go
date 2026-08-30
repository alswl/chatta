package main

import (
	"fmt"

	buildversion "github.com/alswl/chatta/pkg/version"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, _ []string) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "version: %s\ncommit: %s\n", buildversion.Version, buildversion.Commit)
	},
}
