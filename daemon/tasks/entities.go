package tasks

import (
	"context"
	"fmt"

	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/tunnelmanager"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
)

func ListMachinesTask(_ context.Context, req *rpc.ListMachinesRequest, manager configmanager.ConfigManager) (*rpc.ListMachinesResponse, error) {
	if req == nil {
		return &rpc.ListMachinesResponse{Status: rpc.ResponseStatus_Error, Message: "missing list machines request"}, nil
	}
	if manager == nil {
		return &rpc.ListMachinesResponse{Status: rpc.ResponseStatus_Error, Message: "configuration manager is unavailable"}, nil
	}

	machines, err := manager.GetMachines()
	if err != nil {
		return &rpc.ListMachinesResponse{Status: rpc.ResponseStatus_Error, Message: fmt.Sprintf("cannot list machines: %v", err)}, nil
	}
	data := make([]*rpc.Machine, 0, len(machines))
	for _, machine := range machines {
		data = append(data, machineToRPC(machine))
	}
	return &rpc.ListMachinesResponse{Status: rpc.ResponseStatus_Success, Machines: data}, nil
}

func GetMachineTask(_ context.Context, req *rpc.GetMachineRequest, manager configmanager.ConfigManager) (*rpc.GetMachineResponse, error) {
	if req == nil || req.GetId() == "" {
		return &rpc.GetMachineResponse{Status: rpc.ResponseStatus_Error, Message: "missing machine ID"}, nil
	}
	if manager == nil {
		return &rpc.GetMachineResponse{Status: rpc.ResponseStatus_Error, Message: "configuration manager is unavailable"}, nil
	}

	machine, err := manager.GetMachine(req.GetId())
	if err != nil {
		return &rpc.GetMachineResponse{Status: rpc.ResponseStatus_Error, Message: fmt.Sprintf("cannot get machine %q: %v", req.GetId(), err)}, nil
	}
	return &rpc.GetMachineResponse{Status: rpc.ResponseStatus_Success, Data: machineToRPC(machine)}, nil
}

func AddMachineTask(_ context.Context, req *rpc.MachineMutationRequest, manager configmanager.ConfigManager) (*rpc.MachineMutationResponse, error) {
	if req == nil || req.GetData() == nil {
		return machineMutationError("missing machine data"), nil
	}
	if manager == nil {
		return machineMutationError("configuration manager is unavailable"), nil
	}

	machine := machineFromRPC(req.GetData())
	machine.ID = ""
	if err := manager.AddMachine(machine); err != nil {
		return machineMutationError(fmt.Sprintf("cannot add machine: %v", err)), nil
	}
	stored, err := machineByName(manager, machine.Name)
	if err != nil {
		return machineMutationError(fmt.Sprintf("added machine but could not retrieve it: %v", err)), nil
	}
	return &rpc.MachineMutationResponse{Status: rpc.ResponseStatus_Success, Message: fmt.Sprintf("successfully added machine %s", stored.Name), Data: machineToRPC(stored)}, nil
}

func UpdateMachineTask(_ context.Context, req *rpc.MachineMutationRequest, manager configmanager.ConfigManager, service tunnelmanager.TunnelService) (*rpc.MachineMutationResponse, error) {
	if req == nil || req.GetData() == nil {
		return machineMutationError("missing machine data"), nil
	}
	if req.GetId() == "" {
		return machineMutationError("missing machine ID"), nil
	}
	if manager == nil {
		return machineMutationError("configuration manager is unavailable"), nil
	}
	if active, err := activeMachine(service, req.GetId()); err != nil {
		return machineMutationError(err.Error()), nil
	} else if active {
		return machineMutationError(fmt.Sprintf("cannot update machine %q while it has an active connection", req.GetId())), nil
	}

	machine := machineFromRPC(req.GetData())
	machine.ID = req.GetId()
	if err := manager.UpdateMachine(machine); err != nil {
		return machineMutationError(fmt.Sprintf("cannot update machine %q: %v", req.GetId(), err)), nil
	}
	return &rpc.MachineMutationResponse{Status: rpc.ResponseStatus_Success, Message: fmt.Sprintf("successfully updated machine %s", machine.Name), Data: machineToRPC(machine)}, nil
}

