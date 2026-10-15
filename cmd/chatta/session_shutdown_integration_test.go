//go:build darwin || linux

package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func runSessionCLI(t *testing.T, binary, ownerPID string, args ...string) error {
	t.Helper()
	out, err := runSessionCLIOutput(t, binary, ownerPID, args...)
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runSessionCLIOutput(t *testing.T, binary, ownerPID string, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	return runSessionCLIContextOutput(t, ctx, binary, ownerPID, args...)
}

func runSessionCLIContextOutput(t *testing.T, ctx context.Context, binary, ownerPID string, args ...string) ([]byte, error) {
	t.Helper()
	command := exec.CommandContext(ctx, binary, args...)
	command.Env = append(os.Environ(), "CHATTA_CHAT_TEST_OWNER_PID="+ownerPID)
	return command.CombinedOutput()
}

func serveShutdownFixture(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		go serveShutdownFixtureConn(conn)
	}
}

func serveShutdownFixtureConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 1024), 16*1024)
	writer := bufio.NewWriter(conn)
	write := func(line string) bool {
		if _, err := fmt.Fprint(writer, line+"\r\n"); err != nil {
			return false
		}
		return writer.Flush() == nil
	}
	nick := ""
	for reader.Scan() {
		line := reader.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "NICK":
			if len(fields) > 1 {
				nick = fields[1]
				if !write(":fixture 001 " + nick + " :welcome") {
					return
				}
			}
		case "JOIN":
			if len(fields) > 1 {
				channel := strings.TrimPrefix(fields[1], ":")
				if !write(":"+nick+"!fixture@localhost JOIN "+channel) || !write(":fixture 366 "+nick+" "+channel+" :End of NAMES list") {
					return
				}
			}
		case "PING":
			payload := "fixture"
			if len(fields) > 1 {
				payload = strings.TrimPrefix(fields[1], ":")
			}
			if !write(":fixture PONG fixture :" + payload) {
				return
			}
		case "QUIT":
			return
		}
	}
}
