package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type codexAppClient struct {
	Endpoint                 string
	ThreadID                 string
	Timeout                  time.Duration
	TracePath                string
	NoStartAfterSteerFailure bool
	AuthorizeDispatch        func(send func() error) error
}

type codexDeliveryAck struct{ Method, ThreadID, TurnID string }

type rpcError struct {
	Method  string
	Code    int
	Message string
}

func (e *rpcError) Error() string { return fmt.Sprintf("%s: %s", e.Method, e.Message) }

type threadUnavailableError struct {
	State string
	Err   error
}

func (e *threadUnavailableError) Error() string { return e.Err.Error() }
func (e *threadUnavailableError) Unwrap() error { return e.Err }

type rpcMessage struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c codexAppClient) connect(ctx context.Context) (rpc, error) {
	if !strings.HasPrefix(c.Endpoint, "unix:///") || c.ThreadID == "" {
		return rpc{}, errors.New("codex requires a local Unix endpoint and explicit thread ID")
	}
	socketPath := strings.TrimPrefix(c.Endpoint, "unix://")
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second, NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "unix", socketPath)
	}}
	conn, _, err := dialer.DialContext(ctx, "ws://localhost", nil)
	if err != nil {
		return rpc{}, fmt.Errorf("connect Codex app-server: %w", err)
	}
	var trace *rpcTrace
	closed := make(chan struct{})
	closeOnce := &sync.Once{}
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-closed:
		}
	}()
	ready := false
	defer func() {
		if !ready {
			closeOnce.Do(func() { close(closed) })
			_ = conn.Close()
			if trace != nil {
				_ = trace.close()
			}
		}
	}()
	if c.TracePath != "" {
		trace, err = openRPCTrace(c.TracePath)
		if err != nil {
			return rpc{}, err
		}
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	client := rpc{conn: conn, trace: trace, timeout: timeout, closed: closed, closeOnce: closeOnce}
	if _, err = client.call("initialize", map[string]any{"clientInfo": map[string]string{"name": "chatta", "title": "Chatta", "version": "0.1.0"}}); err != nil {
		return rpc{}, err
	}
	if err = client.notify("initialized", map[string]any{}); err != nil {
		return rpc{}, err
	}
	ready = true
	return client, nil
}

func (c codexAppClient) CheckThread(ctx context.Context) error {
	client, err := c.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.close() }()
	if client.trace != nil {
		defer func() { _ = client.trace.close() }()
	}
	state, _, err := client.inspect(c.ThreadID)
	if err != nil {
		return err
	}
	thread, _ := state["thread"].(map[string]any)
	if thread == nil || thread["id"] != c.ThreadID {
		return errors.New("thread/read returned a different thread ID")
	}
	status, _ := thread["status"].(map[string]any)
	kind, _ := status["type"].(string)
	if kind != "idle" && kind != "active" {
		return &threadUnavailableError{State: kind, Err: fmt.Errorf("thread %s is %s", c.ThreadID, kind)}
	}
	return nil
}

