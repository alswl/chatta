package main

import (
	"fmt"

	"github.com/alswl/chatta/pkg/services"
	"github.com/spf13/cobra"
)

var (
	chatHome, chatHost, chatChannel, chatII string
	chatPort                                int
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Coordinate agent sessions over a local chat bus",
	Long:  "Coordinate agent sessions by managing their session, channels, messages, inbox, and local clients.",
}

func init() {
	flags := chatCmd.PersistentFlags()
	flags.StringVar(&chatHome, "home", "", "client home path")
	flags.StringVar(&chatHost, "host", "", "chat server host")
	flags.IntVar(&chatPort, "port", 0, "chat server port")
	flags.StringVar(&chatChannel, "channel", "", "home channel")
	flags.StringVar(&chatII, "ii", "", "deprecated: ignored — chatta no longer shells out to ii")
	_ = chatCmd.PersistentFlags().MarkDeprecated("ii", "chatta now speaks IRC in-process; this flag is ignored")
	chatCmd.AddCommand(sessionCmd, channelCmd, messageCmd, inboxCmd, clientCmd)
	chatCmd.AddCommand(startCmd, healthCmd, joinCmd, partCmd, sendCmd, dmCmd, pollCmd, watchCmd, whoCmd, stopCmd, clientsCmd, gcCmd, supervisorCmd)
	rootCmd.AddCommand(chatCmd)
}

func newChatService() *services.ChatService { return services.NewChatService(appConfig.Chat) }

func startSession(cmd *cobra.Command, args []string) error {
	role := "agent"
	if len(args) == 2 {
		role = args[1]
	}
	if err := newChatService().Start(args[0], role, mustBool(cmd, "takeover")); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "chat session started")
	return nil
}

func checkHealth(cmd *cobra.Command, _ []string) error {
	r := newChatService().Health(true)
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "owner=%t supervisor=%t channels=%t connected=%t server=%t membership=%t\n", r.Owner, r.Supervisor, r.JoinedChannels, r.Connected, r.ServerLink, r.Membership); err != nil {
		return err
	}
	if r.Failure != "" {
		return fmt.Errorf("health: %s", r.Failure)
	}
	return nil
}

func joinChannel(_ *cobra.Command, args []string) error { return newChatService().Join(args[0]) }

func leaveChannel(_ *cobra.Command, args []string) error {
	reason := "done here"
	if len(args) == 2 {
		reason = args[1]
	}
	return newChatService().Part(args[0], reason)
}

func sendMessage(cmd *cobra.Command, args []string) error {
	channel, _ := cmd.Flags().GetString("channel")
	return newChatService().Send(channel, args[0])
}

func sendDirectMessage(_ *cobra.Command, args []string) error {
	return newChatService().DM(args[0], args[1])
}

func readInbox(cmd *cobra.Command, _ []string) error {
	all, _ := cmd.Flags().GetBool("all")
	lines, err := newChatService().Poll(all)
	for _, line := range lines {
		if _, writeErr := fmt.Fprintln(cmd.OutOrStdout(), line); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func watchInbox(cmd *cobra.Command, _ []string) error {
	return newChatService().Watch(cmd.Context(), func(line string) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
	})
}

func channelMembers(cmd *cobra.Command, args []string) error {
	target := ""
	if len(args) == 1 {
		target = args[0]
	}
	lines, err := newChatService().Who(target)
	for _, line := range lines {
		if _, writeErr := fmt.Fprintln(cmd.OutOrStdout(), line); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func stopSession(cmd *cobra.Command, _ []string) error {
	force, _ := cmd.Flags().GetBool("force")
	return newChatService().Stop(force)
}

func listClients(cmd *cobra.Command, _ []string) error {
	rows, err := newChatService().Survey()
	for _, row := range rows {
		if _, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "%s nick=%s owner=%s supervisor=%s client=%s cleanup=%s\n", row.ClientHome, row.SessionSummary, row.OwnerState, row.SupervisorState, row.ClientProcessState, row.CleanupEligibility); writeErr != nil {
			return writeErr
		}
	}
	return err
}

func collectGarbage(cmd *cobra.Command, _ []string) error {
	dry, _ := cmd.Flags().GetBool("dry-run")
	prune, _ := cmd.Flags().GetBool("prune")
	out, err := newChatService().GC(dry, prune)
	if _, writeErr := fmt.Fprint(cmd.OutOrStdout(), out); writeErr != nil {
		return writeErr
	}
	return err
}

func newStartCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.RangeArgs(1, 2), Short: short, Hidden: hidden, RunE: startSession}
	cmd.Flags().Bool("takeover", false, "replace a client after user confirmation")
	return cmd
}

func newHealthCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: checkHealth}
}

func newJoinCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.ExactArgs(1), Short: short, Hidden: hidden, RunE: joinChannel}
}

func newLeaveCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.RangeArgs(1, 2), Short: short, Hidden: hidden, RunE: leaveChannel}
}

func newSendCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.ExactArgs(1), Short: short, Hidden: hidden, RunE: sendMessage}
	cmd.Flags().StringP("channel", "c", "", "channel to send to")
	return cmd
}

func newDirectCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.ExactArgs(2), Short: short, Hidden: hidden, RunE: sendDirectMessage}
}

func newReadCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: readInbox}
	cmd.Flags().Bool("all", false, "replay available history")
	return cmd
}

func newWatchCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: watchInbox}
}

func newMembersCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.MaximumNArgs(1), Short: short, Hidden: hidden, RunE: channelMembers}
}

func newStopCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: stopSession}
	cmd.Flags().Bool("force", false, "stop a client owned by another session")
	return cmd
}

func newListCmd(use, short string, hidden bool) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: listClients}
}

func newGCCmd(use, short string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{Use: use, Args: cobra.NoArgs, Short: short, Hidden: hidden, RunE: collectGarbage}
	cmd.Flags().Bool("dry-run", false, "report actions without changing anything")
	cmd.Flags().Bool("prune", false, "remove confirmed-dead client directories")
	return cmd
}

var sessionCmd = &cobra.Command{Use: "session", Short: "Manage the agent-owned chat session"}
var channelCmd = &cobra.Command{Use: "channel", Short: "Manage shared channel membership and members"}
var messageCmd = &cobra.Command{Use: "message", Short: "Send channel and direct messages"}
var inboxCmd = &cobra.Command{Use: "inbox", Short: "Read or watch incoming conversations"}
var clientCmd = &cobra.Command{Use: "client", Short: "Inspect and clean up local chat clients"}

func init() {
	sessionCmd.AddCommand(
		newStartCmd("start <nick> [role]", "Start an owner-bound chat session", false),
		newHealthCmd("status", "Check owner, client, and server health", false),
		newStopCmd("stop", "Stop the owned client", false),
	)
	channelCmd.AddCommand(
		newJoinCmd("join <channel>", "Join and remember a channel", false),
		newLeaveCmd("leave <channel> [reason]", "Leave a remembered channel", false),
		newMembersCmd("members [channel]", "List channel members", false),
	)
	messageCmd.AddCommand(
		newSendCmd("send <text>", "Send a message to a joined channel", false),
		newDirectCmd("direct <nick> <text>", "Send a direct message", false),
	)
	inboxCmd.AddCommand(
		newReadCmd("read", "Print incoming messages", false),
		newWatchCmd("watch", "Stream incoming messages", false),
	)
	clientCmd.AddCommand(
		newListCmd("list", "Survey local chat clients", false),
		newGCCmd("gc", "Clean up abandoned clients", false),
	)
}

// Flat commands remain available for existing skills and shell scripts, but the
// grouped command tree is the documented public interface.
var startCmd = newStartCmd("start <nick> [role]", "Compatibility alias for session start", true)
var healthCmd = newHealthCmd("health", "Compatibility alias for session status", true)
var joinCmd = newJoinCmd("join <channel>", "Compatibility alias for channel join", true)
var partCmd = newLeaveCmd("part <channel> [reason]", "Compatibility alias for channel leave", true)
var sendCmd = newSendCmd("send <text>", "Compatibility alias for message send", true)
var dmCmd = newDirectCmd("dm <nick> <text>", "Compatibility alias for message direct", true)
var pollCmd = newReadCmd("poll", "Compatibility alias for inbox read", true)
var watchCmd = newWatchCmd("watch", "Compatibility alias for inbox watch", true)
var whoCmd = newMembersCmd("who [channel]", "Compatibility alias for channel members", true)
var stopCmd = newStopCmd("stop", "Compatibility alias for session stop", true)
var clientsCmd = newListCmd("clients", "Compatibility alias for client list", true)
var gcCmd = newGCCmd("gc", "Compatibility alias for client gc", true)
var supervisorCmd = &cobra.Command{Use: "_supervise", Hidden: true, RunE: func(_ *cobra.Command, _ []string) error { return newChatService().RunSupervisor() }}

func mustBool(cmd *cobra.Command, name string) bool {
	value, _ := cmd.Flags().GetBool(name)
	return value
}
