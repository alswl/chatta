package main

import "github.com/spf13/cobra"

var inboxCmd = &cobra.Command{Use: "inbox", Short: "Read or watch incoming conversations"}

func init() { chatCmd.AddCommand(inboxCmd) }
