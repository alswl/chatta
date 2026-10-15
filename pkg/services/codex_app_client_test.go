package services

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestDeliverInitializesAndStartsExactIdleThread(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "ca")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "a.sock")
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)
	defer listener.Close()
	methods := make(chan string, 8)
	tracePath := filepath.Join(dir, "rpc.jsonl")
	upgrader := websocket.Upgrader{}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var request map[string]any
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			method, _ := request["method"].(string)
			methods <- method
			id, hasID := request["id"]
			if !hasID {
				continue
			}
			result := map[string]any{}
			switch method {
			case "thread/read":
				result = map[string]any{"thread": map[string]any{"id": "thread-1", "status": map[string]any{"type": "idle"}}}
			case "thread/turns/list":
				result = map[string]any{"data": []any{}}
			case "turn/start":
				result = map[string]any{"turn": map[string]any{"id": "turn-1", "status": "inProgress"}}
			}
			if err := conn.WriteJSON(map[string]any{"id": id, "result": result}); err != nil {
				return
			}
			if method == "turn/start" {
				if err := conn.WriteJSON(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": "thread-1", "turnId": "turn-1", "turn": map[string]any{"id": "turn-1", "status": "completed"}}}); err != nil {
					return
				}
			}
		}
	})}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	got, err := (codexAppClient{Endpoint: "unix://" + path, ThreadID: "thread-1", TracePath: tracePath}).Deliver(context.Background(), "event_id=e1 source=irc")
	require.NoError(t, err)
	require.Equal(t, codexDeliveryAck{Method: "turn/start", ThreadID: "thread-1", TurnID: "turn-1"}, got)
	var seen []string
	for len(methods) > 0 {
		seen = append(seen, <-methods)
	}
	require.Equal(t, []string{"initialize", "initialized", "thread/read", "thread/turns/list", "turn/start"}, seen)
	deadline := time.Now().Add(time.Second)
	var trace []byte
	for time.Now().Before(deadline) {
		trace, _ = os.ReadFile(tracePath)
		if strings.Contains(string(trace), "turn/completed") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Contains(t, string(trace), "turn/completed")
	require.Contains(t, string(trace), "\"turnId\":\"turn-1\"")
}

func TestRPCMessageCanDecodeWithoutJSONRPCHeader(t *testing.T) {
	var got rpcMessage
	require.NoError(t, json.Unmarshal([]byte(`{"id":2,"result":{"ok":true}}`), &got))
	require.Equal(t, "2", string(got.ID))
}

func TestDeliverReportsBoundThreadNotLoadedWithoutChangingTarget(t *testing.T) {
	endpoint, calls := scriptedEndpoint(t, func(method string, _ map[string]any) map[string]any {
		if method == "thread/read" {
			return map[string]any{"error": map[string]any{"code": -32600, "message": "thread not loaded: bound-thread"}}
		}
		return map[string]any{"result": map[string]any{}}
	})
	_, err := (codexAppClient{Endpoint: endpoint, ThreadID: "bound-thread"}).Deliver(context.Background(), "event")
	var rpcErr *rpcError
	require.ErrorAs(t, err, &rpcErr)
	require.Equal(t, "thread/read", rpcErr.Method)
	require.Equal(t, -32600, rpcErr.Code)
	require.Contains(t, rpcErr.Message, "thread not loaded: bound-thread")
	require.Equal(t, []string{"initialize", "initialized", "thread/read"}, calls())
	require.NotContains(t, calls(), "thread/resume")
	require.NotContains(t, calls(), "thread/start")
}

