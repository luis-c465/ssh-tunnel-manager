package tasks

import (
	"context"
	"fmt"

	"github.com/besrabasant/ssh-tunnel-manager/config"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/tunnelmanager"
	pb "github.com/besrabasant/ssh-tunnel-manager/rpc"
	"github.com/besrabasant/ssh-tunnel-manager/utils"
)

func UpdateConfigurationJSON(ctx context.Context, req *pb.AddOrUpdateConfigurationRequest, service tunnelmanager.TunnelService) (*pb.MutationResponse, error) {
	if req == nil || req.Data == nil || req.Name == "" || req.Data.Name != req.Name {
		return &pb.MutationResponse{Status: pb.ResponseStatus_Error, Message: "configuration name is missing or inconsistent"}, nil
	}

	// Block update if there's an active connection using this config (same behavior as legacy string RPC)
	connections := service.GetManager().ConnectionsSnapshot()
	var port int = -1
	for p, ci := range connections {
		if ci.Config.Name == req.Name {
			port = p
			break
		}
	}

	if port != -1 {
		return &pb.MutationResponse{
			Status:  pb.ResponseStatus_Error,
			Message: fmt.Sprintf("Cannot update configuration: connection open on port %d", port),
		}, nil
	}

	// Resolve daemon config dir and update
	dir, err := utils.ResolveDir(config.ConfigurationDir())
	if err != nil {
		return nil, fmt.Errorf("resolve config directory: %w", err)
	}
	if err := configmanager.NewManager(dir).UpdateConfiguration(*configmanager.ConvertRpcTunnelConfigToConfig(req.Data)); err != nil {
		return &pb.MutationResponse{
			Status:  pb.ResponseStatus_Error,
			Message: fmt.Sprintf("Cannot update configuration %s: %v", req.Name, err),
		}, nil
	}

	return &pb.MutationResponse{
		Status:  pb.ResponseStatus_Success,
		Message: fmt.Sprintf("Successfully updated configuration %s", req.Name),
		Data:    req.Data,
	}, nil
}
