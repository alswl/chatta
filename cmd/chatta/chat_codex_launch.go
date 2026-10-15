//go:build darwin || linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/alswl/chatta/pkg/services"
	"github.com/spf13/cobra"
)

func init() {
	launch := &cobra.Command{
		Use:                "launch -- [codex arguments...]",
		Short:              "Prepare the shared app-server and exec the native Codex CLI",
		DisableFlagParsing: true,
		RunE:               launchCodex,
	}
	codexCmd.AddCommand(launch)
}

func launchCodex(cmd *cobra.Command, args []string) error {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		return cmd.Help()
	}
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	// Shell functions require an executable wrapper; never evaluate a command string.
	executable := os.Getenv("CHATTA_CODEX_COMMAND")
	if executable == "" {
		executable = "codex"
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		return fmt.Errorf("resolve Codex executable (shell functions are not executables): %w", err)
	}
	if !codexLaunchInteractive(args) {
		return syscall.Exec(path, append([]string{executable}, args...), os.Environ())
	}
	endpoint := os.Getenv("CHATTA_CODEX_ENDPOINT")
	explicitRemote, hasRemote := codexLaunchRemote(args)
	if hasRemote {
		if explicitRemote == "" {
			return syscall.Exec(path, append([]string{executable}, args...), os.Environ())
		}
		if endpoint != "" && endpoint != explicitRemote {
			return fmt.Errorf("--remote conflicts with CHATTA_CODEX_ENDPOINT")
		}
		endpoint = explicitRemote
	}
	status, err := services.EnsureCodexAppServer(cmd.Context(), os.Getenv("CHATTA_CODEX_SERVICE_HOME"), endpoint)
	if err != nil {
		return err
	}
	argv := append([]string{executable}, args...)
	if !hasRemote {
		argv = append([]string{executable, "--remote", status.Endpoint}, args...)
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Chatta Codex endpoint: %s\n", status.Endpoint)
	// Replacing this process leaves terminal input, resize, signals and exit
	// status under the native client, with no PTY or RPC proxy in between.
	return syscall.Exec(path, argv, os.Environ())
}

// Only interactive entry points need the shared server. Skip option values
// before deciding whether the first positional argument is a subcommand.
func codexLaunchInteractive(args []string) bool {
	commands := map[string]bool{"agents": true, "exec": true, "e": true, "review": true, "login": true, "logout": true, "mcp": true, "plugin": true, "app-server": true, "remote-control": true, "app": true, "completion": true, "update": true, "doctor": true, "sandbox": true, "debug": true, "apply": true, "a": true, "queue": true, "archive": true, "delete": true, "migrate-rollouts": true, "unarchive": true, "cloud": true, "exec-server": true, "features": true, "help": true}
	positional := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return true
		}
		if codexLaunchValueFlags[arg] {
			i++
			continue
		}
		if arg == "-h" || arg == "--help" || arg == "-V" || arg == "--version" {
			return false
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if !positional {
			if commands[arg] {
				return false
			}
			positional = true
		}
	}
	return true
}

var codexLaunchValueFlags = map[string]bool{"-c": true, "--config": true, "-m": true, "--model": true, "-p": true, "--profile": true, "-s": true, "--sandbox": true, "-a": true, "--ask-for-approval": true, "-C": true, "--cd": true, "-i": true, "--image": true, "--local-provider": true, "--add-dir": true, "--enable": true, "--disable": true, "--remote": true, "--remote-auth-token-env": true}

func codexLaunchRemote(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if arg == "--remote" {
			if i+1 < len(args) {
				return args[i+1], true
			}
			return "", true
		}
		if strings.HasPrefix(arg, "--remote=") {
			return strings.TrimPrefix(arg, "--remote="), true
		}
		if codexLaunchValueFlags[arg] {
			i++
		}
	}
	return "", false
}