func TestDeliverSteersObservedTurnAndRetriesOnlyStaleIdleRace(t *testing.T) {
	t.Run("active", func(t *testing.T) {
		endpoint, calls := scriptedEndpoint(t, func(method string, params map[string]any) map[string]any {
			switch method {
			case "thread/read":
				return map[string]any{"result": map[string]any{"thread": map[string]any{"id": "thread-1", "status": map[string]any{"type": "active"}}}}
			case "thread/turns/list":
				return map[string]any{"result": map[string]any{"data": []any{map[string]any{"id": "turn-active", "status": "inProgress"}}}}
			case "turn/steer":
				require.Equal(t, "turn-active", params["expectedTurnId"])
				return map[string]any{"result": map[string]any{"turnId": "turn-next"}}
			}
			return map[string]any{"result": map[string]any{}}
		})
		got, err := (codexAppClient{Endpoint: endpoint, ThreadID: "thread-1"}).Deliver(context.Background(), "event")
		require.NoError(t, err)
		require.Equal(t, "turn/steer", got.Method)
		require.Equal(t, "turn-next", got.TurnID)
		require.Contains(t, calls(), "turn/steer")
		require.NotContains(t, calls(), "turn/start")
	})
	t.Run("stale steer becomes idle", func(t *testing.T) {
		reads := 0
		endpoint, calls := scriptedEndpoint(t, func(method string, _ map[string]any) map[string]any {
			switch method {
			case "thread/read":
				reads++
				state := "active"
				if reads > 1 {
					state = "idle"
				}
				return map[string]any{"result": map[string]any{"thread": map[string]any{"id": "thread-1", "status": map[string]any{"type": state}}}}
			case "thread/turns/list":
				if reads == 1 {
					return map[string]any{"result": map[string]any{"data": []any{map[string]any{"id": "turn-active", "status": "inProgress"}}}}
				}
				return map[string]any{"result": map[string]any{"data": []any{}}}
			case "turn/steer":
				return map[string]any{"error": map[string]any{"code": -32600, "message": "no active turn to steer"}}
			case "turn/start":
				return map[string]any{"result": map[string]any{"turn": map[string]any{"id": "turn-after-race"}}}
			}
			return map[string]any{"result": map[string]any{}}
		})
		got, err := (codexAppClient{Endpoint: endpoint, ThreadID: "thread-1"}).Deliver(context.Background(), "event")
		require.NoError(t, err)
		require.Equal(t, "turn/start", got.Method)
		require.Equal(t, "turn-after-race", got.TurnID)
		gotCalls := calls()
		require.Equal(t, 1, count(gotCalls, "turn/start"))
		require.Equal(t, 2, count(gotCalls, "thread/read"))
	})
	t.Run("arbitrary steer error never starts", func(t *testing.T) {
		endpoint, calls := scriptedEndpoint(t, func(method string, _ map[string]any) map[string]any {
			switch method {
			case "thread/read":
				return map[string]any{"result": map[string]any{"thread": map[string]any{"id": "thread-1", "status": map[string]any{"type": "active"}}}}
			case "thread/turns/list":
				return map[string]any{"result": map[string]any{"data": []any{map[string]any{"id": "turn-active", "status": "inProgress"}}}}
			case "turn/steer":
				return map[string]any{"error": map[string]any{"code": -32603, "message": "injected arbitrary protocol failure"}}
			}
			return map[string]any{"result": map[string]any{}}
		})
		_, err := (codexAppClient{Endpoint: endpoint, ThreadID: "thread-1"}).Deliver(context.Background(), "event")
		require.ErrorContains(t, err, "injected arbitrary protocol failure")
		require.NotContains(t, calls(), "turn/start")
	})
	t.Run("later idle retry after unclassified steer error remains pending", func(t *testing.T) {
		endpoint, calls := scriptedEndpoint(t, func(method string, _ map[string]any) map[string]any {
			switch method {
			case "thread/read":
				return map[string]any{"result": map[string]any{"thread": map[string]any{"id": "thread-1", "status": map[string]any{"type": "idle"}}}}
			case "thread/turns/list":
				return map[string]any{"result": map[string]any{"data": []any{}}}
			}
			return map[string]any{"result": map[string]any{}}
		})
		_, err := (codexAppClient{Endpoint: endpoint, ThreadID: "thread-1", NoStartAfterSteerFailure: true}).Deliver(context.Background(), "event")
		require.ErrorContains(t, err, "without turn/start")
		require.NotContains(t, calls(), "turn/start")
	})
}

func scriptedEndpoint(t *testing.T, respond func(string, map[string]any) map[string]any) (string, func() []string) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "cs")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "s.sock")
	listener, err := net.Listen("unix", path)
	require.NoError(t, err)
	var mu sync.Mutex
	var seen []string
	upgrader := websocket.Upgrader{}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var call map[string]any
			if conn.ReadJSON(&call) != nil {
				return
			}
			method, _ := call["method"].(string)
			params, _ := call["params"].(map[string]any)
			mu.Lock()
			seen = append(seen, method)
			mu.Unlock()
			if id, ok := call["id"]; ok {
				reply := respond(method, params)
				reply["id"] = id
				if conn.WriteJSON(reply) != nil {
					return
				}
			}
		}
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = listener.Close()
	})
	get := func() []string { mu.Lock(); defer mu.Unlock(); return append([]string(nil), seen...) }
	return "unix://" + path, get
}

func count(values []string, want string) int {
	n := 0
	for _, v := range values {
		if v == want {
			n++
		}
	}
	return n
}
