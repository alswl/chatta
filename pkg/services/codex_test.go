//go:build darwin || linux

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/daemon"
	"github.com/alswl/chatta/pkg/dal"
	"github.com/stretchr/testify/require"
)

type channelLogWriter struct{ messages chan string }

func (w channelLogWriter) Write(p []byte) (int, error) {
	message := string(p)
	select {
	case w.messages <- message:
	default:
	}
	return len(p), nil
}

func TestAddressedToRequiresIdentityAndExcludesSelfEcho(t *testing.T) {
	message := common.StoredMessage{Kind: "channel", Target: "#agents", Nick: "peer", Text: "hello @agent, please look"}
	require.True(t, addressedTo(common.StoredMessage{Kind: "direct", Target: "Agent", Nick: "peer"}, "agent", common.CodexBinding{}))
	require.True(t, addressedTo(common.StoredMessage{Kind: "direct", Target: "peer", Nick: "peer"}, "agent", common.CodexBinding{}))
	require.False(t, addressedTo(message, "agent", common.CodexBinding{}), "a mention alone must not opt a session into group delivery")
	require.True(t, addressedTo(message, "agent", common.CodexBinding{MentionCurrentIdentity: true}))
	require.True(t, addressedTo(common.StoredMessage{Kind: "channel", Target: "#agents", Nick: "peer", Text: "unrelated"}, "agent", common.CodexBinding{SubscribedChannels: []string{"#AGENTS"}}))
	require.False(t, addressedTo(common.StoredMessage{Kind: "channel", Target: "#other", Nick: "peer", Text: "unrelated"}, "agent", common.CodexBinding{SubscribedChannels: []string{"#agents"}}))
	require.False(t, addressedTo(common.StoredMessage{Kind: "direct", Target: "agent", Nick: "agent"}, "agent", common.CodexBinding{MentionCurrentIdentity: true}))
}

func TestRemovedMonitorRecordsNeverDispatch(t *testing.T) {
	message := common.StoredMessage{EventID: "legacy", Source: "monitor:old", Kind: "monitor", Target: "agent", Nick: "peer", Text: "old output"}
	require.False(t, addressedTo(message, "agent", common.CodexBinding{MentionCurrentIdentity: true, SubscribedChannels: []string{"agent"}}))
	h := &controlHandler{}
	for _, op := range []string{"monitor.start", "monitor.list", "monitor.stop"} {
		reply := h.Handle(daemon.ControlRequest{Op: op})
		require.False(t, reply.OK)
		require.Equal(t, daemon.CodeBadRequest, reply.Code)
	}
}

func TestCodexCursorWriteCannotOverwriteReboundOrigin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "binding.json")
	old := common.CodexBinding{BindingID: "old", SessionID: "session", Endpoint: "unix:///tmp/app.sock", ThreadID: "thread", Enabled: true, Offset: 10}
	newBinding := old
	newBinding.BindingID = "new"
	newBinding.Offset = 100
	require.NoError(t, dal.SaveCodexBinding(path, newBinding))
	h := &controlHandler{m: &ChatService{Paths: dal.Paths{CodexBinding: path}}}
	require.Error(t, h.saveCodexOffset(old, 50))
	got, err := dal.LoadCodexBinding(path)
	require.NoError(t, err)
	require.Equal(t, int64(100), got.Offset)
	require.Equal(t, "new", got.BindingID)
}

func TestUnbindRetainsPendingCursorAndBindingIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "binding.json")
	want := common.CodexBinding{BindingID: "binding", SessionID: "session", Endpoint: "unix:///tmp/app.sock", ThreadID: "thread", Enabled: true, Offset: 22}
	require.NoError(t, dal.SaveCodexBinding(path, want))
	h := &controlHandler{m: &ChatService{Paths: dal.Paths{CodexBinding: path}}}
	resp := h.unbindCodex()
	require.True(t, resp.OK)
	require.NotNil(t, resp.Codex)
	require.False(t, resp.Codex.Enabled)
	require.Equal(t, int64(22), resp.Codex.Offset)
	require.Equal(t, want.BindingID, resp.Codex.BindingID)
}

