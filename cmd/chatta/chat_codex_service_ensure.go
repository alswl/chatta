package main

import (
	"fmt"

	"github.com/alswl/chatta/pkg/services"
	"github.com/spf13/cobra"
)

func init() {
	ensureCmd := &cobra.Command{Use: "ensure", Args: cobra.NoArgs, Short: "Reuse or start the shared Codex app-server", RunE: ensureCodexService}
	ensureCmd.Flags().String("service-home", "", "shared service data directory (default: per-user config directory)")
	ensureCmd.Flags().String("endpoint", "", "reuse a healthy local Unix app-server endpoint")
	codexServiceCmd.AddCommand(withJSON(ensureCmd))
}

func ensureCodexService(cmd *cobra.Command, _ []string) error {
	home, _ := cmd.Flags().GetString("service-home")
	endpoint, _ := cmd.Flags().GetString("endpoint")
	status, err := services.EnsureCodexAppServer(cmd.Context(), home, endpoint)
	if err != nil {
		return err
	}
	if wantsJSON(cmd) {
		return emitJSON(cmd, status)
	}
	pid := "unmanaged"
	if status.PID != 0 {
		pid = fmt.Sprint(status.PID)
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "codex_app_server=%s endpoint=%s version=%s codex_home=%s platform=%s/%s pid=%s reused=%t\n", status.State, status.Endpoint, status.Version, status.CodexHome, status.PlatformFamily, status.PlatformOS, pid, status.Reused)
	return err
}
