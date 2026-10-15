package main

import "github.com/spf13/cobra"

var codexServiceCmd = &cobra.Command{Use: "service", Short: "Manage the shared Codex app-server"}

func init() {
	codexCmd.AddCommand(codexServiceCmd)
}
