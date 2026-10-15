package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alswl/chatta/pkg/dal"
	"github.com/gorilla/websocket"
	"golang.org/x/sys/unix"
)

const codexServiceStartupTimeout = 20 * time.Second

type CodexServiceStatus struct {
	Endpoint       string `json:"endpoint"`
	PID            int    `json:"pid,omitempty"`
	Version        string `json:"version,omitempty"`
	CodexHome      string `json:"codex_home"`
	PlatformFamily string `json:"platform_family"`
	PlatformOS     string `json:"platform_os,omitempty"`
	State          string `json:"state"`
	Reused         bool   `json:"reused"`
}

type codexEndpointIdentity struct {
	CodexHome      string `json:"codex_home"`
	PlatformFamily string `json:"platform_family"`
	PlatformOS     string `json:"platform_os"`
}

type codexServiceRecord struct {
	Version    int                   `json:"version"`
	PID        int                   `json:"pid"`
	Start      string                `json:"start_fingerprint"`
	Endpoint   string                `json:"endpoint"`
	Executable string                `json:"executable"`
	CLIVersion string                `json:"cli_version"`
	Identity   codexEndpointIdentity `json:"identity"`
}

func DefaultCodexServiceHome() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(base, "chatta", "codex-app-server"), nil
}

func EnsureCodexAppServer(ctx context.Context, serviceHome, configuredEndpoint string) (CodexServiceStatus, error) {
	if configuredEndpoint != "" {
		if serviceHome != "" {
			return CodexServiceStatus{}, errors.New("choose either --service-home or --endpoint")
		}
		if err := validateLocalEndpoint(configuredEndpoint); err != nil {
			return CodexServiceStatus{}, err
		}
		identity, err := probeCodexEndpoint(ctx, configuredEndpoint)
		if err != nil {
			return CodexServiceStatus{}, fmt.Errorf("configured Codex endpoint is not healthy: %w", err)
		}
		return CodexServiceStatus{Endpoint: configuredEndpoint, Version: "unknown", CodexHome: identity.CodexHome, PlatformFamily: identity.PlatformFamily, PlatformOS: identity.PlatformOS, State: "reused", Reused: true}, nil
	}
	if serviceHome == "" {
		var err error
		serviceHome, err = DefaultCodexServiceHome()
		if err != nil {
			return CodexServiceStatus{}, err
		}
	}
	serviceHome, err := filepath.Abs(serviceHome)
	if err != nil {
		return CodexServiceStatus{}, err
	}
	if err = secureServiceHome(serviceHome); err != nil {
		return CodexServiceStatus{}, err
	}
	unlock, err := lockCodexService(ctx, serviceHome)
	if err != nil {
		return CodexServiceStatus{}, err
	}
	defer unlock()

	socket := filepath.Join(serviceHome, "app-server.sock")
	endpoint := "unix://" + socket
	if err := validateLocalEndpoint(endpoint); err != nil {
		return CodexServiceStatus{}, err
	}
	recordPath := filepath.Join(serviceHome, "service.json")
	record, recordErr := readCodexServiceRecord(recordPath)
	if recordErr != nil && !errors.Is(recordErr, os.ErrNotExist) {
		return CodexServiceStatus{}, recordErr
	}
	identity, probeErr := probeCodexEndpoint(ctx, endpoint)
	if probeErr != nil && record != nil && record.Endpoint == endpoint && recordProcessMatches(record) {
		// A recorded managed process owns this endpoint. Normalize only this
		// service's socket; configured --endpoint paths never enter this branch.
		if normalizeManagedCodexSocket(socket) == nil {
			identity, probeErr = probeCodexEndpoint(ctx, endpoint)
		}
	}
	if probeErr == nil {
		if record != nil {
			if recordProcessMatches(record) && record.Endpoint != endpoint {
				return CodexServiceStatus{}, errors.New("recorded Codex app-server is alive on a different endpoint")
			}
			if record.PID > 0 && (!recordProcessMatches(record) || record.Endpoint != endpoint) {
				return CodexServiceStatus{}, errors.New("managed Codex endpoint is healthy but its process identity does not match the service record")
			}
		}
		if record != nil && record.Identity != identity {
			return CodexServiceStatus{}, errors.New("managed Codex endpoint identity differs from its service record")
		}
		pid := 0
		version := ""
		if record != nil {
			pid = record.PID
			version = record.CLIVersion
		} else {
			version = "unknown"
		}
		return CodexServiceStatus{Endpoint: endpoint, PID: pid, Version: version, CodexHome: identity.CodexHome, PlatformFamily: identity.PlatformFamily, PlatformOS: identity.PlatformOS, State: "healthy", Reused: true}, nil
	}
	if record != nil && recordProcessMatches(record) {
		return CodexServiceStatus{}, fmt.Errorf("recorded Codex app-server PID %d is alive but its endpoint is unhealthy; refusing to start a duplicate", record.PID)
	}
	if err := removeStaleSocket(socket); err != nil {
		return CodexServiceStatus{}, err
	}
	binary, err := exec.LookPath("codex")
	if err != nil {
		return CodexServiceStatus{}, fmt.Errorf("find Codex CLI: %w", err)
	}
	binary, err = filepath.EvalSymlinks(binary)
	if err != nil {
		return CodexServiceStatus{}, fmt.Errorf("resolve Codex CLI: %w", err)
	}
	version, err := codexCLIVersion(ctx, binary)
	if err != nil {
		return CodexServiceStatus{}, err
	}
	logPath := filepath.Join(serviceHome, "app-server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return CodexServiceStatus{}, fmt.Errorf("open Codex app-server log: %w", err)
	}
	defer func() { _ = logFile.Close() }()
	if err := logFile.Chmod(0600); err != nil {
		_ = logFile.Close()
		return CodexServiceStatus{}, err
	}
	logDir := filepath.Join(serviceHome, "codex-log")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return CodexServiceStatus{}, fmt.Errorf("create private Codex log directory: %w", err)
	}
	if err := os.Chmod(logDir, 0700); err != nil {
		return CodexServiceStatus{}, err
	}
	cmd := exec.Command(binary, "app-server", "-c", "log_dir="+strconv.Quote(logDir), "--listen", endpoint)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return CodexServiceStatus{}, fmt.Errorf("start Codex app-server: %w", err)
	}
	_ = logFile.Close()
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	start := dal.ProcessStart(pid)
	if start == "" {
		stopStartedServer(pid, "", binary, endpoint)
		return CodexServiceStatus{}, fmt.Errorf("could not establish Codex app-server process identity for PID %d", pid)
	}
	ready := false
	defer func() {
		if !ready {
			stopStartedServer(pid, start, binary, endpoint)
			_ = removeStaleSocket(socket)
		}
	}()
	deadline := time.NewTimer(codexServiceStartupTimeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		if !processMatches(pid, start, binary, endpoint) {
			return CodexServiceStatus{}, fmt.Errorf("codex app-server exited during startup; see %s", logPath)
		}
		if normalizeErr := normalizeManagedCodexSocket(socket); normalizeErr == nil {
			identity, probeErr := probeCodexEndpoint(ctx, endpoint)
			if probeErr == nil {
				record = &codexServiceRecord{Version: 1, PID: pid, Start: start, Endpoint: endpoint, Executable: binary, CLIVersion: version, Identity: identity}
				if err := writeCodexServiceRecord(recordPath, record); err != nil {
					return CodexServiceStatus{}, err
				}
				ready = true
				return CodexServiceStatus{Endpoint: endpoint, PID: pid, Version: version, CodexHome: identity.CodexHome, PlatformFamily: identity.PlatformFamily, PlatformOS: identity.PlatformOS, State: "started", Reused: false}, nil
			}
		}
		select {
		case <-ctx.Done():
			return CodexServiceStatus{}, ctx.Err()
		case <-deadline.C:
			return CodexServiceStatus{}, fmt.Errorf("codex app-server did not become healthy within %s; see %s", codexServiceStartupTimeout, logPath)
		case <-ticker.C:
		}
	}
}

