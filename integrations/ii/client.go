//go:build darwin || linux

package ii

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/dal"
)

type Client struct {
	Paths dal.Paths
	Log   *os.File
	Cmd   *exec.Cmd
}

func (i *Client) Start(session common.ChatSession, executable string) error {
	if executable == "" {
		executable = "ii"
	}
	if _, err := exec.LookPath(executable); err != nil {
		return fmt.Errorf("chat transport %q is unavailable; install ii or set --ii/CHATTA_CHAT_II: %w", executable, err)
	}
	if err := os.MkdirAll(i.Paths.Conversations, 0700); err != nil {
		return err
	}
	log, err := os.OpenFile(i.Paths.Log, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	i.Log = log
	cmd := exec.Command(executable, "-s", session.Host, "-p", strconv.Itoa(session.Port), "-n", session.Nick, "-f", session.Role, "-i", i.Paths.Conversations)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return err
	}
	i.Cmd = cmd
	return nil
}

func (i *Client) Close() error {
	if i.Log != nil {
		return i.Log.Close()
	}
	return nil
}
