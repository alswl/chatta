//go:build darwin || linux

package daemon

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alswl/chatta/pkg/common"
)

type echoHandler struct {
	resp ControlResponse
}

func (h echoHandler) Handle(_ ControlRequest) ControlResponse { return h.resp }

// shortSocketPath returns a socket path short enough to stay under the
// platform's sun_path limit (108 bytes on Linux, 104 on macOS) even when
// the calling test's name is long — t.TempDir() embeds the full test name.
func shortSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "ctl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "control.sock")
}

func startTestServer(t *testing.T, handler Handler) (path string, stop func()) {
	t.Helper()
	path = shortSocketPath(t)
	srv, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = srv.Serve(ctx, handler)
		close(done)
	}()
	return path, func() {
		cancel()
		_ = srv.Close()
		<-done
	}
}

func TestRoundTripAllOps(t *testing.T) {
	cases := []ControlRequest{
		{Op: "status"},
		{Op: "join", Target: "#agents"},
		{Op: "part", Target: "#agents", Reason: "done"},
		{Op: "names", Target: "#agents"},
		{Op: "privmsg", Target: "#agents", Text: "hello"},
		{Op: "privmsg", Target: "agent-b", Text: "hello"},
		{Op: "quit", Reason: "leaving"},
	}
	want := ControlResponse{OK: true, Members: []string{"agent-a", "@agent-b"}, Status: &common.TransportStatus{Connected: true, Registered: true, Nick: "agent-a", Channels: []string{"#agents"}, LastPong: 1757913600}}
	path, stop := startTestServer(t, echoHandler{resp: want})
	defer stop()
	for _, req := range cases {
		resp, err := Request(path, req)
		if err != nil {
			t.Fatalf("op %s: %v", req.Op, err)
		}
		if !resp.OK || len(resp.Members) != 2 || resp.Status == nil || resp.Status.Nick != "agent-a" {
			t.Fatalf("op %s: unexpected response %+v", req.Op, resp)
		}
	}
}

func TestRoundTripFailureCodes(t *testing.T) {
	codes := []string{CodeNotConnected, CodeNotRegistered, CodeNickInUse, CodeNoSuchNick, CodeNotOnChannel, CodeJoinFailed, CodeTimeout, CodeBadRequest}
	for _, code := range codes {
		path, stop := startTestServer(t, echoHandler{resp: ControlResponse{OK: false, Code: code, Error: "failure: " + code}})
		resp, err := Request(path, ControlRequest{Op: "status"})
		stop()
		if err != nil {
			t.Fatalf("code %s: unexpected transport error: %v", code, err)
		}
		if resp.OK || resp.Code != code {
			t.Fatalf("code %s: unexpected response %+v", code, resp)
		}
	}
}

func TestRequestAgainstStaleSocketFile(t *testing.T) {
	path := shortSocketPath(t)
	if err := os.WriteFile(path, []byte("not a socket"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Request(path, ControlRequest{Op: "status"})
	if err == nil {
		t.Fatal("expected error connecting to a stale non-socket file")
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeNotConnected {
		t.Fatalf("expected not_connected, got %v", err)
	}
}

func TestListenUnlinksStaleSocket(t *testing.T) {
	path := shortSocketPath(t)
	stale, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	_ = stale.Close()
	// stale.Close() already unlinks on most platforms, but simulate a
	// crash by recreating the file without unlinking through net.Listen.
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	srv, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen did not recover a stale socket file: %v", err)
	}
	_ = srv.Close()
}

func TestRequestNoServerListening(t *testing.T) {
	path := shortSocketPath(t)
	_, err := Request(path, ControlRequest{Op: "status"})
	if err == nil {
		t.Fatal("expected error when nothing is listening")
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Code != CodeNotConnected {
		t.Fatalf("expected not_connected, got %v", err)
	}
}

func TestRequestExceedsResponseDeadline(t *testing.T) {
	path := shortSocketPath(t)
	srv, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Close() }()
	go func() {
		conn, err := srv.ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(50 * time.Millisecond)
	}()
	// Use a short-lived override by connecting directly and relying on the
	// server never writing a response; the client's own responseDeadline
	// governs how long Request blocks.
	_, err = Request(path, ControlRequest{Op: "status"})
	if err == nil {
		t.Fatal("expected a timeout when the server never responds")
	}
}
