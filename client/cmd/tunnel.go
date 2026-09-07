package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/client/formatters"
	"github.com/besrabasant/ssh-tunnel-manager/client/lib"
	pb "github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/spf13/cobra"
)

// StartTunnelCmd starts either a saved profile or a temporary tunnel through a machine.
var StartTunnelCmd = newStartTunnelCommand()

func newStartTunnelCommand() *cobra.Command {
	var localPort int
	var remoteHost string
	command := &cobra.Command{
		Use:   "start <profile> | <machine> <remote-port|remote-host:remote-port>",
		Short: "Start a saved profile or a temporary tunnel through a machine",
		Long: `Start a saved tunnel profile, or start a temporary tunnel through an SSH machine.

One argument is a profile name. Two arguments select a machine and its destination.
Temporary tunnels default to localhost on the SSH machine and an automatically
allocated local port.

Examples:
  sshtm start analytics
  sshtm start bastion 5432
  sshtm start bastion db.internal:5432 --local-port 15432`,
		Args:          cobra.RangeArgs(1, 2),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				if cmd.Flags().Changed("remote-host") {
					return fmt.Errorf("--remote-host is only valid when starting a tunnel through a machine")
				}
				return startProfile(cmd, args[0], localPort, cmd.Flags().Changed("local-port"))
			}
			return startMachineTunnel(cmd, args[0], args[1], remoteHost, localPort)
		},
	}
	command.Flags().IntVar(&localPort, "local-port", 0, "Local port (temporary tunnels default to auto-allocate)")
	command.Flags().StringVar(&remoteHost, "remote-host", "", "Destination host as seen from the SSH machine (default localhost)")
	return command
}

func startProfile(cmd *cobra.Command, profileName string, localPort int, overridePort bool) error {
	port := -1
	if overridePort {
		if err := validatePort("local port", localPort, true); err != nil {
			return err
		}
		port = localPort
	}
	client, cleanup, err := lib.CreateDaemonServiceClient()
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
	defer cancel()
	response, err := client.StartTunnel(ctx, &pb.StartTunnelRequest{ConfigName: profileName, LocalPort: int32(port)})
	if err != nil {
		return fmt.Errorf("start tunnel: %w", err)
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return responseError("start tunnel", response.GetMessage())
	}
	fmt.Fprint(cmd.OutOrStdout(), formatters.NewOperationFormatter(cmd.OutOrStdout()).Format(formatters.OperationFromStart(response)))
	return nil
}

func startMachineTunnel(cmd *cobra.Command, machineName, destination, remoteHost string, localPort int) error {
	host, remotePort, err := parseDestination(destination, remoteHost)
	if err != nil {
		return err
	}
	if err := validatePort("local port", localPort, true); err != nil {
		return err
	}
	client, cleanup, err := lib.CreateDaemonServiceClient()
	if err != nil {
		return fmt.Errorf("connect to daemon: %w", err)
	}
	defer cleanup()
	machine, err := findMachine(cmd.Context(), client, machineName)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
	defer cancel()
	response, err := client.StartOneOffTunnel(ctx, &pb.StartOneOffTunnelRequest{
		MachineId: machine.GetId(), RemoteHost: host, RemotePort: int32(remotePort), LocalPort: int32(localPort),
	})
	if err != nil {
		return fmt.Errorf("start tunnel: %w", err)
	}
	if response == nil {
		return fmt.Errorf("start tunnel: empty response")
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return responseError("start tunnel", response.GetMessage())
	}
	fmt.Fprint(cmd.OutOrStdout(), formatters.NewOperationFormatter(cmd.OutOrStdout()).Format(formatters.OperationFromStart(response)))
	return nil
}

func parseDestination(destination, remoteHost string) (string, int, error) {
	host := strings.TrimSpace(remoteHost)
	portText := destination
	if destinationHost, value, found := strings.Cut(destination, ":"); found {
		if host != "" {
			return "", 0, fmt.Errorf("destination host is specified both in the destination and --remote-host")
		}
		host, portText = strings.TrimSpace(destinationHost), value
		if host == "" {
			return "", 0, fmt.Errorf("destination host cannot be empty")
		}
	}
	port, err := strconv.Atoi(portText)
	if err != nil || validatePort("remote port", port, false) != nil {
		return "", 0, fmt.Errorf("remote port must be an integer between 1 and 65535")
	}
	return host, port, nil
}

func validatePort(label string, port int, allowZero bool) error {
	if port < 0 || port > 65535 || (!allowZero && port == 0) {
		if allowZero {
			return fmt.Errorf("%s must be an integer between 0 and 65535", label)
		}
		return fmt.Errorf("%s must be an integer between 1 and 65535", label)
	}
	return nil
}