func validateLocalEndpoint(endpoint string) error {
	if !strings.HasPrefix(endpoint, "unix:///") {
		return errors.New("codex app-server endpoint must be an absolute local unix:///path endpoint")
	}
	path := strings.TrimPrefix(endpoint, "unix://")
	if len(path) >= 100 || filepath.Clean(path) != path || hasParentSegment(path) {
		return errors.New("codex Unix endpoint path is unsafe or too long")
	}
	if strings.IndexByte(path, 0) >= 0 {
		return errors.New("codex Unix endpoint path contains NUL")
	}
	return nil
}

func hasParentSegment(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func probeCodexEndpoint(ctx context.Context, endpoint string) (codexEndpointIdentity, error) {
	if err := validateLocalEndpoint(endpoint); err != nil {
		return codexEndpointIdentity{}, err
	}
	path := strings.TrimPrefix(endpoint, "unix://")
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return codexEndpointIdentity{}, err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		if stat, ok := linkInfo.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
			return codexEndpointIdentity{}, errors.New("endpoint link is not owned by the current user")
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return codexEndpointIdentity{}, err
	}
	if info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0077 != 0 {
		return codexEndpointIdentity{}, errors.New("endpoint is not a private Unix socket")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return codexEndpointIdentity{}, errors.New("endpoint socket is not owned by the current user")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return codexEndpointIdentity{}, err
	}
	parent, err := os.Stat(filepath.Dir(resolved))
	if err != nil || parent.Mode().Perm()&0077 != 0 {
		return codexEndpointIdentity{}, errors.New("endpoint socket directory is not private")
	}
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second, NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 2 * time.Second}).DialContext(ctx, "unix", path)
	}}
	conn, _, err := dialer.DialContext(ctx, "ws://localhost", nil)
	if err != nil {
		return codexEndpointIdentity{}, err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	r := rpc{conn: conn, timeout: 3 * time.Second}
	result, err := r.call("initialize", map[string]any{"clientInfo": map[string]string{"name": "chatta", "title": "Chatta", "version": "0.1.0"}})
	if err != nil {
		return codexEndpointIdentity{}, err
	}
	if err := r.notify("initialized", map[string]any{}); err != nil {
		return codexEndpointIdentity{}, err
	}
	if getString(result, "codexHome") == "" || getString(result, "platformFamily") == "" {
		return codexEndpointIdentity{}, errors.New("initialize response does not identify a compatible Codex app-server")
	}
	return codexEndpointIdentity{CodexHome: getString(result, "codexHome"), PlatformFamily: getString(result, "platformFamily"), PlatformOS: getString(result, "platformOs")}, nil
}