func TestCodexBindPersistsExplicitGroupRoutingOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "binding.json")
	h := &controlHandler{m: &ChatService{State: common.ChatSession{SessionID: "session"}, Paths: dal.Paths{CodexBinding: path}}}
	resp := h.bindCodex("unix:///tmp/app.sock", "thread", false, true, []string{"#Agents", "#agents"})
	require.True(t, resp.OK)
	require.True(t, resp.Codex.MentionCurrentIdentity)
	require.Equal(t, []string{"#agents"}, resp.Codex.SubscribedChannels)

	invalid := h.bindCodex("unix:///tmp/app.sock", "thread", false, false, []string{"agents"})
	require.False(t, invalid.OK)
	require.Contains(t, invalid.Error, "must start with #")
}

func TestCodexStatusExposesBlockedQueueAndClearsAfterRepair(t *testing.T) {
	dir := t.TempDir()
	messages := filepath.Join(dir, "messages.jsonl")
	bindingPath := filepath.Join(dir, "binding.json")
	prefix := common.StoredMessage{EventID: "acked-prefix", Source: "irc", Kind: "direct", Nick: "peer", Text: "already acknowledged"}
	require.NoError(t, dal.AppendMessage(messages, prefix))
	prefixBytes, err := os.ReadFile(messages)
	require.NoError(t, err)
	badOffset := int64(len(prefixBytes))
	f, err := os.OpenFile(messages, os.O_APPEND|os.O_WRONLY, 0600)
	require.NoError(t, err)
	_, err = f.Write([]byte("{malformed-json}\n"))
	require.NoError(t, err)
	require.NoError(t, f.Close())
	require.NoError(t, dal.AppendMessage(messages, common.StoredMessage{EventID: "after-bad-line", Source: "irc", Kind: "direct", Nick: "peer", Text: "later"}))
	binding := common.CodexBinding{BindingID: "binding", SessionID: "session", Endpoint: "unix:///tmp/app.sock", ThreadID: "thread", Enabled: true, Offset: badOffset}
	require.NoError(t, dal.SaveCodexBinding(bindingPath, binding))
	h := &controlHandler{m: &ChatService{Paths: dal.Paths{Messages: messages, CodexBinding: bindingPath}}}

	blocked := h.Handle(daemon.ControlRequest{Op: "codex.status"})
	require.True(t, blocked.OK)
	require.Equal(t, badOffset, blocked.Codex.Offset)
	require.NotNil(t, blocked.CodexQueue)
	require.True(t, blocked.CodexQueue.Blocked)
	require.Equal(t, 0, blocked.CodexQueue.PendingRecords)
	require.Greater(t, blocked.CodexQueue.PendingBytes, int64(0))
	require.Equal(t, &badOffset, blocked.CodexQueue.ErrorOffset)
	require.Contains(t, blocked.CodexQueue.Error, "invalid message JSONL record")
	wire, err := json.Marshal(blocked)
	require.NoError(t, err)
	require.Contains(t, string(wire), `"codex_queue"`)
	require.Contains(t, string(wire), `"error_offset":`+jsonNumber(badOffset))

	// Repair the malformed line while preserving the queue cursor at its start.
	require.NoError(t, os.WriteFile(messages, prefixBytes, 0600))
	require.NoError(t, dal.AppendMessage(messages, common.StoredMessage{EventID: "repaired", Source: "irc", Kind: "direct", Nick: "peer", Text: "repaired record"}))
	require.NoError(t, dal.AppendMessage(messages, common.StoredMessage{EventID: "after-bad-line", Source: "irc", Kind: "direct", Nick: "peer", Text: "later"}))
	repaired := h.Handle(daemon.ControlRequest{Op: "codex.status"})
	require.True(t, repaired.OK)
	require.False(t, repaired.CodexQueue.Blocked)
	require.Empty(t, repaired.CodexQueue.Error)
	require.Nil(t, repaired.CodexQueue.ErrorOffset)
	require.Equal(t, 2, repaired.CodexQueue.PendingRecords)
	require.Equal(t, badOffset, repaired.Codex.Offset)
	stored, err := dal.LoadCodexBinding(bindingPath)
	require.NoError(t, err)
	require.Equal(t, badOffset, stored.Offset)
}

