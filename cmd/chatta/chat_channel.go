package main

import "github.com/spf13/cobra"

var channelCmd = &cobra.Command{Use: "channel", Short: "Manage shared channel membership and members"}

func init() { chatCmd.AddCommand(channelCmd) }
