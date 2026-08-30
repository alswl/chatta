package main

import (
	"fmt"

	"github.com/alswl/chatta/pkg/chat"
	"github.com/spf13/cobra"
)

var (
	chatHome, chatHost, chatChannel, chatII string
	chatPort                                int
)

var chatCmd = &cobra.Command{
	Use: "chat", Short: "Coordinate agent sessions over a local chat bus",
}

func init() {
	flags := chatCmd.PersistentFlags()
	flags.StringVar(&chatHome, "home", "", "client home path")
	flags.StringVar(&chatHost, "host", "", "chat server host")
	flags.IntVar(&chatPort, "port", 0, "chat server port")
	flags.StringVar(&chatChannel, "channel", "", "home channel")
	flags.StringVar(&chatII, "ii", "", "ii executable path")
	chatCmd.AddCommand(startCmd, healthCmd, joinCmd, partCmd, sendCmd, dmCmd, pollCmd, watchCmd, whoCmd, stopCmd, clientsCmd, gcCmd, supervisorCmd)
	rootCmd.AddCommand(chatCmd)
}

func newChatManager() *chat.Manager { return chat.NewManager(appConfig.Chat) }

var startCmd = &cobra.Command{
	Use: "start <nick> [role]", Args: cobra.RangeArgs(1, 2),
	Short: "Start an owner-bound chat session",
	RunE: func(cmd *cobra.Command, args []string) error {
		role := "agent"
		if len(args) == 2 {
			role = args[1]
		}
		if err := newChatManager().Start(args[0], role, mustBool(cmd, "takeover")); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "chat session started")
		return nil
	},
}

var healthCmd = &cobra.Command{Use: "health", Short: "Check owner, client, and server health", RunE: func(cmd *cobra.Command, _ []string) error {
	m := newChatManager()
	r := m.Health(true)
	fmt.Fprintf(cmd.OutOrStdout(), "owner=%t supervisor=%t channels=%t reader=%t server=%t membership=%t\n", r.Owner, r.Supervisor, r.JoinedChannels, r.ClientReader, r.ServerLink, r.Membership)
	if r.Failure != "" {
		return fmt.Errorf("health: %s", r.Failure)
	}
	return nil
}}

var joinCmd = &cobra.Command{Use: "join <channel>", Args: cobra.ExactArgs(1), Short: "Join and remember a channel", RunE: func(_ *cobra.Command, args []string) error { return newChatManager().Join(args[0]) }}
var partCmd = &cobra.Command{Use: "part <channel> [reason]", Args: cobra.RangeArgs(1, 2), Short: "Leave a remembered channel", RunE: func(_ *cobra.Command, args []string) error {
	reason := "done here"
	if len(args) == 2 {
		reason = args[1]
	}
	return newChatManager().Part(args[0], reason)
}}
var sendCmd = &cobra.Command{Use: "send <text>", Args: cobra.ExactArgs(1), Short: "Send a message to a joined channel", RunE: func(cmd *cobra.Command, args []string) error {
	channel, _ := cmd.Flags().GetString("channel")
	return newChatManager().Send(channel, args[0])
}}
var dmCmd = &cobra.Command{Use: "dm <nick> <text>", Args: cobra.ExactArgs(2), Short: "Send a direct message", RunE: func(_ *cobra.Command, args []string) error { return newChatManager().DM(args[0], args[1]) }}
var pollCmd = &cobra.Command{Use: "poll", Short: "Print incoming messages", RunE: func(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	lines, err := newChatManager().Poll(all)
	for _, line := range lines {
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	return err
}}
var watchCmd = &cobra.Command{Use: "watch", Short: "Stream incoming messages", RunE: func(cmd *cobra.Command, _ []string) error {
	return newChatManager().Watch(cmd.Context().Done(), func(line string) { fmt.Fprintln(cmd.OutOrStdout(), line) })
}}
var whoCmd = &cobra.Command{Use: "who [channel]", Args: cobra.MaximumNArgs(1), Short: "List channel members", RunE: func(cmd *cobra.Command, args []string) error {
	target := ""
	if len(args) == 1 {
		target = args[0]
	}
	lines, err := newChatManager().Who(target)
	for _, line := range lines {
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	return err
}}
var stopCmd = &cobra.Command{Use: "stop", Short: "Stop the owned client", RunE: func(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	return newChatManager().Stop(force)
}}
var clientsCmd = &cobra.Command{Use: "clients", Short: "Survey local chat clients", RunE: func(cmd *cobra.Command, _ []string) error {
	rows, err := newChatManager().Survey()
	for _, row := range rows {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\n", row.SessionSummary, row.ClientProcessState, row.CleanupEligibility)
	}
	return err
}}
var gcCmd = &cobra.Command{Use: "gc", Short: "Clean up abandoned clients", RunE: func(cmd *cobra.Command, _ []string) error {
	dry, _ := cmd.Flags().GetBool("dry-run")
	prune, _ := cmd.Flags().GetBool("prune")
	out, err := newChatManager().GC(dry, prune)
	fmt.Fprint(cmd.OutOrStdout(), out)
	return err
}}
var supervisorCmd = &cobra.Command{Use: "_supervise", Hidden: true, RunE: func(_ *cobra.Command, _ []string) error { return newChatManager().RunSupervisor() }}

func mustBool(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}

func init() {
	startCmd.Flags().Bool("takeover", false, "replace a client after user confirmation")
	sendCmd.Flags().StringP("channel", "c", "", "channel to send to")
	pollCmd.Flags().Bool("all", false, "replay available history")
	stopCmd.Flags().Bool("force", false, "stop a client owned by another session")
	gcCmd.Flags().Bool("dry-run", false, "report actions without changing anything")
	gcCmd.Flags().Bool("prune", false, "remove confirmed-dead client directories")
}
