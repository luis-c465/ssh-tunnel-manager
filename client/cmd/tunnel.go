package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/client/formatters"
	"github.com/besrabasant/ssh-tunnel-manager/client/lib"
	pb "github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/spf13/cobra"
)

var StartSshTunnelCmd = &cobra.Command{
	Use:     "tunnel <configuration name> [local port]",
	Aliases: []string{"t"},
	Short:   "Start an SSH tunnel using a saved configuration, optionally specifying a local port.",
	Long: `
Start an SSH tunnel using a predefined configuration, with the option to specify a local port for forwarding.

This command initiates an SSH tunnel based on a saved configuration you specify by name. It's designed to forward connections from a local port on your machine to a remote destination defined in the configuration. If you do not specify a local port, the system will automatically allocate a random port for forwarding.

When specifying a local port, ensure it is not in use to avoid binding errors. The command provides a seamless way to establish secure SSH connections for various purposes like secure remote access or port forwarding for services.

Usage:
- sshtm tunnel my_configuration: Starts a tunnel using the my_configuration setup. The tunnel will use the local port saved with the configuration if not specified.
- sshtm tunnel my_configuration 8080: Starts a tunnel using the my_configuration setup with local port 8080 explicitly defined for forwarding.
- sshtm t my_configuration
- sshtm t my_configuration 8080
`,
	Args:          cobra.RangeArgs(1, 2),
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		configName := args[0]
		localPort := -1
		if len(args) > 1 {
			parsedPort, err := strconv.Atoi(args[1])
			if err != nil || parsedPort < 1 || parsedPort > 65535 {
				return fmt.Errorf("local port must be an integer between 1 and 65535")
			}
			localPort = parsedPort
		}

		c, cleanup, err := lib.CreateDaemonServiceClient()
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer cleanup()

		ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
		defer cancel()

		r, err := c.StartTunnel(ctx, &pb.StartTunnelRequest{ConfigName: configName, LocalPort: int32(localPort)})
		if err != nil {
			return fmt.Errorf("start tunnel: %w", err)
		}
		if r.GetStatus() == pb.ResponseStatus_Error {
			if r.GetMessage() != "" {
				return fmt.Errorf("start tunnel: %s", r.GetMessage())
			}
			return fmt.Errorf("start tunnel failed: %s", r.GetResult())
		}

		fmt.Print(formatters.NewOperationFormatter(os.Stdout).Format(formatters.OperationFromStart(r)))
		return nil
	},
}
