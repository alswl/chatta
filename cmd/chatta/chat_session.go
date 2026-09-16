package main

import "github.com/spf13/cobra"

var sessionCmd = &cobra.Command{Use: "session", Short: "Manage the agent-owned chat session"}

func init() { chatCmd.AddCommand(sessionCmd) }