func (c codexAppClient) Deliver(ctx context.Context, text string) (codexDeliveryAck, error) {
	client, err := c.connect(ctx)
	if err != nil {
		return codexDeliveryAck{}, err
	}
	keepConnection := false
	defer func() {
		if !keepConnection {
			_ = client.close()
			if client.trace != nil {
				_ = client.trace.close()
			}
		}
	}()
	state, turns, err := client.inspect(c.ThreadID)
	if err != nil {
		return codexDeliveryAck{}, err
	}
	thread, _ := state["thread"].(map[string]any)
	if thread == nil || thread["id"] != c.ThreadID {
		return codexDeliveryAck{}, errors.New("thread/read returned a different thread ID")
	}
	status, _ := thread["status"].(map[string]any)
	typeName, _ := status["type"].(string)
	input := []map[string]string{{"type": "text", "text": text}}
	if typeName == "idle" {
		if c.NoStartAfterSteerFailure {
			return codexDeliveryAck{}, errors.New("thread is idle after an unclassified turn/steer error; retaining event without turn/start")
		}
		result, err := client.callDispatched("turn/start", map[string]any{"threadId": c.ThreadID, "input": input}, c.AuthorizeDispatch)
		if err != nil {
			return codexDeliveryAck{}, err
		}
		turnID := getString(result, "turn", "id")
		if turnID == "" {
			return codexDeliveryAck{}, errors.New("turn/start ACK omitted turn ID")
		}
		a := codexDeliveryAck{Method: "turn/start", ThreadID: c.ThreadID, TurnID: turnID}
		keepConnection = client.keepForCompletion(ctx, a)
		return a, nil
	}
	if typeName != "active" {
		return codexDeliveryAck{}, &threadUnavailableError{State: typeName, Err: fmt.Errorf("thread %s is %s; delivery retained", c.ThreadID, typeName)}
	}
	var activeID string
	for _, raw := range turns {
		turn, _ := raw.(map[string]any)
		if turn != nil && turn["status"] == "inProgress" {
			activeID, _ = turn["id"].(string)
			break
		}
	}
	if activeID == "" {
		return codexDeliveryAck{}, errors.New("active thread has no observed in-progress turn")
	}
	result, err := client.callDispatched("turn/steer", map[string]any{"threadId": c.ThreadID, "expectedTurnId": activeID, "input": input}, c.AuthorizeDispatch)
	if err != nil {
		// Only a positively identified no-active-turn race can fall back to start.
		var steerErr *rpcError
		if !errors.As(err, &steerErr) || !isIdleSteerRace(steerErr) {
			return codexDeliveryAck{}, err
		}
		state, _, err = client.inspect(c.ThreadID)
		if err != nil {
			return codexDeliveryAck{}, err
		}
		thread, _ = state["thread"].(map[string]any)
		if thread == nil || thread["id"] != c.ThreadID {
			return codexDeliveryAck{}, errors.New("thread/read returned a different thread ID after stale steer")
		}
		status, _ = thread["status"].(map[string]any)
		typeName, _ = status["type"].(string)
		if typeName != "idle" {
			return codexDeliveryAck{}, &threadUnavailableError{State: typeName, Err: fmt.Errorf("stale steer rechecked same thread: %s", typeName)}
		}
		result, err = client.callDispatched("turn/start", map[string]any{"threadId": c.ThreadID, "input": input}, c.AuthorizeDispatch)
		if err != nil {
			return codexDeliveryAck{}, err
		}
		turnID := getString(result, "turn", "id")
		if turnID == "" {
			return codexDeliveryAck{}, errors.New("turn/start ACK omitted turn ID")
		}
		a := codexDeliveryAck{Method: "turn/start", ThreadID: c.ThreadID, TurnID: turnID}
		keepConnection = client.keepForCompletion(ctx, a)
		return a, nil
	}
	turnID, _ := result["turnId"].(string)
	if turnID == "" {
		return codexDeliveryAck{}, errors.New("turn/steer ACK omitted turn ID")
	}
	a := codexDeliveryAck{Method: "turn/steer", ThreadID: c.ThreadID, TurnID: turnID}
	keepConnection = client.keepForCompletion(ctx, a)
	return a, nil
}

func isIdleSteerRace(err *rpcError) bool {
	return err.Method == "turn/steer" && err.Code == -32600 && strings.EqualFold(strings.TrimSpace(err.Message), "no active turn to steer")
}

type rpc struct {
	closed    chan struct{}
	closeOnce *sync.Once
	conn      *websocket.Conn
	next      int
	trace     *rpcTrace
	timeout   time.Duration
}

type rpcTrace struct {
	file *os.File
	mu   sync.Mutex
}

func openRPCTrace(path string) (*rpcTrace, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	return &rpcTrace{file: f}, nil
}
func (t *rpcTrace) write(direction string, message any) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	b, err := json.Marshal(map[string]any{"time": time.Now().Format(time.RFC3339Nano), "direction": direction, "message": message})
	if err == nil {
		_, _ = t.file.Write(append(b, '\n'))
	}
}
func (t *rpcTrace) close() error {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.file.Close()
}

