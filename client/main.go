package main

import (
	"fmt"
	"os"

	"github.com/besrabasant/ssh-tunnel-manager/client/cmd"
)

func main() {
	cmd.SshtmCmd.AddCommand(cmd.StartTunnelCmd)
	cmd.SshtmCmd.AddCommand(cmd.ListTunnelsCmd)
	cmd.SshtmCmd.AddCommand(cmd.StopTunnelCmd)
	cmd.SshtmCmd.AddCommand(cmd.MachineCmd)
	cmd.SshtmCmd.AddCommand(cmd.ProfileCmd)
	cmd.SshtmCmd.Long = cmd.HelpCmd.Long
	cmd.SshtmCmd.AddCommand(cmd.VersionCmd)

	if err := cmd.SshtmCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