func TestCodexStatusExposesUnavailableDeliveryAndClearsOnAck(t *testing.T) {
	dir := t.TempDir()
	messages := filepath.Join(dir, "messages.jsonl")
	bindingPath := filepath.Join(dir, "binding.json")
	require.NoError(t, dal.AppendMessage(messages, common.StoredMessage{EventID: "pending", Source: "irc", Kind: "direct", Nick: "peer", Text: "still pending"}))
	binding := common.CodexBinding{BindingID: "binding", SessionID: "session", Endpoint: "unix:///tmp/app.sock", ThreadID: "thread", Enabled: true}
	require.NoError(t, dal.SaveCodexBinding(bindingPath, binding))
	h := &controlHandler{m: &ChatService{Paths: dal.Paths{Messages: messages, CodexBinding: bindingPath}}}

	require.NoError(t, h.saveCodexDeliveryFailure(binding, &rpcError{Method: "thread/read", Code: -32600, Message: "thread not loaded: thread"}))
	blocked := h.Handle(daemon.ControlRequest{Op: "codex.status"})
	require.True(t, blocked.OK)
	require.True(t, blocked.Codex.DeliveryBlocked)
	require.Equal(t, "not_loaded", blocked.Codex.DeliveryState)
	require.Contains(t, blocked.Codex.DeliveryError, "thread/read: thread not loaded: thread")
	require.Equal(t, int64(0), blocked.Codex.Offset)
	require.Equal(t, 1, blocked.CodexQueue.PendingRecords)

	data, err := os.ReadFile(messages)
	require.NoError(t, err)
	require.NoError(t, h.saveCodexProgress(binding, int64(len(data)), true))
	recovered := h.Handle(daemon.ControlRequest{Op: "codex.status"})
	require.True(t, recovered.OK)
	require.False(t, recovered.Codex.DeliveryBlocked)
	require.Equal(t, "accepted", recovered.Codex.DeliveryState)
	require.Empty(t, recovered.Codex.DeliveryError)
	require.Equal(t, 0, recovered.CodexQueue.PendingRecords)
}

func TestUnclassifiedSteerFailureIsVisibleAndSuppressesLaterStart(t *testing.T) {
	binding := common.CodexBinding{BindingID: "binding", SessionID: "session", Endpoint: "unix:///tmp/app.sock", ThreadID: "thread", Enabled: true}
	h := &controlHandler{m: &ChatService{Paths: dal.Paths{CodexBinding: filepath.Join(t.TempDir(), "binding.json")}}}
	require.NoError(t, dal.SaveCodexBinding(h.m.Paths.CodexBinding, binding))
	require.NoError(t, h.saveCodexDeliveryFailure(binding, &rpcError{Method: "turn/steer", Code: -32603, Message: "injected arbitrary protocol failure"}))
	got, err := dal.LoadCodexBinding(h.m.Paths.CodexBinding)
	require.NoError(t, err)
	require.True(t, got.DeliveryBlocked)
	require.True(t, got.NoStartAfterSteerFailure)
	require.Equal(t, "steer_error_blocked", got.DeliveryState)
	require.False(t, isIdleSteerRace(&rpcError{Method: "turn/steer", Code: -32603, Message: "no active turn to steer"}))
	require.False(t, isIdleSteerRace(&rpcError{Method: "turn/steer", Code: -32600, Message: "permission denied"}))
	require.True(t, isIdleSteerRace(&rpcError{Method: "turn/steer", Code: -32600, Message: "no active turn to steer"}))
}

