package cmd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/client/formatters"
	"github.com/besrabasant/ssh-tunnel-manager/client/lib"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/spf13/cobra"
)

// StopTunnelCmd stops a running tunnel by connection name or local port.
var StopTunnelCmd = &cobra.Command{
	Use:     "stop <connection-name-or-local-port>",
	Aliases: []string{"kill", "terminate"},
	Short:   "Stop an active tunnel",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		identifier := args[0]
		localPort := 0
		if port, err := strconv.Atoi(identifier); err == nil {
			identifier = ""
			localPort = port
		}

		client, cleanup, err := lib.CreateDaemonServiceClient()
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer cleanup()

		ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
		defer cancel()
		response, err := client.KillTunnel(ctx, &rpc.KillTunnelRequest{ConfigName: identifier, LocalPort: int32(localPort)})
		if err != nil {
			return fmt.Errorf("stop tunnel: %w", err)
		}
		if response.GetStatus() == rpc.ResponseStatus_Error {
			return responseError("stop tunnel", response.GetMessage())
		}
		fmt.Fprint(cmd.OutOrStdout(), formatters.NewMutationFormatter(cmd.OutOrStdout()).Format(formatters.MutationFromKill(response)))
		return nil
	},
}
