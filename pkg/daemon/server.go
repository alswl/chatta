//go:build darwin || linux

package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
)

// Handler dispatches one ControlRequest to a response. Implemented by the
// supervisor.
type Handler interface {
	Handle(req ControlRequest) ControlResponse
}

// Server listens on a Unix domain socket and serves one request per
// connection until its context is cancelled.
type Server struct {
	ln net.Listener
}

// Listen binds the control socket at path, mode 0600, unlinking a stale
// socket file first.
func Listen(path string) (*Server, error) {
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	return &Server{ln: ln}, nil
}

// Serve accepts connections and dispatches each request to handler until
// ctx is cancelled, at which point it closes the listener and returns.
func (s *Server) Serve(ctx context.Context, handler Handler) error {
	go func() {
		<-ctx.Done()
		_ = s.ln.Close()
	}()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go s.serveOne(conn, handler)
	}
}

func (s *Server) serveOne(conn net.Conn, handler Handler) {
	defer func() { _ = conn.Close() }()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 4096), 1<<20)
	if !scanner.Scan() {
		return
	}
	var req ControlRequest
	var resp ControlResponse
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
		resp = ControlResponse{OK: false, Code: CodeBadRequest, Error: "malformed request: " + err.Error()}
	} else {
		resp = handler.Handle(req)
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return
	}
	b = append(b, '\n')
	_, _ = conn.Write(b)
}

// Close closes the listener.
func (s *Server) Close() error { return s.ln.Close() }