func TestWorkerRetryAfterGenericSteerErrorNeverStartsAndRetainsEvent(t *testing.T) {
	reads := 0
	endpoint, calls := scriptedEndpoint(t, func(method string, _ map[string]any) map[string]any {
		switch method {
		case "thread/read":
			reads++
			state := "active"
			if reads > 1 {
				state = "idle"
			}
			return map[string]any{"result": map[string]any{"thread": map[string]any{"id": "thread", "status": map[string]any{"type": state}}}}
		case "thread/turns/list":
			if reads == 1 {
				return map[string]any{"result": map[string]any{"data": []any{map[string]any{"id": "turn-active", "status": "inProgress"}}}}
			}
			return map[string]any{"result": map[string]any{"data": []any{}}}
		case "turn/steer":
			return map[string]any{"error": map[string]any{"code": -32603, "message": "injected arbitrary protocol failure"}}
		case "turn/start":
			return map[string]any{"result": map[string]any{"turn": map[string]any{"id": "unexpected-turn"}}}
		}
		return map[string]any{"result": map[string]any{}}
	})
	dir := t.TempDir()
	messages := filepath.Join(dir, "messages.jsonl")
	bindingPath := filepath.Join(dir, "binding.json")
	require.NoError(t, dal.AppendMessage(messages, common.StoredMessage{EventID: "event", Source: "irc", Kind: "direct", Nick: "peer", Target: "agent", Text: "pending"}))
	binding := common.CodexBinding{BindingID: "binding", SessionID: "session", Endpoint: endpoint, ThreadID: "thread", Enabled: true}
	require.NoError(t, dal.SaveCodexBinding(bindingPath, binding))
	m := &ChatService{State: common.ChatSession{SessionID: "session", Nick: "agent"}, Paths: dal.Paths{Messages: messages, CodexBinding: bindingPath}}
	h := &controlHandler{m: m}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		m.runCodexWorker(ctx, h, slog.New(slog.NewTextHandler(io.Discard, nil)), wake)
		close(done)
	}()
	wake <- struct{}{}
	deadline := time.Now().Add(6 * time.Second)
	var got common.CodexBinding
	for time.Now().Before(deadline) {
		got, _ = dal.LoadCodexBinding(bindingPath)
		if got.DeliveryState == "steer_error_blocked" && strings.Contains(got.DeliveryError, "idle after an unclassified turn/steer error") && count(calls(), "thread/read") >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Codex worker did not stop")
	}
	require.True(t, got.DeliveryBlocked)
	require.True(t, got.NoStartAfterSteerFailure)
	require.Equal(t, int64(0), got.Offset)
	require.GreaterOrEqual(t, count(calls(), "turn/steer"), 1)
	require.GreaterOrEqual(t, count(calls(), "thread/read"), 2, "the worker must reach the later retry attempt")
	require.NotContains(t, calls(), "turn/start")
	status := h.Handle(daemon.ControlRequest{Op: "codex.status"})
	require.Equal(t, 1, status.CodexQueue.PendingRecords)
	require.Equal(t, int64(0), status.Codex.Offset)
}

