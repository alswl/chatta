//go:build darwin || linux

package daemon

import (
	"bufio"
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
// a full response line arrives, maps to CodeNotConnected.
func Request(path string, req ControlRequest) (ControlResponse, error) {
	conn, err := net.DialTimeout("unix", path, connectDeadline)
	if err != nil {
		return ControlResponse{}, &Error{Code: CodeNotConnected, Msg: fmt.Sprintf("control socket unreachable: %v", err)}
	}
	defer func() { _ = conn.Close() }()

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
