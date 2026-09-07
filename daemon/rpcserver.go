package main

import (
	"context"

	"github.com/besrabasant/ssh-tunnel-manager/daemon/tasks"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/tunnelmanager"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
)

type server struct {
	rpc.UnimplementedDaemonServiceServer
	service       tunnelmanager.TunnelService
	configManager configmanager.ConfigManager
}

func (s *server) RegisterTunnelService(service tunnelmanager.TunnelService) {
	s.service = service
}

func (s *server) ListConfigurations(ctx context.Context, req *rpc.ListConfigurationsRequest) (*rpc.ListConfigurationsResponse, error) {
	return tasks.ListConfigurationTask(ctx, req)
}

func (s *server) ListConfigurationsJSON(ctx context.Context, req *rpc.ListConfigurationsJSONRequest) (*rpc.ListConfigurationsJSONResponse, error) {
	return tasks.ListConfigurationsJSONTask(ctx, req)
}

func (s *server) FetchConfiguration(ctx context.Context, req *rpc.FetchConfigurationRequest) (*rpc.FetchConfigurationResponse, error) {
	return tasks.FetchTunnelConfigTask(ctx, req)
}

func (s *server) UpdateConfiguration(ctx context.Context, req *rpc.AddOrUpdateConfigurationRequest) (*rpc.AddOrUpdateConfigurationResponse, error) {
	return tasks.UpdateConfiguration(ctx, req, s.service)
}

func (s *server) UpdateConfigurationJSON(ctx context.Context, req *rpc.AddOrUpdateConfigurationRequest) (*rpc.MutationResponse, error) {
	return tasks.UpdateConfigurationJSON(ctx, req, s.service)
}

func (s *server) AddConfiguration(ctx context.Context, req *rpc.AddOrUpdateConfigurationRequest) (*rpc.AddOrUpdateConfigurationResponse, error) {
	return tasks.AddConfiguration(ctx, req)
}

func (s *server) AddConfigurationJSON(ctx context.Context, req *rpc.AddOrUpdateConfigurationRequest) (*rpc.MutationResponse, error) {
	return tasks.AddConfigurationJSON(ctx, req)
}

func (s *server) DeleteConfiguration(ctx context.Context, req *rpc.DeleteConfigurationRequest) (*rpc.DeleteConfigurationResponse, error) {
	return tasks.DeleteTunnelConfigTask(ctx, req, s.service)
}

func (s *server) DeleteConfigurationJSON(ctx context.Context, req *rpc.DeleteConfigurationRequest) (*rpc.MutationResponse, error) {
	return tasks.DeleteConfigurationJSON(ctx, req, s.service)
}

// Tunneling
func (s *server) StartTunnel(ctx context.Context, req *rpc.StartTunnelRequest) (*rpc.StartTunnelResponse, error) {
	return tasks.StartTunnelTask(ctx, req, s.service)
}

func (s *server) KillTunnel(ctx context.Context, req *rpc.KillTunnelRequest) (*rpc.KillTunnelResponse, error) {
	return tasks.KillTunnelTask(ctx, req, s.service)
}

func (s *server) ListActiveTunnels(ctx context.Context, req *rpc.ListActiveTunnelsRequest) (*rpc.ListActiveTunnelsResponse, error) {
	return tasks.ListActiveTunnelsTask(ctx, req, s.service)
}

func (s *server) ListActiveTunnelsJSON(ctx context.Context, req *rpc.ListActiveTunnelsJSONRequest) (*rpc.ListActiveTunnelsJSONResponse, error) {
	return tasks.ListActiveTunnelsJSONTask(ctx, req, s.service)
}

func (s *server) ListMachines(ctx context.Context, req *rpc.ListMachinesRequest) (*rpc.ListMachinesResponse, error) {
	return tasks.ListMachinesTask(ctx, req, s.configManager)
}

func (s *server) GetMachine(ctx context.Context, req *rpc.GetMachineRequest) (*rpc.GetMachineResponse, error) {
	return tasks.GetMachineTask(ctx, req, s.configManager)
}

func (s *server) AddMachine(ctx context.Context, req *rpc.MachineMutationRequest) (*rpc.MachineMutationResponse, error) {
	return tasks.AddMachineTask(ctx, req, s.configManager)
}

func (s *server) UpdateMachine(ctx context.Context, req *rpc.MachineMutationRequest) (*rpc.MachineMutationResponse, error) {
	return tasks.UpdateMachineTask(ctx, req, s.configManager, s.service)
}

func (s *server) DeleteMachine(ctx context.Context, req *rpc.DeleteMachineRequest) (*rpc.MachineMutationResponse, error) {
	return tasks.DeleteMachineTask(ctx, req, s.configManager, s.service)
}

func (s *server) ListTunnelProfiles(ctx context.Context, req *rpc.ListTunnelProfilesRequest) (*rpc.ListTunnelProfilesResponse, error) {
	return tasks.ListTunnelProfilesTask(ctx, req, s.configManager)
}

func (s *server) GetTunnelProfile(ctx context.Context, req *rpc.GetTunnelProfileRequest) (*rpc.GetTunnelProfileResponse, error) {
	return tasks.GetTunnelProfileTask(ctx, req, s.configManager)
}

func (s *server) AddTunnelProfile(ctx context.Context, req *rpc.TunnelProfileMutationRequest) (*rpc.TunnelProfileMutationResponse, error) {
	return tasks.AddTunnelProfileTask(ctx, req, s.configManager)
}

func (s *server) UpdateTunnelProfile(ctx context.Context, req *rpc.TunnelProfileMutationRequest) (*rpc.TunnelProfileMutationResponse, error) {
	return tasks.UpdateTunnelProfileTask(ctx, req, s.configManager, s.service)
}

func (s *server) DeleteTunnelProfile(ctx context.Context, req *rpc.DeleteTunnelProfileRequest) (*rpc.TunnelProfileMutationResponse, error) {
	return tasks.DeleteTunnelProfileTask(ctx, req, s.configManager, s.service)
}