// normalizeManagedCodexSocket is called only for a service-home endpoint
// whose process is owned by this ensure operation or its private record.
func normalizeManagedCodexSocket(path string) error {
	link, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if link.Mode()&os.ModeSymlink == 0 && link.Mode()&os.ModeSocket == 0 {
		return errors.New("managed Codex endpoint is not a socket or socket link")
	}
	if stat, ok := link.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return errors.New("managed Codex endpoint link is not owned by the current user")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	socket, err := os.Stat(resolved)
	if err != nil {
		return err
	}
	if socket.Mode()&os.ModeSocket == 0 {
		return errors.New("managed Codex endpoint target is not a Unix socket")
	}
	if stat, ok := socket.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return errors.New("managed Codex endpoint socket is not owned by the current user")
	}
	parent, err := os.Stat(filepath.Dir(resolved))
	if err != nil || parent.Mode().Perm()&0077 != 0 {
		return errors.New("managed Codex endpoint socket directory is not private")
	}
	if stat, ok := parent.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return errors.New("managed Codex endpoint socket directory is not owned by the current user")
	}
	if socket.Mode().Perm()&0077 != 0 {
		if err := os.Chmod(resolved, 0600); err != nil {
			return fmt.Errorf("secure managed Codex endpoint socket: %w", err)
		}
	}
	return nil
}

func secureServiceHome(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("create Codex service home: %w", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("codex service home must be a real directory")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return errors.New("codex service home is not owned by the current user")
	}
	if err := os.Chmod(path, 0700); err != nil {
		return fmt.Errorf("secure Codex service home: %w", err)
	}
	return nil
}

func lockCodexService(ctx context.Context, home string) (func(), error) {
	path := filepath.Join(home, ".ensure.lock")
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, fmt.Errorf("open Codex service lock: %w", err)
	}
	f := os.NewFile(uintptr(fd), path)
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return nil, err
	}
	for {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		} else if err != unix.EWOULDBLOCK && err != unix.EAGAIN {
			_ = f.Close()
			return nil, fmt.Errorf("lock Codex service home: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
}

func readCodexServiceRecord(path string) (*codexServiceRecord, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("codex service record is not a private regular file")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return nil, errors.New("codex service record is not owned by the current user")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var record codexServiceRecord
	if err := json.Unmarshal(b, &record); err != nil || record.Version != 1 || record.PID <= 1 || record.Start == "" || record.Endpoint == "" {
		return nil, errors.New("codex service record is invalid; move or remove it after verifying the service")
	}
	return &record, nil
}

func writeCodexServiceRecord(path string, record *codexServiceRecord) error {
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return nil
}

func recordProcessMatches(record *codexServiceRecord) bool {
	return processMatches(record.PID, record.Start, record.Executable, record.Endpoint)
}
func processMatches(pid int, start, executable, endpoint string) bool {
	if pid <= 1 || start == "" || executable == "" || endpoint == "" || dal.ProcessStart(pid) != start {
		return false
	}
	out, err := exec.Command("ps", "-o", "command=", "-p", fmt.Sprint(pid)).Output()
	if err != nil {
		return false
	}
	command := strings.TrimSpace(string(out))
	return strings.Contains(command, " app-server ") && strings.Contains(command, "--listen "+endpoint)
}

func stopStartedServer(pid int, start, executable, endpoint string) {
	if start == "" || !processMatches(pid, start, executable, endpoint) {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	deadline := time.Now().Add(2 * time.Second)
	for processMatches(pid, start, executable, endpoint) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if processMatches(pid, start, executable, endpoint) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 && info.Mode()&os.ModeSymlink == 0 {
		return errors.New("refusing to remove a non-socket at the managed Codex endpoint")
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Getuid() {
		return errors.New("refusing to remove a Codex socket owned by another user")
	}
	return os.Remove(path)
}

func codexCLIVersion(ctx context.Context, binary string) (string, error) {
	versionCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(versionCtx, binary, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("read Codex CLI version: %w", err)
	}
	version := strings.TrimSpace(string(out))
	if !strings.HasPrefix(version, "codex-cli ") {
		return "", fmt.Errorf("unsupported Codex CLI version output %q", version)
	}
	return version, nil
}
