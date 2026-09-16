//go:build integration && (darwin || linux)

package chat_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/config"
	"github.com/alswl/chatta/pkg/dal"
	"github.com/alswl/chatta/pkg/services"
)

// shortHome returns a temp directory short enough that <home>/control.sock
// stays under the platform's sun_path limit (104 bytes on macOS) even
// though t.TempDir() would otherwise embed this test's full (long) name.
func shortHome(t *testing.T, label string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "cthome")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, label)
}

// newSession starts an owner-bound session against the given ngircd port
// with no external chat client involved — the whole point of this feature.
func newSession(t *testing.T, binary string, port int, home, nick string) *services.ChatService {
	t.Helper()
	owner := common.OwnerBinding{PID: os.Getpid(), StartFingerprint: dal.ProcessStart(os.Getpid()), Runtime: "test"}
	m := services.NewChatService(config.ChatConfig{Home: home, Host: "127.0.0.1", Port: port, Channel: "#agents"})
	m.Executable = binary
	m.OwnerLookup = func() (common.OwnerBinding, error) { return owner, nil }
	if err := m.Start(nick, "smoke", false); err != nil {
		t.Fatalf("start %s: %v", nick, err)
	}
	t.Cleanup(func() { _ = m.Stop(true) })
	return m
}

func TestNativeTransportChannelMessageCrossesBetweenSessions(t *testing.T) {
	if _, err := exec.LookPath("ngircd"); err != nil {
		t.Skip("scenario requires ngircd")
	}
	port := freePort(t)
	server := startServer(t, port)
	t.Cleanup(func() { stopCommand(server) })
	binary := buildChatta(t)

	first := newSession(t, binary, port, shortHome(t, "first"), "smoke-one")
	second := newSession(t, binary, port, shortHome(t, "second"), "smoke-two")
	if err := first.Join("smoke-work"); err != nil {
		t.Fatal(err)
	}
	if err := second.Join("smoke-work"); err != nil {
		t.Fatal(err)
	}
	if err := first.Send("#smoke-work", "channel hello"); err != nil {
		t.Fatal(err)
	}
	waitForMessage(t, second, "channel hello")
}

func TestNativeTransportDirectMessageAttribution(t *testing.T) {
	if _, err := exec.LookPath("ngircd"); err != nil {
		t.Skip("scenario requires ngircd")
	}
	port := freePort(t)
	server := startServer(t, port)
	t.Cleanup(func() { stopCommand(server) })
	binary := buildChatta(t)

	first := newSession(t, binary, port, shortHome(t, "first"), "smoke-one")
	second := newSession(t, binary, port, shortHome(t, "second"), "smoke-two")
	if err := first.DM("smoke-two", "private hello"); err != nil {
		t.Fatal(err)
	}
	line := waitForMessage(t, second, "private hello")
	if !strings.Contains(line, "(DM)") || !strings.Contains(line, "<smoke-one>") {
		t.Fatalf("direct message not attributed to sender: %q", line)
	}
}

func TestNativeTransportNonASCIIDeliveredByteIdentical(t *testing.T) {
	if _, err := exec.LookPath("ngircd"); err != nil {
		t.Skip("scenario requires ngircd")
	}
	port := freePort(t)
	server := startServer(t, port)
	t.Cleanup(func() { stopCommand(server) })
	binary := buildChatta(t)

	first := newSession(t, binary, port, shortHome(t, "first"), "smoke-one")
	second := newSession(t, binary, port, shortHome(t, "second"), "smoke-two")
	if err := first.Join("smoke-work"); err != nil {
		t.Fatal(err)
	}
	if err := second.Join("smoke-work"); err != nil {
		t.Fatal(err)
	}
	want := "你好，世界 — こんにちは"
	if err := first.Send("#smoke-work", want); err != nil {
		t.Fatal(err)
	}
	waitForMessage(t, second, want)
}