func TestWorkerUnbindWhileThreadLockContendedRetainsMessageAndFreshBindingDelivers(t *testing.T) {
	var rpcMu sync.Mutex
	var rpcMethods []string
	var rpcBodies []string
	endpoint, _ := scriptedEndpoint(t, func(method string, params map[string]any) map[string]any {
		rpcMu.Lock()
		rpcMethods = append(rpcMethods, method)
		if method == "turn/start" {
			body, _ := json.Marshal(params)
			rpcBodies = append(rpcBodies, string(body))
		}
		rpcMu.Unlock()
		switch method {
		case "thread/read":
			return map[string]any{"result": map[string]any{"thread": map[string]any{"id": "thread", "status": map[string]any{"type": "idle"}}}}
		case "thread/turns/list":
			return map[string]any{"result": map[string]any{"data": []any{}}}
		case "turn/start":
			return map[string]any{"result": map[string]any{"turn": map[string]any{"id": "fresh-turn"}}}
		}
		return map[string]any{"result": map[string]any{}}
	})
	dir := t.TempDir()
	messages := filepath.Join(dir, "messages.jsonl")
	bindingPath := filepath.Join(dir, "binding.json")
	old := common.CodexBinding{BindingID: "old-generation", SessionID: "session", Endpoint: endpoint, ThreadID: "thread", Enabled: true}
	oldEvent := common.StoredMessage{EventID: "old-irc-event", Source: "irc", Kind: "direct", Target: "peer", Nick: "peer", Text: "retain after unbind"}
	require.NoError(t, dal.AppendMessage(messages, oldEvent))
	require.NoError(t, dal.SaveCodexBinding(bindingPath, old))
	contended := make(chan struct{})
	var contendedOnce sync.Once
	m := &ChatService{
		State:              common.ChatSession{SessionID: "session", Nick: "agent"},
		Paths:              dal.Paths{Messages: messages, CodexBinding: bindingPath},
		codexLockContended: func() { contendedOnce.Do(func() { close(contended) }) },
	}
	h := &controlHandler{m: m}
	releaseThreadLock, err := lockThread(old)
	require.NoError(t, err)
	defer releaseThreadLock()

	logMessages := make(chan string, 32)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		m.runCodexWorker(ctx, h, slog.New(slog.NewTextHandler(channelLogWriter{messages: logMessages}, nil)), wake)
		close(done)
	}()
	wake <- struct{}{}
	select {
	case <-contended:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach the held per-thread lock")
	}

	unbound := h.unbindCodex()
	require.True(t, unbound.OK, unbound.Error)
	fresh := h.bindCodex(endpoint, "thread", false, false, nil)
	require.True(t, fresh.OK, fresh.Error)
	require.NotEqual(t, old.BindingID, fresh.Codex.BindingID, "each bind starts a new delivery generation")
	releaseThreadLock()
	select {
	case message := <-logMessages:
		for !strings.Contains(message, "binding changed or disabled while waiting for thread lock") {
			select {
			case message = <-logMessages:
			case <-time.After(3 * time.Second):
				t.Fatal("worker did not report cancellation after revalidating the disabled binding")
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not cancel dispatch after unbind")
	}
	rpcMu.Lock()
	firstPhaseRPCs := append([]string(nil), rpcMethods...)
	rpcMu.Unlock()
	require.NotContains(t, firstPhaseRPCs, "turn/start", "an unbound stale delivery must not start work")
	require.NotContains(t, firstPhaseRPCs, "turn/steer", "an unbound stale delivery must not steer work")
	status := h.Handle(daemon.ControlRequest{Op: "codex.status"})
	require.True(t, status.OK)
	require.True(t, status.Codex.Enabled)
	require.Equal(t, fresh.Codex.BindingID, status.Codex.BindingID)
	require.Equal(t, fresh.Codex.Offset, status.Codex.Offset)
	require.Equal(t, 0, status.CodexQueue.PendingRecords)

	freshEvent := common.StoredMessage{EventID: "fresh-binding-event", Source: "irc", Kind: "direct", Target: "peer", Nick: "peer", Text: "deliver to fresh binding"}
	require.NoError(t, dal.AppendMessage(messages, freshEvent))
	fileInfo, err := os.Stat(messages)
	require.NoError(t, err)
	wake <- struct{}{}
	deadline := time.After(4 * time.Second)
	for {
		rpcMu.Lock()
		started := count(rpcMethods, "turn/start") > 0
		rpcMu.Unlock()
		if started {
			break
		}
		select {
		case <-deadline:
			t.Fatal("fresh enabled binding did not dispatch its pending event")
		case <-time.After(10 * time.Millisecond):
		}
	}
	require.Eventually(t, func() bool {
		got, loadErr := dal.LoadCodexBinding(bindingPath)
		return loadErr == nil && got.BindingID == fresh.Codex.BindingID && got.Offset == fileInfo.Size()
	}, 2*time.Second, 10*time.Millisecond, "fresh binding ACK should advance its own cursor")
	rpcMu.Lock()
	joinedBodies := strings.Join(rpcBodies, "\n")
	rpcMu.Unlock()
	require.Contains(t, joinedBodies, "deliver to fresh binding")
	require.NotContains(t, joinedBodies, "retain after unbind", "a fresh binding must skip messages received before binding")
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Codex worker did not stop")
	}
}

