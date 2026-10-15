package main

import (
	"bytes"
	"testing"

	"github.com/alswl/chatta/pkg/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// `null` is the one output shape that breaks a consumer instead of merely
// disappointing it: `jq '.[]'` iterates an empty array quietly and errors on
// null, which buries the real reason -- on stderr -- under a type complaint.
// A failing command hands back the nil slice from its error path, so this is
// the case that matters.
func TestEmitJSONListNeverWritesNull(t *testing.T) {
	for _, tc := range []struct {
		name string
		emit func(*cobra.Command) error
	}{
		{"nil messages", func(c *cobra.Command) error { return emitJSONList(c, []common.StoredMessage(nil)) }},
		{"nil clients", func(c *cobra.Command) error { return emitJSONList(c, []common.ClientSurvey(nil)) }},
		{"empty clients", func(c *cobra.Command) error { return emitJSONList(c, []common.ClientSurvey{}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			require.NoError(t, tc.emit(cmd))
			require.Equal(t, "[]\n", out.String())
		})
	}
}

func TestEmitCodexStatusShowsDeliveryBlockInHumanAndJSON(t *testing.T) {
	status := &common.CodexDeliveryStatus{
		Binding: &common.CodexBinding{Enabled: true, ThreadID: "thread-a", Endpoint: "unix:///tmp/app.sock", Offset: 24, DeliveryBlocked: true, DeliveryState: "not_loaded", DeliveryError: "thread not loaded"},
		Queue:   &common.CodexQueueStatus{PendingRecords: 1, PendingBytes: 128},
	}
	t.Run("human", func(t *testing.T) {
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		require.NoError(t, emitCodexStatus(cmd, status))
		require.Contains(t, out.String(), `delivery blocked=true state=not_loaded error="thread not loaded"`)
	})
	t.Run("json", func(t *testing.T) {
		var out bytes.Buffer
		cmd := withJSON(&cobra.Command{})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Flags().Set("json", "true"))
		require.NoError(t, emitCodexStatus(cmd, status))
		require.JSONEq(t, `{"binding":{"binding_id":"","session_id":"","endpoint":"unix:///tmp/app.sock","thread_id":"thread-a","enabled":true,"offset":24,"replay":false,"delivery_blocked":true,"delivery_state":"not_loaded","delivery_error":"thread not loaded","updated_at":0},"queue":{"pending_records":1,"pending_bytes":128,"blocked":false}}`, out.String())
	})
}

func TestEmitCodexStatusShowsExplicitGroupRoutingOptions(t *testing.T) {
	status := &common.CodexDeliveryStatus{
		Binding: &common.CodexBinding{Enabled: true, ThreadID: "thread-a", Endpoint: "unix:///tmp/app.sock", MentionCurrentIdentity: true, SubscribedChannels: []string{"#agents"}},
		Queue:   &common.CodexQueueStatus{},
	}
	t.Run("human", func(t *testing.T) {
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		require.NoError(t, emitCodexStatus(cmd, status))
		require.Contains(t, out.String(), "mention_current_identity=true subscribed_channels=#agents")
	})
	t.Run("json", func(t *testing.T) {
		var out bytes.Buffer
		cmd := withJSON(&cobra.Command{})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Flags().Set("json", "true"))
		require.NoError(t, emitCodexStatus(cmd, status))
		require.Contains(t, out.String(), `"mention_current_identity": true`)
		require.Contains(t, out.String(), `"subscribed_channels": [`)
		require.Contains(t, out.String(), `"#agents"`)
	})
}

func TestEmitJSONListKeepsContent(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	require.NoError(t, emitJSONList(cmd, []common.ChannelMember{{Nick: "misky", You: true}}))
	require.JSONEq(t, `[{"nick":"misky","you":true}]`, out.String())
}

func TestEmitCodexStatusShowsQueueBlockInHumanAndJSON(t *testing.T) {
	errorOffset := int64(42)
	status := &common.CodexDeliveryStatus{
		Binding: &common.CodexBinding{Enabled: true, ThreadID: "thread-a", Endpoint: "unix:///tmp/app.sock", Offset: errorOffset},
		Queue:   &common.CodexQueueStatus{PendingRecords: 1, PendingBytes: 128, Blocked: true, Error: "invalid message JSONL record at byte 42", ErrorOffset: &errorOffset},
	}
	t.Run("human", func(t *testing.T) {
		var out bytes.Buffer
		cmd := &cobra.Command{}
		cmd.SetOut(&out)
		require.NoError(t, emitCodexStatus(cmd, status))
		require.Contains(t, out.String(), "queue pending_records=1 pending_bytes=128 blocked=true")
		require.Contains(t, out.String(), "error_offset=42")
		require.Contains(t, out.String(), `error="invalid message JSONL record at byte 42"`)
	})
	t.Run("json", func(t *testing.T) {
		var out bytes.Buffer
		cmd := withJSON(&cobra.Command{})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Flags().Set("json", "true"))
		require.NoError(t, emitCodexStatus(cmd, status))
		require.JSONEq(t, `{"binding":{"binding_id":"","session_id":"","endpoint":"unix:///tmp/app.sock","thread_id":"thread-a","enabled":true,"offset":42,"replay":false,"updated_at":0},"queue":{"pending_records":1,"pending_bytes":128,"blocked":true,"error":"invalid message JSONL record at byte 42","error_offset":42}}`, out.String())
	})
}
