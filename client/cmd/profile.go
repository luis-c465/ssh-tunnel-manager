package cmd

import (
	"context"
	"fmt"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/spf13/cobra"
)

// ProfileCmd manages saved tunnel profiles.
var ProfileCmd = newProfileCommand()

func newProfileCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "profile",
		Short: "Manage saved tunnel profiles",
		Args:  cobra.NoArgs,
	}
	command.AddCommand(ListConfigurationsCmd)
	command.AddCommand(AddConfigurationsCmd)
	command.AddCommand(EditConfigurationsCmd)
	command.AddCommand(DeleteConfigurationsCmd)
	return command
}

func findTunnelProfile(parent context.Context, client rpc.DaemonServiceClient, name string) (*rpc.TunnelProfile, error) {
	ctx, cancel := context.WithTimeout(parent, machineCommandTimeout)
	defer cancel()
	response, err := client.ListTunnelProfiles(ctx, &rpc.ListTunnelProfilesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list tunnel profiles: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("list tunnel profiles: empty response")
	}
	if response.GetStatus() == rpc.ResponseStatus_Error {
		return nil, responseError("list tunnel profiles", response.GetMessage())
	}
	for _, profile := range response.GetTunnelProfiles() {
		if profile != nil && profile.GetName() == name {
			if profile.GetId() == "" {
				return nil, fmt.Errorf("profile %q has no ID", name)
			}
			return profile, nil
		}
	}
	return nil, fmt.Errorf("profile %q not found", name)
}
