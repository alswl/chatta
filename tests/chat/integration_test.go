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
	"github.com/alswl/chatta/pkg/managers"
)

func TestLocalDaemonLifecycleAndMessaging(t *testing.T) {
	if _, err := exec.LookPath("ii"); err != nil {
		t.Skip("integration scenarios require ii")
	}
	if _, err := exec.LookPath("ngircd"); err != nil {
		t.Skip("integration scenarios require ngircd")
	}
	port := freePort(t)
	server := startServer(t, port)
	t.Cleanup(func() { stopCommand(server) })
	binary := buildChatta(t)
	owner := common.OwnerBinding{PID: os.Getpid(), StartFingerprint: dal.ProcessStart(os.Getpid()), Runtime: "test"}
	newManager := func(home, nick string) *managers.Manager {
		m := managers.NewManager(config.ChatConfig{Home: home, Host: "127.0.0.1", Port: port, Channel: "#agents"})
		m.Executable = binary
		m.OwnerLookup = func() (common.OwnerBinding, error) { return owner, nil }
		if err := m.Start(nick, "smoke", false); err != nil {
			t.Fatalf("start %s: %v", nick, err)
		}
		t.Cleanup(func() { _ = m.Stop(true) })
		return m
	}
	first := newManager(filepath.Join(t.TempDir(), "first"), "smoke-one")
	second := newManager(filepath.Join(t.TempDir(), "second"), "smoke-two")
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
	if err := first.DM("smoke-two", "private hello"); err != nil {
		t.Fatal(err)
	}
	waitForMessage(t, second, "private hello")

	ownerProcess := exec.Command("sleep", "30")
	if err := ownerProcess.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ownerProcess.Process.Kill(); _ = ownerProcess.Wait() })
	deadOwner := common.OwnerBinding{PID: ownerProcess.Process.Pid, StartFingerprint: dal.ProcessStart(ownerProcess.Process.Pid), Runtime: "test"}
	third := managers.NewManager(config.ChatConfig{Home: filepath.Join(t.TempDir(), "third"), Host: "127.0.0.1", Port: port, Channel: "#agents"})
	third.Executable = binary
	third.OwnerLookup = func() (common.OwnerBinding, error) { return deadOwner, nil }
	if err := third.Start("smoke3", "owner-death", false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = third.Stop(true) })
	if err := ownerProcess.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	_, _ = ownerProcess.Process.Wait()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if !third.Health(false).Owner {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("owner death was not observed")
}

func waitForMessage(t *testing.T, m *managers.Manager, want string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	var seen []string
	for time.Now().Before(deadline) {
		lines, err := m.Poll(false)
		if err != nil {
			t.Fatal(err)
		}
		seen = append(seen, lines...)
		for _, line := range lines {
			if strings.Contains(line, want) {
				return
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("did not receive %q; saw %q", want, seen)
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
