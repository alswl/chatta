package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/services"
	"github.com/spf13/cobra"
)

var codexCmd = &cobra.Command{Use: "codex", Short: "Bind a chat session to a Codex app-server thread"}

func init() {
	bind := &cobra.Command{Use: "bind", Args: cobra.NoArgs, Short: "Bind an explicit local app-server thread", RunE: bindCodex}
	bind.Flags().String("endpoint", "", "local Unix app-server endpoint (unix:///path)")
	bind.Flags().String("thread", "", "explicit Codex thread ID")
	bind.Flags().Bool("replay", false, "deliver existing messages from the beginning")
	bind.Flags().Bool("mention-current-identity", false, "opt in to channel messages that mention this session identity")
	bind.Flags().StringArray("subscribe-channel", nil, "opt in to all messages in a channel (repeatable, e.g. --subscribe-channel #agents)")
	bind.Flags().Bool("current", false, "verify and bind CODEX_THREAD_ID from this Codex conversation")
	bind.Flags().Bool("if-needed", false, "reuse a matching current binding without resetting its cursor; reject a conflicting binding")
	codexCmd.AddCommand(bind)
	codexCmd.AddCommand(withJSON(&cobra.Command{Use: "status", Args: cobra.NoArgs, Short: "Show the Codex binding and delivery queue state", RunE: statusCodex}))
	codexCmd.AddCommand(&cobra.Command{Use: "unbind", Args: cobra.NoArgs, Short: "Stop Codex dispatch and retain pending events", RunE: unbindCodex})
	chatCmd.AddCommand(codexCmd)
}

func bindCodex(cmd *cobra.Command, _ []string) error {
	channels, err := cmd.Flags().GetStringArray("subscribe-channel")
	if err != nil {
		return err
	}
	endpoint, thread := mustString(cmd, "endpoint"), mustString(cmd, "thread")
	current := mustBool(cmd, "current")
	reuse := mustBool(cmd, "if-needed")
	if reuse && (!current || mustBool(cmd, "replay")) {
		return fmt.Errorf("--if-needed requires --current and cannot be combined with --replay")
	}
	if current {
		if cmd.Flags().Changed("thread") {
			return fmt.Errorf("choose --current or --thread, not both")
		}
		thread = os.Getenv("CODEX_THREAD_ID")
		if thread == "" {
			return fmt.Errorf("CODEX_THREAD_ID is unavailable; run --current inside the target Codex conversation")
		}
		if endpoint == "" {
			status, ensureErr := services.EnsureCodexAppServer(cmd.Context(), os.Getenv("CHATTA_CODEX_SERVICE_HOME"), os.Getenv("CHATTA_CODEX_ENDPOINT"))
			if ensureErr != nil {
				return ensureErr
			}
			endpoint = status.Endpoint
		}
	} else if endpoint == "" || thread == "" {
		return fmt.Errorf("bind requires --endpoint and --thread, or --current")
	}
	var b common.CodexBinding
	if reuse {
		b, err = newChatService().CodexConnectCurrent(cmd.Context(), endpoint, thread, mustBool(cmd, "mention-current-identity"), channels)
	} else if current {
		b, err = newChatService().CodexBindCurrent(cmd.Context(), endpoint, thread, mustBool(cmd, "replay"), mustBool(cmd, "mention-current-identity"), channels)
	} else {
		b, err = newChatService().CodexBind(cmd.Context(), endpoint, thread, mustBool(cmd, "replay"), mustBool(cmd, "mention-current-identity"), channels)
	}
	if err != nil {
		return err
	}
	return emitCodexBinding(cmd, &b)
}
func statusCodex(cmd *cobra.Command, _ []string) error {
	status, err := newChatService().CodexStatus(cmd.Context())
	if err != nil {
		return err
	}
	if status == nil || status.Binding == nil {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "no Codex binding")
		return err
	}
	return emitCodexStatus(cmd, status)
}
func unbindCodex(cmd *cobra.Command, _ []string) error {
	b, err := newChatService().CodexUnbind(cmd.Context())
	if err != nil {
		return err
	}
	if b == nil {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "no Codex binding")
		return err
	}
	return emitCodexBinding(cmd, b)
}
func emitCodexBinding(cmd *cobra.Command, b *common.CodexBinding) error {
	if wantsJSON(cmd) {
		return emitJSON(cmd, b)
	}
	state := "disabled"
	if b.Enabled {
		state = "enabled"
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "codex=%s target_state=%s thread=%s endpoint=%s offset=%d mention_current_identity=%t subscribed_channels=%s\n", state, codexTargetState(b), b.ThreadID, b.Endpoint, b.Offset, b.MentionCurrentIdentity, strings.Join(b.SubscribedChannels, ","))
	if err == nil && b.DeliveryBlocked {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "binding configured; target unavailable state=%s error=%q\n", b.DeliveryState, b.DeliveryError)
	}
	return err
}

func emitCodexStatus(cmd *cobra.Command, status *common.CodexDeliveryStatus) error {
	if wantsJSON(cmd) {
		return emitJSON(cmd, status)
	}
	b := status.Binding
	state := "disabled"
	if b.Enabled {
		state = "enabled"
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "codex=%s target_state=%s thread=%s endpoint=%s offset=%d mention_current_identity=%t subscribed_channels=%s\n", state, codexTargetState(b), b.ThreadID, b.Endpoint, b.Offset, b.MentionCurrentIdentity, strings.Join(b.SubscribedChannels, ",")); err != nil {
		return err
	}
	if status.Queue == nil {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "queue=unknown")
		return err
	}
	q := status.Queue
	if q.ErrorOffset != nil {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "queue pending_records=%d pending_bytes=%d blocked=%t error_offset=%d error=%q\n", q.PendingRecords, q.PendingBytes, q.Blocked, *q.ErrorOffset, q.Error)
		return err
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "queue pending_records=%d pending_bytes=%d blocked=%t", q.PendingRecords, q.PendingBytes, q.Blocked)
	if err != nil {
		return err
	}
	if q.Error != "" {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), " error=%q", q.Error)
		if err != nil {
			return err
		}
	}
	if _, err = fmt.Fprintln(cmd.OutOrStdout()); err != nil {
		return err
	}
	if b.DeliveryBlocked {
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "delivery blocked=true state=%s error=%q\n", b.DeliveryState, b.DeliveryError)
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), "delivery blocked=false")
	return err
}
func mustString(cmd *cobra.Command, name string) string {
	value, _ := cmd.Flags().GetString(name)
	return value
}

func codexTargetState(b *common.CodexBinding) string {
	if !b.Enabled {
		return "disabled"
	}
	if b.DeliveryState == "" {
		return "unchecked"
	}
	return b.DeliveryState
}