func DeleteMachineTask(_ context.Context, req *rpc.DeleteMachineRequest, manager configmanager.ConfigManager, service tunnelmanager.TunnelService) (*rpc.MachineMutationResponse, error) {
	if req == nil || req.GetId() == "" {
		return machineMutationError("missing machine ID"), nil
	}
	if manager == nil {
		return machineMutationError("configuration manager is unavailable"), nil
	}
	if active, err := activeMachine(service, req.GetId()); err != nil {
		return machineMutationError(err.Error()), nil
	} else if active {
		return machineMutationError(fmt.Sprintf("cannot delete machine %q while it has an active connection", req.GetId())), nil
	}

	machine, err := manager.GetMachine(req.GetId())
	if err != nil {
		return machineMutationError(fmt.Sprintf("cannot delete machine %q: %v", req.GetId(), err)), nil
	}
	if err := manager.RemoveMachine(req.GetId()); err != nil {
		return machineMutationError(fmt.Sprintf("cannot delete machine %q: %v", req.GetId(), err)), nil
	}
	return &rpc.MachineMutationResponse{Status: rpc.ResponseStatus_Success, Message: fmt.Sprintf("successfully deleted machine %s", machine.Name), Data: machineToRPC(machine)}, nil
}

func ListTunnelProfilesTask(_ context.Context, req *rpc.ListTunnelProfilesRequest, manager configmanager.ConfigManager) (*rpc.ListTunnelProfilesResponse, error) {
	if req == nil {
		return &rpc.ListTunnelProfilesResponse{Status: rpc.ResponseStatus_Error, Message: "missing list tunnel profiles request"}, nil
	}
	if manager == nil {
		return &rpc.ListTunnelProfilesResponse{Status: rpc.ResponseStatus_Error, Message: "configuration manager is unavailable"}, nil
	}

	profiles, err := manager.GetTunnelProfiles()
	if err != nil {
		return &rpc.ListTunnelProfilesResponse{Status: rpc.ResponseStatus_Error, Message: fmt.Sprintf("cannot list tunnel profiles: %v", err)}, nil
	}
	data := make([]*rpc.TunnelProfile, 0, len(profiles))
	for _, profile := range profiles {
		rpcProfile := tunnelProfileToRPC(profile, nil)
		if machine, err := manager.GetMachine(profile.MachineID); err == nil {
			rpcProfile.Machine = machineToRPC(machine)
		}
		data = append(data, rpcProfile)
	}
	return &rpc.ListTunnelProfilesResponse{Status: rpc.ResponseStatus_Success, TunnelProfiles: data}, nil
}

func GetTunnelProfileTask(_ context.Context, req *rpc.GetTunnelProfileRequest, manager configmanager.ConfigManager) (*rpc.GetTunnelProfileResponse, error) {
	if req == nil || req.GetId() == "" {
		return &rpc.GetTunnelProfileResponse{Status: rpc.ResponseStatus_Error, Message: "missing tunnel profile ID"}, nil
	}
	if manager == nil {
		return &rpc.GetTunnelProfileResponse{Status: rpc.ResponseStatus_Error, Message: "configuration manager is unavailable"}, nil
	}

	profile, err := manager.GetTunnelProfile(req.GetId())
	if err != nil {
		return &rpc.GetTunnelProfileResponse{Status: rpc.ResponseStatus_Error, Message: fmt.Sprintf("cannot get tunnel profile %q: %v", req.GetId(), err)}, nil
	}
	data := tunnelProfileToRPC(profile, nil)
	if machine, err := manager.GetMachine(profile.MachineID); err == nil {
		data.Machine = machineToRPC(machine)
	}
	return &rpc.GetTunnelProfileResponse{Status: rpc.ResponseStatus_Success, Data: data}, nil
}