func TestNativeTransportRecoversAfterServerRestart(t *testing.T) {
	if _, err := exec.LookPath("ngircd"); err != nil {
		t.Skip("scenario requires ngircd")
	}
	port := freePort(t)
	server := startServer(t, port)
	binary := buildChatta(t)

	first := newSession(t, binary, port, shortHome(t, "first"), "smoke-one")
	second := newSession(t, binary, port, shortHome(t, "second"), "smoke-two")
	if err := first.Join("smoke-work"); err != nil {
		t.Fatal(err)
	}
	if err := second.Join("smoke-work"); err != nil {
		t.Fatal(err)
	}

	stopCommand(server)
	deadline := time.Now().Add(8 * time.Second)
	var lastFailure string
	for time.Now().Before(deadline) {
		r := first.Health(false)
		if r.Failure != "" {
			lastFailure = r.Failure
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if lastFailure == "" {
		t.Fatal("health did not report a failure after the server went down")
	}

	server = startServer(t, port)
	t.Cleanup(func() { stopCommand(server) })

	if err := first.Send("#smoke-work", "recovered hello"); err != nil {
		t.Fatal(err)
	}
	waitForMessage(t, second, "recovered hello")
}

func waitForMessage(t *testing.T, m *services.ChatService, want string) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var seen []string
	for time.Now().Before(deadline) {
		lines, err := m.Poll(false)
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, lines...)
		for _, line := range lines {
			if strings.Contains(line, want) {
				return line
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("did not receive %q; saw %q", want, seen)
	return ""
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func startServer(t *testing.T, port int) *exec.Cmd {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "ngircd.conf")
	contents := fmt.Sprintf("[Global]\nName = smoke.local\nListen = 127.0.0.1\nPorts = %d\n[Limits]\nMaxConnectionsIP = 50\n[Options]\nDNS = false\nIdent = false\n", port)
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ngircd", "--nodaemon", "--config", configPath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			_ = connection.Close()
			return cmd
		}
		time.Sleep(50 * time.Millisecond)
	}
	stopCommand(cmd)
	t.Fatal("ngircd did not become ready")
	return nil
}

func buildChatta(t *testing.T) string {
	t.Helper()
	rootOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "chatta")
	build := exec.Command("go", "build", "-o", binary, "./cmd/chatta")
	build.Dir = strings.TrimSpace(string(rootOut))
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build chatta: %v\n%s", err, output)
	}
	return binary
}

func stopCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

// The runtime exiting under the agent is the one case where no agent layer is
// left to say goodbye, so the supervisor has to send it.
func TestNativeTransportAnnouncesDepartureWhenOwnerExits(t *testing.T) {
	if _, err := exec.LookPath("ngircd"); err != nil {
		t.Skip("scenario requires ngircd")
	}
	port := freePort(t)
	server := startServer(t, port)
	t.Cleanup(func() { stopCommand(server) })
	binary := buildChatta(t)

	watcher := newSession(t, binary, port, shortHome(t, "watcher"), "smoke-one")
	if err := watcher.Join("smoke-work"); err != nil {
		t.Fatal(err)
	}

	// A session whose owner is a process this test can actually end.
	owner := exec.Command("sleep", "300")
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Process.Kill() })
	pid := owner.Process.Pid
	leaving := services.NewChatService(config.ChatConfig{Home: shortHome(t, "leaving"), Host: "127.0.0.1", Port: port, Channel: "#agents"})
	leaving.Executable = binary
	leaving.OwnerLookup = func() (common.OwnerBinding, error) {
		return common.OwnerBinding{PID: pid, StartFingerprint: dal.ProcessStart(pid), Runtime: "test"}, nil
	}
	if err := leaving.Start("smoke-two", "smoke", false); err != nil {
		t.Fatalf("start smoke-two: %v", err)
	}
	t.Cleanup(func() { _ = leaving.Stop(true) })
	if err := leaving.Join("smoke-work"); err != nil {
		t.Fatal(err)
	}

	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	// Reap it: an unwaited child stays a zombie, and a zombie still answers
	// kill -0, so the supervisor would never see the owner leave.
	_ = owner.Wait()
	line := waitForMessage(t, watcher, "signing off")
	if !strings.Contains(line, "<smoke-two>") || !strings.Contains(line, "[STATUS]") {
		t.Fatalf("departure not announced as a tagged status from the leaver: %q", line)
	}
}
