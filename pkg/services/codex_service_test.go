package services

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestCodexAppServerFixtureProcess(t *testing.T) {
	if os.Getenv("CHATTA_CODEX_FIXTURE") != "1" {
		return
	}
	path := os.Getenv("CHATTA_CODEX_FIXTURE_SOCKET")
	_ = os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		os.Exit(2)
	}
	mode := os.Getenv("CHATTA_CODEX_FIXTURE_SOCKET_MODE")
	if mode == "" {
		mode = "0600"
	}
	if mode == "0660" {
		_ = os.Chmod(path, 0660)
	} else {
		_ = os.Chmod(path, 0600)
	}
	upgrader := websocket.Upgrader{}
	httpServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var request map[string]any
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			if request["id"] != nil {
				result := map[string]any{"codexHome": "/fixture", "platformFamily": "unix", "userAgent": "test-codex-app-server"}
				if badFile := os.Getenv("CHATTA_CODEX_FIXTURE_BAD_HEALTH_FILE"); badFile != "" {
					if _, err := os.Stat(badFile); err == nil {
						result = map[string]any{"unexpected": true}
					}
				}
				_ = conn.WriteJSON(map[string]any{"id": request["id"], "result": result})
			}
		}
	})}
	go func() { _ = httpServer.Serve(listener) }()
	select {}
}