func TestWorkerUnbindDuringThreadInspectionCancelsBeforeDispatchRPC(t *testing.T) {
	threadRead := make(chan struct{})
	threadReadOnce := sync.Once{}
	releaseRead := make(chan struct{})
	releaseReadOnce := sync.Once{}
	defer releaseReadOnce.Do(func() { close(releaseRead) })
	var rpcMu sync.Mutex
	var rpcMethods []string
	endpoint, _ := scriptedEndpoint(t, func(method string, _ map[string]any) map[string]any {
		rpcMu.Lock()
		rpcMethods = append(rpcMethods, method)
		rpcMu.Unlock()
		switch method {
		case "thread/read":
			threadReadOnce.Do(func() { close(threadRead) })
			<-releaseRead
			return map[string]any{"result": map[string]any{"thread": map[string]any{"id": "thread", "status": map[string]any{"type": "idle"}}}}
		case "thread/turns/list":
			return map[string]any{"result": map[string]any{"data": []any{}}}
		}
		return map[string]any{"result": map[string]any{}}
	})
	dir := t.TempDir()
	messages := filepath.Join(dir, "messages.jsonl")
	bindingPath := filepath.Join(dir, "binding.json")
	require.NoError(t, dal.AppendMessage(messages, common.StoredMessage{EventID: "pending-before-send", Source: "irc", Kind: "direct", Target: "peer", Nick: "peer", Text: "retain until dispatch ACK"}))
	binding := common.CodexBinding{BindingID: "generation", SessionID: "session", Endpoint: endpoint, ThreadID: "thread", Enabled: true}
	require.NoError(t, dal.SaveCodexBinding(bindingPath, binding))
	m := &ChatService{State: common.ChatSession{SessionID: "session", Nick: "agent"}, Paths: dal.Paths{Messages: messages, CodexBinding: bindingPath}}
	h := &controlHandler{m: m}
	logMessages := make(chan string, 32)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		m.runCodexWorker(ctx, h, slog.New(slog.NewTextHandler(channelLogWriter{messages: logMessages}, nil)), wake)
		close(done)
	}()
	wake <- struct{}{}
	select {
	case <-threadRead:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach app-server thread inspection")
	}
	unbound := h.unbindCodex()
	require.True(t, unbound.OK, unbound.Error)
	releaseReadOnce.Do(func() { close(releaseRead) })
	select {
	case message := <-logMessages:
		for !strings.Contains(message, "binding changed or was disabled before dispatch") {
			select {
			case message = <-logMessages:
			case <-time.After(3 * time.Second):
				t.Fatal("worker did not cancel at the final dispatch authorization")
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not cancel before the dispatch RPC")
	}
	rpcMu.Lock()
	methods := append([]string(nil), rpcMethods...)
	rpcMu.Unlock()
	require.Contains(t, methods, "thread/read")
	require.Contains(t, methods, "thread/turns/list")
	require.NotContains(t, methods, "turn/start")
	require.NotContains(t, methods, "turn/steer")
	status := h.Handle(daemon.ControlRequest{Op: "codex.status"})
	require.True(t, status.OK)
	require.False(t, status.Codex.Enabled)
	require.Zero(t, status.Codex.Offset)
	require.Equal(t, 1, status.CodexQueue.PendingRecords)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Codex worker did not stop")
	}
}

