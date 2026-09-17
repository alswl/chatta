//go:build darwin || linux

package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

const (
	connectDeadline  = 5 * time.Second
	responseDeadline = 10 * time.Second
)

// Request connects to the control socket at path, sends req, and returns
// its response. A refused or missing socket, or a deadline exceeded before
// a full response line arrives, maps to CodeNotConnected. Cancelling ctx
// abandons the round trip: the deadlines below bound a peer that is merely
// slow, and closing the socket is what unblocks a read already in progress.
func Request(ctx context.Context, path string, req ControlRequest) (ControlResponse, error) {
	dialer := net.Dialer{Timeout: connectDeadline}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ControlResponse{}, &Error{Code: CodeTimeout, Msg: fmt.Sprintf("control request cancelled: %v", ctxErr)}
		}
		return ControlResponse{}, &Error{Code: CodeNotConnected, Msg: fmt.Sprintf("control socket unreachable: %v", err)}
	}
	defer func() { _ = conn.Close() }()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	b, err := json.Marshal(req)
	if err != nil {
		return ControlResponse{}, err
	}
	b = append(b, '\n')
	if err := conn.SetDeadline(time.Now().Add(responseDeadline)); err != nil {
		return ControlResponse{}, err
	}
	if _, err := conn.Write(b); err != nil {
		return ControlResponse{}, &Error{Code: CodeNotConnected, Msg: fmt.Sprintf("control socket write failed: %v", err)}
	}
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 4096), 1<<20)
	if !scanner.Scan() {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ControlResponse{}, &Error{Code: CodeTimeout, Msg: fmt.Sprintf("control request cancelled: %v", ctxErr)}
		}
		if err := scanner.Err(); err != nil {
			return ControlResponse{}, &Error{Code: CodeTimeout, Msg: fmt.Sprintf("control socket response timed out: %v", err)}
		}
		return ControlResponse{}, &Error{Code: CodeTimeout, Msg: "control socket closed without a response"}
	}
	var resp ControlResponse
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return ControlResponse{}, fmt.Errorf("malformed control response: %w", err)
	}
	return resp, nil
}

// Error is a client-side transport failure (as opposed to an ok:false
// ControlResponse, which is a successful round trip carrying an
// application-level failure).
type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string { return e.Msg }