func TestEnsureCodexAppServerSerializesStartsAndSurvivesCallerExit(t *testing.T) {
	serviceHome := shortTempDir(t)
	setupFixtureCodex(t, filepath.Join(serviceHome, "bad-health"), "0660")

	const callers = 6
	results := make(chan CodexServiceStatus, callers)
	errors := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, err := EnsureCodexAppServer(context.Background(), serviceHome, "")
			if err != nil {
				errors <- err
				return
			}
			results <- status
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	var first CodexServiceStatus
	started := 0
	for result := range results {
		if first.PID == 0 {
			first = result
		}
		if result.PID != first.PID {
			t.Fatalf("competing ensure calls returned different PIDs: %d and %d", first.PID, result.PID)
		}
		if !result.Reused {
			started++
		}
	}
	require.Equal(t, 1, started, "exactly one caller starts the service")
	require.NotZero(t, first.PID)
	require.Equal(t, "unix://"+filepath.Join(serviceHome, "app-server.sock"), first.Endpoint)
	require.Equal(t, "codex-cli 9.9.9", first.Version)
	external, err := EnsureCodexAppServer(context.Background(), "", first.Endpoint)
	require.NoError(t, err)
	require.True(t, external.Reused)
	require.Empty(t, external.PID, "reuse-only endpoints do not claim process ownership")
	require.Equal(t, "unknown", external.Version)
	require.Equal(t, "/fixture", external.CodexHome)
	require.Equal(t, "unix", external.PlatformFamily)
	socketPath := filepath.Join(serviceHome, "app-server.sock")
	socketInfo, err := os.Stat(socketPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), socketInfo.Mode().Perm(), "managed startup normalizes Codex's group-accessible socket")
	require.NoError(t, os.Chmod(socketPath, 0660))
	_, err = EnsureCodexAppServer(context.Background(), "", first.Endpoint)
	require.ErrorContains(t, err, "not healthy")
	socketInfo, err = os.Stat(socketPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0660), socketInfo.Mode().Perm(), "reuse-only endpoint must not chmod an external socket")

	// A later caller must reuse the process after the original ensure call returns.
	reused, err := EnsureCodexAppServer(context.Background(), serviceHome, "")
	require.NoError(t, err)
	require.True(t, reused.Reused)
	require.Equal(t, first.PID, reused.PID)
	socketInfo, err = os.Stat(socketPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), socketInfo.Mode().Perm(), "the recorded managed service may restore its own socket contract")
	record := readTestRecord(t, serviceHome)
	require.True(t, processMatches(first.PID, record.Start, record.Executable, record.Endpoint))
	info, err := os.Stat(filepath.Join(serviceHome, "service.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	homeInfo, err := os.Stat(serviceHome)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), homeInfo.Mode().Perm())
	logInfo, err := os.Stat(filepath.Join(serviceHome, "app-server.log"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), logInfo.Mode().Perm())
	codexLogInfo, err := os.Stat(filepath.Join(serviceHome, "codex-log"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), codexLogInfo.Mode().Perm())

	t.Cleanup(func() {
		stopStartedServer(first.PID, record.Start, record.Executable, record.Endpoint)
	})
}

func TestEnsureRejectsUnsafeAndUnhealthyConfiguredEndpoint(t *testing.T) {
	_, err := EnsureCodexAppServer(context.Background(), "", "ws://127.0.0.1:1234")
	require.ErrorContains(t, err, "absolute local unix")
	_, err = EnsureCodexAppServer(context.Background(), "", "unix:///tmp/not-a-codex-server.sock")
	require.ErrorContains(t, err, "not healthy")
	_, err = EnsureCodexAppServer(context.Background(), shortTempDir(t), "unix:///tmp/not-a-codex-server.sock")
	require.ErrorContains(t, err, "choose either")
}

func TestEnsureDoesNotDuplicateRecordedLiveUnhealthyProcess(t *testing.T) {
	serviceHome := shortTempDir(t)
	badHealth := filepath.Join(serviceHome, "bad-health")
	setupFixtureCodex(t, badHealth)
	status, err := EnsureCodexAppServer(context.Background(), serviceHome, "")
	require.NoError(t, err)
	record := readTestRecord(t, serviceHome)
	t.Cleanup(func() { stopStartedServer(status.PID, record.Start, record.Executable, record.Endpoint) })
	require.NoError(t, os.WriteFile(badHealth, nil, 0600))
	_, err = EnsureCodexAppServer(context.Background(), serviceHome, "")
	require.ErrorContains(t, err, "refusing to start a duplicate")
	after := readTestRecord(t, serviceHome)
	require.Equal(t, record.PID, after.PID)
	require.True(t, processMatches(record.PID, record.Start, record.Executable, record.Endpoint), "the live unhealthy service must not be killed")
}

func TestEnsureReplacesDeadStaleRecord(t *testing.T) {
	serviceHome := shortTempDir(t)
	setupFixtureCodex(t, "")
	recordPath := filepath.Join(serviceHome, "service.json")
	stale := &codexServiceRecord{Version: 1, PID: 1<<30 + 1, Start: "old-process", Endpoint: "unix://" + filepath.Join(serviceHome, "app-server.sock"), Executable: "/gone/codex", CLIVersion: "codex-cli old"}
	require.NoError(t, writeCodexServiceRecord(recordPath, stale))
	status, err := EnsureCodexAppServer(context.Background(), serviceHome, "")
	require.NoError(t, err)
	require.Equal(t, "started", status.State)
	require.NotEqual(t, stale.PID, status.PID)
	after, err := readCodexServiceRecord(recordPath)
	require.NoError(t, err)
	require.Equal(t, status.PID, after.PID)
	t.Cleanup(func() { stopStartedServer(status.PID, after.Start, after.Executable, after.Endpoint) })
}

func readTestRecord(t *testing.T, home string) *codexServiceRecord {
	t.Helper()
	record, err := readCodexServiceRecord(filepath.Join(home, "service.json"))
	require.NoError(t, err)
	return record
}

func shortTempDir(t *testing.T) string {
	t.Helper()
	path, err := os.MkdirTemp("/tmp", "cs-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(path) })
	return path
}

func setupFixtureCodex(t *testing.T, badHealthFile string, socketMode ...string) {
	t.Helper()
	binDir := t.TempDir()
	testBinary, err := os.Executable()
	require.NoError(t, err)
	codexPath := filepath.Join(binDir, "codex")
	shim := "#!/bin/bash\nif [ \"$1\" = \"--version\" ]; then echo 'codex-cli 9.9.9'; exit 0; fi\nfor endpoint do :; done\nendpoint=${endpoint#unix://}\nexport CHATTA_CODEX_FIXTURE=1 CHATTA_CODEX_FIXTURE_SOCKET=\"$endpoint\" CHATTA_CODEX_FIXTURE_BINARY='" + testBinary + "' CHATTA_CODEX_FIXTURE_EXE='" + codexPath + "'\nexec -a \"$CHATTA_CODEX_FIXTURE_EXE\" \"$CHATTA_CODEX_FIXTURE_BINARY\" -test.run '^TestCodexAppServerFixtureProcess$' \"$@\"\n"
	require.NoError(t, os.WriteFile(codexPath, []byte(shim), 0700))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CHATTA_CODEX_FIXTURE_BAD_HEALTH_FILE", badHealthFile)
	mode := "0600"
	if len(socketMode) != 0 {
		mode = socketMode[0]
	}
	t.Setenv("CHATTA_CODEX_FIXTURE_SOCKET_MODE", mode)
}

func TestCodexEnsureClosesLogWhenPrivateLogDirectoryIsInvalid(t *testing.T) {
	home, err := os.MkdirTemp("/tmp", "chatta-log-")
	require.NoError(t, err)
	defer os.RemoveAll(home)
	tools := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tools, "codex"), []byte("#!/bin/sh\necho codex-cli 0.162.1\n"), 0700))
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	require.NoError(t, os.WriteFile(filepath.Join(home, "codex-log"), []byte("not a directory"), 0600))
	logPath := filepath.Join(home, "app-server.log")
	require.NoError(t, os.WriteFile(logPath, nil, 0600))
	info, err := os.Stat(logPath)
	require.NoError(t, err)
	target := info.Sys().(*syscall.Stat_t)
	for i := 0; i < 3; i++ {
		_, err = EnsureCodexAppServer(context.Background(), home, "")
		require.ErrorContains(t, err, "create private Codex log directory")
	}
	directory, err := os.Open("/dev/fd")
	require.NoError(t, err)
	defer directory.Close()
	entries, err := directory.Readdirnames(-1)
	require.NoError(t, err)
	leaked := 0
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry)
		if err != nil {
			continue
		}
		var stat syscall.Stat_t
		if syscall.Fstat(fd, &stat) == nil && stat.Dev == target.Dev && stat.Ino == target.Ino {
			leaked++
			_ = syscall.Close(fd)
		}
	}
	require.Zero(t, leaked, "failed ensure calls leaked app-server log descriptors")
}
