package main

import "github.com/spf13/cobra"

var clientCmd = &cobra.Command{Use: "client", Short: "Inspect and clean up local chat clients"}

func init() { chatCmd.AddCommand(clientCmd) }
