package main

import "github.com/spf13/cobra"

var messageCmd = &cobra.Command{Use: "message", Short: "Send channel and direct messages"}

func init() { chatCmd.AddCommand(messageCmd) }
