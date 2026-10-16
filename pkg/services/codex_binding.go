package services

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/daemon"
	"github.com/alswl/chatta/pkg/dal"
)

func inspectCodexQueue(path string, offset int64) common.CodexQueueStatus {
	queue := common.CodexQueueStatus{}
	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		queue.Blocked = true
		queue.Error = fmt.Sprintf("stat Codex message queue: %v", err)
		return queue
	}
	if err == nil {
		queue.PendingBytes = info.Size() - offset
		if queue.PendingBytes < 0 {
			queue.PendingBytes = 0
		}
	}
	records, _, err := dal.ReadCompleteMessageRecords(path, offset)
	queue.PendingRecords = len(records)
	if err == nil {
		return queue
	}
	queue.Blocked = true
	queue.Error = err.Error()
	var invalid *dal.InvalidMessageRecordError
	if errors.As(err, &invalid) {
		offset := invalid.Offset
		queue.ErrorOffset = &offset
	}
	return queue
}

func (m *ChatService) CodexBind(ctx context.Context, endpoint, threadID string, replay, mentionCurrentIdentity bool, subscribedChannels []string) (common.CodexBinding, error) {
	return m.codexBind(ctx, daemon.ControlRequest{Op: "codex.bind", Endpoint: endpoint, ThreadID: threadID, Replay: replay, MentionCurrentIdentity: mentionCurrentIdentity, SubscribedChannels: subscribedChannels})
}

func (m *ChatService) CodexBindCurrent(ctx context.Context, endpoint, threadID string, replay, mention bool, channels []string) (common.CodexBinding, error) {
	return m.codexBind(ctx, daemon.ControlRequest{Op: "codex.bind", Endpoint: endpoint, ThreadID: threadID, Replay: replay, MentionCurrentIdentity: mention, SubscribedChannels: channels, RequireAvailable: true})
}

func (m *ChatService) CodexConnectCurrent(ctx context.Context, endpoint, threadID string, mention bool, channels []string) (common.CodexBinding, error) {
	return m.codexBind(ctx, daemon.ControlRequest{Op: "codex.bind", Endpoint: endpoint, ThreadID: threadID, MentionCurrentIdentity: mention, SubscribedChannels: channels, RequireAvailable: true, ReuseExisting: true})
}

func (m *ChatService) codexBind(ctx context.Context, req daemon.ControlRequest) (common.CodexBinding, error) {
	if _, err := m.Ensure(ctx); err != nil {
		return common.CodexBinding{}, err
	}
	binding, err := m.codexControl(ctx, req)
	if binding == nil {
		return common.CodexBinding{}, err
	}
	return *binding, err
}

func (m *ChatService) CodexStatus(ctx context.Context) (*common.CodexDeliveryStatus, error) {
	if _, err := m.Ensure(ctx); err != nil {
		return nil, err
	}
	resp, err := daemon.Request(ctx, m.Paths.ControlSock, daemon.ControlRequest{Op: "codex.status"})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, errors.New(resp.Error)
	}
	return &common.CodexDeliveryStatus{Binding: resp.Codex, Queue: resp.CodexQueue}, nil
}

func (m *ChatService) CodexUnbind(ctx context.Context) (*common.CodexBinding, error) {
	if _, err := m.Ensure(ctx); err != nil {
		return nil, err
	}
	return m.codexControl(ctx, daemon.ControlRequest{Op: "codex.unbind"})
}

// hasActiveCodexBinding is a liveness lease for a native Codex session. Codex
// invokes tools from short-lived turn processes, so their PID cannot decide
// whether the supervisor that wakes the same native session should stay alive.
func hasActiveCodexBinding(paths dal.Paths, sessionID string) bool {
	binding, err := dal.LoadCodexBinding(paths.CodexBinding)
	return err == nil && binding.Enabled && binding.SessionID == sessionID
}

func (m *ChatService) codexControl(ctx context.Context, req daemon.ControlRequest) (*common.CodexBinding, error) {
	resp, err := daemon.Request(ctx, m.Paths.ControlSock, req)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, errors.New(resp.Error)
	}
	return resp.Codex, nil
}
