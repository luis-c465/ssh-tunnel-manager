package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/client/formatters"
	"github.com/besrabasant/ssh-tunnel-manager/client/lib"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/spf13/cobra"
)

// ListTunnelsCmd lists currently running tunnels.
var ListTunnelsCmd = newListTunnelsCommand()

func newListTunnelsCommand() *cobra.Command {
	var machineName, profileName string
	command := &cobra.Command{
		Use:   "list",
		Short: "List active tunnels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, cleanup, err := lib.CreateDaemonServiceClient()
			if err != nil {
				return fmt.Errorf("connect to daemon: %w", err)
			}
			defer cleanup()

			var machineID, profileID string
			if machineName != "" {
				machine, err := findMachine(cmd.Context(), client, machineName)
				if err != nil {
					return err
				}
				machineID = machine.GetId()
			}
			if profileName != "" {
				profile, err := findTunnelProfile(cmd.Context(), client, profileName)
				if err != nil {
					return err
				}
				profileID = profile.GetId()
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			response, err := client.ListActiveTunnels(ctx, &rpc.ListActiveTunnelsRequest{})
			if err != nil {
				return fmt.Errorf("list active tunnels: %w", err)
			}
			if response == nil {
				return fmt.Errorf("list active tunnels: empty response")
			}
			if machineID != "" || profileID != "" {
				filtered := make([]*rpc.ActiveTunnel, 0, len(response.GetTunnels()))
				for _, tunnel := range response.GetTunnels() {
					if tunnel != nil && (machineID == "" || tunnel.GetMachineId() == machineID) && (profileID == "" || tunnel.GetProfileId() == profileID) {
						filtered = append(filtered, tunnel)
					}
				}
				response.Tunnels = filtered
				if len(filtered) == 0 {
					response.Result = "No matching active tunnels.\n"
				} else {
					response.Result = ""
				}
			}
			fmt.Fprint(cmd.OutOrStdout(), formatters.NewActiveTunnelsFormatter(cmd.OutOrStdout()).Format(response))
			return nil
		},
	}
	command.Flags().StringVar(&machineName, "machine", "", "Only tunnels through this machine")
	command.Flags().StringVar(&profileName, "profile", "", "Only tunnels started from this profile")
	return command
}