func AddTunnelProfileTask(_ context.Context, req *rpc.TunnelProfileMutationRequest, manager configmanager.ConfigManager) (*rpc.TunnelProfileMutationResponse, error) {
	if req == nil || req.GetData() == nil {
		return tunnelProfileMutationError("missing tunnel profile data"), nil
	}
	if manager == nil {
		return tunnelProfileMutationError("configuration manager is unavailable"), nil
	}

	profile := tunnelProfileFromRPC(req.GetData())
	profile.ID = ""
	if err := manager.AddTunnelProfile(profile); err != nil {
		return tunnelProfileMutationError(fmt.Sprintf("cannot add tunnel profile: %v", err)), nil
	}
	stored, err := tunnelProfileByName(manager, profile.Name)
	if err != nil {
		return tunnelProfileMutationError(fmt.Sprintf("added tunnel profile but could not retrieve it: %v", err)), nil
	}
	data, err := resolvedProfileToRPC(manager, stored)
	if err != nil {
		return tunnelProfileMutationError(fmt.Sprintf("added tunnel profile but could not resolve it: %v", err)), nil
	}
	return &rpc.TunnelProfileMutationResponse{Status: rpc.ResponseStatus_Success, Message: fmt.Sprintf("successfully added tunnel profile %s", stored.Name), Data: data}, nil
}

func UpdateTunnelProfileTask(_ context.Context, req *rpc.TunnelProfileMutationRequest, manager configmanager.ConfigManager, service tunnelmanager.TunnelService) (*rpc.TunnelProfileMutationResponse, error) {
	if req == nil || req.GetData() == nil {
		return tunnelProfileMutationError("missing tunnel profile data"), nil
	}
	if req.GetId() == "" {
		return tunnelProfileMutationError("missing tunnel profile ID"), nil
	}
	if manager == nil {
		return tunnelProfileMutationError("configuration manager is unavailable"), nil
	}
	if active, err := activeTunnelProfile(service, req.GetId()); err != nil {
		return tunnelProfileMutationError(err.Error()), nil
	} else if active {
		return tunnelProfileMutationError(fmt.Sprintf("cannot update tunnel profile %q while it has an active connection", req.GetId())), nil
	}

	profile := tunnelProfileFromRPC(req.GetData())
	profile.ID = req.GetId()
	if err := manager.UpdateTunnelProfile(profile); err != nil {
		return tunnelProfileMutationError(fmt.Sprintf("cannot update tunnel profile %q: %v", req.GetId(), err)), nil
	}
	data, err := resolvedProfileToRPC(manager, profile)
	if err != nil {
		return tunnelProfileMutationError(fmt.Sprintf("updated tunnel profile but could not resolve it: %v", err)), nil
	}
	return &rpc.TunnelProfileMutationResponse{Status: rpc.ResponseStatus_Success, Message: fmt.Sprintf("successfully updated tunnel profile %s", profile.Name), Data: data}, nil
}

func DeleteTunnelProfileTask(_ context.Context, req *rpc.DeleteTunnelProfileRequest, manager configmanager.ConfigManager, service tunnelmanager.TunnelService) (*rpc.TunnelProfileMutationResponse, error) {
	if req == nil || req.GetId() == "" {
		return tunnelProfileMutationError("missing tunnel profile ID"), nil
	}
	if manager == nil {
		return tunnelProfileMutationError("configuration manager is unavailable"), nil
	}
	if active, err := activeTunnelProfile(service, req.GetId()); err != nil {
		return tunnelProfileMutationError(err.Error()), nil
	} else if active {
		return tunnelProfileMutationError(fmt.Sprintf("cannot delete tunnel profile %q while it has an active connection", req.GetId())), nil
	}

	profile, err := manager.GetTunnelProfile(req.GetId())
	if err != nil {
		return tunnelProfileMutationError(fmt.Sprintf("cannot delete tunnel profile %q: %v", req.GetId(), err)), nil
	}
	data := tunnelProfileToRPC(profile, nil)
	if machine, err := manager.GetMachine(profile.MachineID); err == nil {
		data.Machine = machineToRPC(machine)
	}
	if err := manager.RemoveTunnelProfile(req.GetId()); err != nil {
		return tunnelProfileMutationError(fmt.Sprintf("cannot delete tunnel profile %q: %v", req.GetId(), err)), nil
	}
	return &rpc.TunnelProfileMutationResponse{Status: rpc.ResponseStatus_Success, Message: fmt.Sprintf("successfully deleted tunnel profile %s", profile.Name), Data: data}, nil
}