func jsonNumber(value int64) string { return fmt.Sprint(value) }

func TestCodexBindReportsUnavailableBeforeAnyMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "binding.json")
	h := &controlHandler{m: &ChatService{State: common.ChatSession{SessionID: "session"}, Paths: dal.Paths{CodexBinding: path}}}
	resp := h.bindCodex("unix:///tmp/chatta-absent-immediate.sock", "thread", false, false, nil)
	require.True(t, resp.OK)
	require.True(t, resp.Codex.DeliveryBlocked)
	require.Equal(t, "unavailable", resp.Codex.DeliveryState)
	require.NotEmpty(t, resp.Codex.DeliveryError)
	previous, err := os.ReadFile(path)
	require.NoError(t, err)
	strict := h.bindCodexChecked("unix:///tmp/chatta-absent-immediate.sock", "current-thread", false, false, nil, true)
	require.False(t, strict.OK)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, previous, after)
	require.NoError(t, h.saveCodexOffset(*resp.Codex, 10))
	got, err := dal.LoadCodexBinding(path)
	require.NoError(t, err)
	require.True(t, got.DeliveryBlocked, "filtered records cannot conceal the target failure")
}

func TestCodexStartupReusePreservesPendingCursorAndRejectsRouteChange(t *testing.T) {
	endpoint, _ := scriptedEndpoint(t, func(method string, params map[string]any) map[string]any {
		if method == "thread/read" {
			return map[string]any{"result": map[string]any{"thread": map[string]any{"id": "thread", "status": map[string]any{"type": "idle"}}}}
		}
		return map[string]any{"result": map[string]any{}}
	})
	dir := t.TempDir()
	h := &controlHandler{m: &ChatService{State: common.ChatSession{SessionID: "session"}, Paths: dal.Paths{CodexBinding: filepath.Join(dir, "binding.json"), Messages: filepath.Join(dir, "messages.jsonl")}}}
	request := daemon.ControlRequest{Op: "codex.bind", Endpoint: endpoint, ThreadID: "thread", RequireAvailable: true, ReuseExisting: true, SubscribedChannels: []string{"#agents"}}
	first := h.Handle(request)
	require.True(t, first.OK, first.Error)
	require.NoError(t, dal.AppendMessage(h.m.Paths.Messages, common.StoredMessage{EventID: "pending", Kind: "direct", Nick: "peer", Target: "peer", Text: "not yet processed"}))
	second := h.Handle(request)
	require.True(t, second.OK, second.Error)
	require.Equal(t, first.Codex.BindingID, second.Codex.BindingID)
	require.Equal(t, first.Codex.Offset, second.Codex.Offset)
	require.Equal(t, 1, second.CodexQueue.PendingRecords)
	request.SubscribedChannels = nil
	conflict := h.Handle(request)
	require.False(t, conflict.OK)
	require.Contains(t, conflict.Error, "binding unchanged")
	current, err := dal.LoadCodexBinding(h.m.Paths.CodexBinding)
	require.NoError(t, err)
	require.Equal(t, *first.Codex, current)
}

func TestCodexEventIdentifiesRecipientRatherThanDirectConversationPeer(t *testing.T) {
	direct := common.StoredMessage{EventID: "dm", Source: "irc", Nick: "sender", Target: "sender", Kind: "direct", Text: "question"}
	body := formatCodexEvent(direct, "receiver")
	require.Contains(t, body, "from=sender\ntarget=receiver\nkind=direct")
	require.Equal(t, "sender", direct.Target, "do not rewrite the original inbox record")
	direct.Kind, direct.Target = "channel", "#agents"
	require.Contains(t, formatCodexEvent(direct, "receiver"), "target=#agents\nkind=channel")
}
