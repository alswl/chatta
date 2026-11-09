//go:build darwin || linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCodexLaunchRouting(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		interactive bool
	}{
		{"version", []string{"--version"}, false}, {"help", []string{"--help"}, false},
		{"exec", []string{"exec", "--json", "hello"}, false},
		{"options then exec", []string{"-m", "gpt-6-luna", "-c", "x=exec", "exec", "hello"}, false},
		{"value named help", []string{"-c", "--help", "hello"}, true},
		{"resume", []string{"resume", "thread"}, true}, {"resume help", []string{"resume", "--help"}, false},
		{"prompt named command after separator", []string{"--", "exec"}, true},
		{"quoted prompt", []string{"a; $(false)\nline"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.interactive, codexLaunchInteractive(tc.args)) })
	}
}

func TestCodexLaunchRemoteRespectsArgumentBoundaries(t *testing.T) {
	endpoint, exists := codexLaunchRemote([]string{"-c", "--remote", "hello"})
	require.False(t, exists)
	require.Empty(t, endpoint)
	endpoint, exists = codexLaunchRemote([]string{"resume", "thread", "--remote", "unix:///tmp/a.sock"})
	require.True(t, exists)
	require.Equal(t, "unix:///tmp/a.sock", endpoint)
	endpoint, exists = codexLaunchRemote([]string{"--", "--remote", "prompt"})
	require.False(t, exists)
	require.Empty(t, endpoint)
}

func TestCodexLaunchWrapperHelpDoesNotRequireCodex(t *testing.T) {
	t.Setenv("CHATTA_CODEX_COMMAND", "/nonexistent/codex")
	var out bytes.Buffer
	cmd := &cobra.Command{Use: "launch", RunE: launchCodex}
	cmd.SetOut(&out)
	require.NoError(t, launchCodex(cmd, []string{"--help"}))
	require.Contains(t, out.String(), "Usage:")
}

func TestCodexLaunchCommandArgs(t *testing.T) {
	for _, tc := range []struct {
		value   string
		want    []string
		invalid bool
	}{
		{"", nil, false}, {`[]`, []string{}, false},
		{`["--ccc","a b","","$(false); *\nline","中文"]`, []string{"--ccc", "a b", "", "$(false); *\nline", "中文"}, false},
		{`--ccc`, nil, true}, {`"--ccc"`, nil, true}, {`null`, nil, true},
		{`{}`, nil, true}, {`[1]`, nil, true}, {`[null]`, nil, true},
		{`[true]`, nil, true}, {`[[]]`, nil, true}, {`["x",]`, nil, true},
		{`["x"] []`, nil, true}, {`["\u0000"]`, nil, true},
	} {
		t.Run(tc.value, func(t *testing.T) {
			got, err := codexLaunchCommandArgs(tc.value)
			if tc.invalid {
				require.ErrorContains(t, err, "CHATTA_CODEX_COMMAND_ARGS")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

// A subprocess can be replaced by syscall.Exec without replacing the test runner.
func TestCodexLaunchProcess(t *testing.T) {
	if os.Getenv("CHATTA_TEST_LAUNCH") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			cmd := &cobra.Command{Use: "launch"}
			cmd.SetContext(context.Background())
			if err := launchCodex(cmd, os.Args[i+1:]); err != nil {
				_, _ = os.Stderr.WriteString(err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(2)
}

func TestCodexLaunchExecArguments(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "chatta-launch-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	launcher := filepath.Join(dir, "launcher with spaces")
	require.NoError(t, os.WriteFile(launcher, []byte("#!/bin/sh\nprintf '%s\\0' \"$@\"\nexit 17\n"), 0700))
	socket := filepath.Join(dir, "app.sock")
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(socket, 0600))
	upgrader := websocket.Upgrader{}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var request map[string]any
			if conn.ReadJSON(&request) != nil {
				return
			}
			if request["id"] != nil {
				if conn.WriteJSON(map[string]any{"id": request["id"], "result": map[string]any{"codexHome": "/fixture", "platformFamily": "unix", "userAgent": "test"}}) != nil {
					return
				}
			}
		}
	})}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	endpoint := "unix://" + socket
	prefix := []string{"--ccc", "a b", "", "$(touch " + filepath.Join(dir, "injected") + "); *\n中文", "--remote=launcher-only", "exec"}
	encoded, err := json.Marshal(prefix)
	require.NoError(t, err)
	t.Setenv("CHATTA_TEST_LAUNCH", "1")
	t.Setenv("CHATTA_CODEX_COMMAND", launcher)
	t.Setenv("CHATTA_CODEX_COMMAND_ARGS", string(encoded))
	t.Setenv("CHATTA_CODEX_ENDPOINT", endpoint)
	t.Setenv("CHATTA_CODEX_SERVICE_HOME", "")
	for _, tc := range []struct {
		name       string
		args, want []string
	}{
		{"automatic remote", []string{"--", "-m", "model", "hello world"}, []string{"--remote", endpoint, "-m", "model", "hello world"}},
		{"explicit remote", []string{"resume", "thread", "--remote", endpoint}, []string{"resume", "thread", "--remote", endpoint}},
		{"noninteractive", []string{"--version"}, []string{"--version"}},
		{"without prefix", []string{"--version"}, []string{"--version"}},
		{"missing remote value", []string{"--remote"}, []string{"--remote"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expectedPrefix := prefix
			if tc.name == "without prefix" {
				t.Setenv("CHATTA_CODEX_COMMAND_ARGS", "")
				expectedPrefix = nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestCodexLaunchProcess$", "--"}, tc.args...)...)
			output, err := command.CombinedOutput()
			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr, "%s", output)
			require.Equal(t, 17, exitErr.ExitCode(), "%s", output)
			require.Equal(t, append(append([]string{}, expectedPrefix...), tc.want...), strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00"))
		})
	}
	_, err = os.Stat(filepath.Join(dir, "injected"))
	require.True(t, os.IsNotExist(err))
}

func TestCodexLaunchInvalidArgsFailBeforeResolvingExecutable(t *testing.T) {
	t.Setenv("CHATTA_CODEX_COMMAND", "/nonexistent/codex")
	t.Setenv("CHATTA_CODEX_COMMAND_ARGS", "--ccc")
	require.ErrorContains(t, launchCodex(&cobra.Command{}, nil), "CHATTA_CODEX_COMMAND_ARGS")
}