func (r *rpc) keepForCompletion(ctx context.Context, a codexDeliveryAck) bool {
	if r.trace == nil {
		return false
	}
	go func() {
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				_ = r.conn.Close()
			case <-done:
			}
		}()
		defer close(done)
		defer func() { _ = r.close() }()
		defer func() { _ = r.trace.close() }()
		_ = r.conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
		for {
			var message map[string]any
			if err := r.conn.ReadJSON(&message); err != nil {
				return
			}
			r.trace.write("server", message)
			if message["method"] == "turn/completed" {
				params, _ := message["params"].(map[string]any)
				if params["threadId"] == a.ThreadID && (params["turnId"] == a.TurnID || getString(params, "turn", "id") == a.TurnID) {
					return
				}
				continue
			}
			if message["method"] == "thread/status/changed" {
				params, _ := message["params"].(map[string]any)
				status, _ := params["status"].(map[string]any)
				if params["threadId"] == a.ThreadID && status["type"] == "idle" {
					result, err := r.call("thread/turns/list", map[string]any{"threadId": a.ThreadID, "limit": 3, "sortDirection": "desc", "itemsView": "notLoaded"})
					if err != nil {
						return
					}
					turns, _ := result["data"].([]any)
					for _, raw := range turns {
						turn, _ := raw.(map[string]any)
						if turn["id"] == a.TurnID && turn["status"] == "completed" {
							return
						}
					}
				}
			}
		}
	}()
	return true
}

func (r *rpc) notify(method string, params any) error {
	_ = r.conn.SetWriteDeadline(time.Now().Add(r.timeout))
	message := map[string]any{"method": method, "params": params}
	r.trace.write("client", message)
	return r.conn.WriteJSON(message)
}
func (r *rpc) call(method string, params any) (map[string]any, error) {
	return r.callDispatched(method, params, nil)
}

func (r *rpc) callDispatched(method string, params any, authorize func(send func() error) error) (map[string]any, error) {
	_ = r.conn.SetWriteDeadline(time.Now().Add(r.timeout))
	r.next++
	id := r.next
	request := map[string]any{"id": id, "method": method, "params": params}
	send := func() error {
		_ = r.conn.SetWriteDeadline(time.Now().Add(r.timeout))
		r.trace.write("client", request)
		return r.conn.WriteJSON(request)
	}
	var err error
	if authorize == nil {
		err = send()
	} else {
		err = authorize(send)
	}
	if err != nil {
		return nil, err
	}
	_ = r.conn.SetReadDeadline(time.Now().Add(r.timeout))
	for {
		var raw map[string]any
		if err := r.conn.ReadJSON(&raw); err != nil {
			return nil, err
		}
		r.trace.write("server", raw)
		encoded, _ := json.Marshal(raw)
		var msg rpcMessage
		if err := json.Unmarshal(encoded, &msg); err != nil {
			return nil, err
		}
		if len(msg.ID) == 0 || string(msg.ID) != fmt.Sprint(id) {
			continue
		}
		if msg.Error != nil {
			return nil, &rpcError{Method: method, Code: msg.Error.Code, Message: msg.Error.Message}
		}
		var result map[string]any
		if err := json.Unmarshal(msg.Result, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
}
func (r *rpc) inspect(threadID string) (map[string]any, []any, error) {
	state, err := r.call("thread/read", map[string]any{"threadId": threadID})
	if err != nil {
		return nil, nil, err
	}
	list, err := r.call("thread/turns/list", map[string]any{"threadId": threadID, "limit": 3, "sortDirection": "desc", "itemsView": "notLoaded"})
	if err != nil {
		return nil, nil, err
	}
	data, _ := list["data"].([]any)
	return state, data, nil
}
func getString(value map[string]any, keys ...string) string {
	var current any = value
	for _, key := range keys {
		object, _ := current.(map[string]any)
		if object == nil {
			return ""
		}
		current = object[key]
	}
	result, _ := current.(string)
	return result
}

func (r *rpc) close() error {
	if r.closeOnce != nil {
		r.closeOnce.Do(func() { close(r.closed) })
	}
	return r.conn.Close()
}
