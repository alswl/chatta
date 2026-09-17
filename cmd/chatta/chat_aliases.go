package main

// Flat commands remain available for existing skills and shell scripts, but the
// grouped command tree is the documented public interface. They share each
// subcommand's factory so an alias can never drift from the command it aliases.
func init() {
	chatCmd.AddCommand(
		newStartCmd("start <nick> [role]", "Compatibility alias for session start", true),
		newHealthCmd("health", "Compatibility alias for session status", true),
		newJoinCmd("join <channel>", "Compatibility alias for channel join", true),
		newLeaveCmd("part <channel> [reason]", "Compatibility alias for channel leave", true),
		newSendCmd("send <text>", "Compatibility alias for message send", true),
		newDirectCmd("dm <nick> <text>", "Compatibility alias for message direct", true),
		newReadCmd("poll", "Compatibility alias for inbox read", true),
		newWatchCmd("watch", "Compatibility alias for inbox watch", true),
		newMembersCmd("who [channel]", "Compatibility alias for channel members", true),
		newStopCmd("stop", "Compatibility alias for session stop", true),
		newListCmd("clients", "Compatibility alias for client list", true),
		newGCCmd("gc", "Compatibility alias for client gc", true),
	)
}
