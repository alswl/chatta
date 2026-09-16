package main

import "github.com/spf13/cobra"

var supervisorCmd = &cobra.Command{Use: "_supervise", Hidden: true, RunE: func(_ *cobra.Command, _ []string) error {
	return newChatService().RunSupervisor()
}}

func init() { chatCmd.AddCommand(supervisorCmd) }
