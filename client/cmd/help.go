package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const sshtmPreface = `
SSH Tunnel Manager starts and manages SSH port-forwarding tunnels.

Common usage:
  sshtm start <profile>
  sshtm start <machine> <remote-port|remote-host:remote-port>
  sshtm list
  sshtm stop <connection-name-or-local-port>

Temporary tunnels started through a machine are not saved and are not restored
when the daemon restarts. Use "sshtm machine" and "sshtm profile" to manage
saved SSH machines and tunnel profiles.
`

var HelpCmd = &cobra.Command{
	Use:   "help",
	Short: "Show help",
	Long:  sshtmPreface,
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := cmd.Root()
		if len(args) == 0 {
			return root.Help()
		}

		target, _, err := root.Find(args)
		if err != nil || target == nil {
			return fmt.Errorf("unknown help topic: %q", strings.Join(args, " "))
		}
		return target.Help()
	},
}
