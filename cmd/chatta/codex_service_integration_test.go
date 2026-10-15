//go:build darwin || linux

package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/alswl/chatta/pkg/dal"
	"github.com/stretchr/testify/require"
)

func TestSessionStopPreservesSharedCodexAppServer(t *testing.T) {
	codex, err := exec.LookPath("codex")
	if err != nil {
		t.Skip("Codex CLI is required for this process-level integration test")
	}
	if output, err := exec.Command(codex, "--version").CombinedOutput(); err != nil || !strings.HasPrefix(strings.TrimSpace(string(output)), "codex-cli ") {
		t.Skip("installed Codex CLI does not expose the supported version command")
	}
	binary := os.Getenv("CHATTA_TEST_BINARY")
	if binary == "" {
		binDir, err := os.MkdirTemp("/tmp", "cp3-bin-")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(binDir) })
		binary = filepath.Join(binDir, "chatta")
		build := exec.Command("go", "build", "-o", binary, ".")
		if output, buildErr := build.CombinedOutput(); buildErr != nil {
			t.Fatalf("build Chatta CLI for integration test: %v\n%s", buildErr, output)
		}
	}
	makeHome := func(prefix string) string {
		home, err := os.MkdirTemp("/tmp", prefix)
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(home) })
		return home
	}
	serviceHome := makeHome("cp3-sv-")
	codexHome := makeHome("cp3-cx-")
	sessionHome := makeHome("cp3-chat-")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	go serveShutdownFixture(listener)
	ownerPID := strconv.Itoa(os.Getpid())
	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	env := append(os.Environ(), "CODEX_HOME="+codexHome)

	started := runP3CLI(t, binary, env, "chat", "codex", "service", "ensure", "--service-home", serviceHome, "--json")
	require.NoError(t, started.err, "start isolated shared app-server: %s", started.output)
	var before codexServiceCLIResult
	require.NoError(t, json.Unmarshal(started.output, &before))
	require.Equal(t, "started", before.State)
	require.NotZero(t, before.PID)
	require.NotEmpty(t, before.Endpoint)
	socketPath := filepath.Join(serviceHome, "app-server.sock")
	socketInfo, err := os.Stat(socketPath)
	require.NoError(t, err)
	require.NotZero(t, socketInfo.Mode()&os.ModeSocket)
	require.Equal(t, os.FileMode(0600), socketInfo.Mode().Perm(), "the actual managed Codex socket must be private")
	require.NoError(t, os.Chmod(socketPath, 0660))
	repaired := runP3CLI(t, binary, env, "chat", "codex", "service", "ensure", "--service-home", serviceHome, "--json")
	require.NoError(t, repaired.err, "managed ensure should repair its own socket mode: %s", repaired.output)
	socketInfo, err = os.Stat(socketPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), socketInfo.Mode().Perm())
	serviceRecord, err := os.ReadFile(filepath.Join(serviceHome, "service.json"))
	require.NoError(t, err)
	var record struct {
		Start string `json:"start_fingerprint"`
	}
	require.NoError(t, json.Unmarshal(serviceRecord, &record))
	t.Cleanup(func() {
		if dal.ProcessStart(before.PID) == record.Start {
			_ = syscall.Kill(before.PID, syscall.SIGTERM)
		}
	})

	startSession := runSessionCLI(t, binary, ownerPID, "chat", "--home", sessionHome, "--host", "127.0.0.1", "--port", port, "--channel", "#p3-test", "session", "start", "p3-test", "service lifecycle fixture")
	require.NoError(t, startSession, "start isolated Chatta session")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	stopSession := runP3CLIContext(ctx, binary, env, "chat", "--home", sessionHome, "session", "stop", "--force")
	require.NoError(t, stopSession.err, "stop Chatta session: %s", stopSession.output)

	reused := runP3CLI(t, binary, env, "chat", "codex", "service", "ensure", "--service-home", serviceHome, "--json")
	require.NoError(t, reused.err, "reuse app-server after session stop: %s", reused.output)
	var after codexServiceCLIResult
	require.NoError(t, json.Unmarshal(reused.output, &after))
	require.True(t, after.Reused)
	require.Equal(t, before.PID, after.PID, "stopping one Chatta session must preserve the shared app-server process")
	require.Equal(t, before.Endpoint, after.Endpoint)
}

type codexServiceCLIResult struct {
	Endpoint string `json:"endpoint"`
	PID      int    `json:"pid"`
	State    string `json:"state"`
	Reused   bool   `json:"reused"`
}

type p3CLIResult struct {
	output []byte
	err    error
}

func runP3CLI(t *testing.T, binary string, env []string, args ...string) p3CLIResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return runP3CLIContext(ctx, binary, env, args...)
}

func runP3CLIContext(ctx context.Context, binary string, env []string, args ...string) p3CLIResult {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env
	output, err := cmd.CombinedOutput()
	return p3CLIResult{output: output, err: err}
}