func machineFromRPC(machine *rpc.Machine) configmanager.Machine {
	return configmanager.Machine{ID: machine.GetId(), Name: machine.GetName(), Server: machine.GetServer(), User: machine.GetUser(), KeyFile: machine.GetKeyFile()}
}

func machineToRPC(machine configmanager.Machine) *rpc.Machine {
	return &rpc.Machine{Id: machine.ID, Name: machine.Name, Server: machine.Server, User: machine.User, KeyFile: machine.KeyFile}
}

func tunnelProfileFromRPC(profile *rpc.TunnelProfile) configmanager.TunnelProfile {
	return configmanager.TunnelProfile{ID: profile.GetId(), Name: profile.GetName(), Description: profile.GetDescription(), MachineID: profile.GetMachineId(), RemoteHost: profile.GetRemoteHost(), RemotePort: int(profile.GetRemotePort()), LocalPort: int(profile.GetLocalPort())}
}

func tunnelProfileToRPC(profile configmanager.TunnelProfile, machine *rpc.Machine) *rpc.TunnelProfile {
	return &rpc.TunnelProfile{Id: profile.ID, Name: profile.Name, Description: profile.Description, MachineId: profile.MachineID, RemoteHost: profile.RemoteHost, RemotePort: int32(profile.RemotePort), LocalPort: int32(profile.LocalPort), Machine: machine}
}

func resolvedProfileToRPC(manager configmanager.ConfigManager, profile configmanager.TunnelProfile) (*rpc.TunnelProfile, error) {
	machine, err := manager.GetMachine(profile.MachineID)
	if err != nil {
		return nil, err
	}
	return tunnelProfileToRPC(profile, machineToRPC(machine)), nil
}

func machineByName(manager configmanager.ConfigManager, name string) (configmanager.Machine, error) {
	machines, err := manager.GetMachines()
	if err != nil {
		return configmanager.Machine{}, err
	}
	for _, machine := range machines {
		if machine.Name == name {
			return machine, nil
		}
	}
	return configmanager.Machine{}, fmt.Errorf("machine %q was not found", name)
}

func tunnelProfileByName(manager configmanager.ConfigManager, name string) (configmanager.TunnelProfile, error) {
	profiles, err := manager.GetTunnelProfiles()
	if err != nil {
		return configmanager.TunnelProfile{}, err
	}
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, nil
		}
	}
	return configmanager.TunnelProfile{}, fmt.Errorf("tunnel profile %q was not found", name)
}

func activeMachine(service tunnelmanager.TunnelService, machineID string) (bool, error) {
	if service == nil || service.GetManager() == nil {
		return false, fmt.Errorf("tunnel service is unavailable")
	}
	for _, connection := range service.GetManager().ConnectionsSnapshot() {
		if connection.Config.MachineID == machineID {
			return true, nil
		}
	}
	return false, nil
}

func activeTunnelProfile(service tunnelmanager.TunnelService, profileID string) (bool, error) {
	if service == nil || service.GetManager() == nil {
		return false, fmt.Errorf("tunnel service is unavailable")
	}
	for _, connection := range service.GetManager().ConnectionsSnapshot() {
		if connection.Config.ID == profileID {
			return true, nil
		}
	}
	return false, nil
}

func machineMutationError(message string) *rpc.MachineMutationResponse {
	return &rpc.MachineMutationResponse{Status: rpc.ResponseStatus_Error, Message: message}
}

func tunnelProfileMutationError(message string) *rpc.TunnelProfileMutationResponse {
	return &rpc.TunnelProfileMutationResponse{Status: rpc.ResponseStatus_Error, Message: message}
}
