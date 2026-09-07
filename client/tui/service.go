package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/besrabasant/ssh-tunnel-manager/client/lib"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	pb "github.com/besrabasant/ssh-tunnel-manager/rpc"
)

type Active struct {
	Name      string
	LocalPort int
}

func newDaemonClient() (pb.DaemonServiceClient, func(), context.Context, context.CancelFunc, error) {
	client, cleanup, err := lib.CreateDaemonServiceClient()
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("rpc connect: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	return client, cleanup, ctx, cancel, nil
}

func machineFromRPC(machine *pb.Machine) configmanager.Machine {
	if machine == nil {
		return configmanager.Machine{}
	}
	return configmanager.Machine{
		ID:      machine.GetId(),
		Name:    machine.GetName(),
		Server:  machine.GetServer(),
		User:    machine.GetUser(),
		KeyFile: machine.GetKeyFile(),
	}
}

func machineToRPC(machine configmanager.Machine) *pb.Machine {
	return &pb.Machine{
		Id:      machine.ID,
		Name:    machine.Name,
		Server:  machine.Server,
		User:    machine.User,
		KeyFile: machine.KeyFile,
	}
}

// entryFromRPC resolves a profile into the legacy view used by the TUI. A missing
// resolved machine is represented by empty SSH fields so a stale profile is safe to display.
func entryFromRPC(profile *pb.TunnelProfile) configmanager.Entry {
	machine := machineFromRPC(profile.GetMachine())
	machineID := profile.GetMachineId()
	if machineID == "" {
		machineID = machine.ID
	}
	return configmanager.Entry{
		ID:          profile.GetId(),
		MachineID:   machineID,
		Name:        profile.GetName(),
		Description: profile.GetDescription(),
		Server:      machine.Server,
		User:        machine.User,
		KeyFile:     machine.KeyFile,
		RemoteHost:  profile.GetRemoteHost(),
		RemotePort:  int(profile.GetRemotePort()),
		LocalPort:   int(profile.GetLocalPort()),
	}
}

func profileToRPC(profile configmanager.TunnelProfile) *pb.TunnelProfile {
	return &pb.TunnelProfile{
		Id:          profile.ID,
		Name:        profile.Name,
		Description: profile.Description,
		MachineId:   profile.MachineID,
		RemoteHost:  profile.RemoteHost,
		RemotePort:  int32(profile.RemotePort),
		LocalPort:   int32(profile.LocalPort),
	}
}

func LoadMachines() ([]configmanager.Machine, error) {
	client, cleanup, ctx, cancel, err := newDaemonClient()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer cancel()

	response, err := client.ListMachines(ctx, &pb.ListMachinesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list machines rpc: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("list machines rpc: empty response")
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return nil, fmt.Errorf("%s", response.GetMessage())
	}

	machines := make([]configmanager.Machine, 0, len(response.GetMachines()))
	for _, machine := range response.GetMachines() {
		machines = append(machines, machineFromRPC(machine))
	}
	return machines, nil
}

func LoadTunnelProfiles() ([]configmanager.Entry, error) {
	client, cleanup, ctx, cancel, err := newDaemonClient()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	defer cancel()

	response, err := client.ListTunnelProfiles(ctx, &pb.ListTunnelProfilesRequest{})
	if err != nil {
		return nil, fmt.Errorf("list tunnel profiles rpc: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("list tunnel profiles rpc: empty response")
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return nil, fmt.Errorf("%s", response.GetMessage())
	}

	profiles := make([]configmanager.Entry, 0, len(response.GetTunnelProfiles()))
	for _, profile := range response.GetTunnelProfiles() {
		profiles = append(profiles, entryFromRPC(profile))
	}
	return profiles, nil
}

func AddMachine(machine configmanager.Machine) error {
	if err := machine.Validate(); err != nil {
		return err
	}
	client, cleanup, ctx, cancel, err := newDaemonClient()
	if err != nil {
		return err
	}
	defer cleanup()
	defer cancel()

	response, err := client.AddMachine(ctx, &pb.MachineMutationRequest{Data: machineToRPC(machine)})
	if err != nil {
		return fmt.Errorf("add machine rpc: %w", err)
	}
	if response == nil {
		return fmt.Errorf("add machine rpc: empty response")
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return fmt.Errorf("%s", response.GetMessage())
	}
	return nil
}

func UpdateMachine(machine configmanager.Machine) error {
	if err := machine.Validate(); err != nil {
		return err
	}
	client, cleanup, ctx, cancel, err := newDaemonClient()
	if err != nil {
		return err
	}
	defer cleanup()
	defer cancel()

	response, err := client.UpdateMachine(ctx, &pb.MachineMutationRequest{Id: machine.ID, Data: machineToRPC(machine)})
	if err != nil {
		return fmt.Errorf("update machine rpc: %w", err)
	}
	if response == nil {
		return fmt.Errorf("update machine rpc: empty response")
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return fmt.Errorf("%s", response.GetMessage())
	}
	return nil
}

func DeleteMachine(id string) error {
	client, cleanup, ctx, cancel, err := newDaemonClient()
	if err != nil {
		return err
	}
	defer cleanup()
	defer cancel()

	response, err := client.DeleteMachine(ctx, &pb.DeleteMachineRequest{Id: id})
	if err != nil {
		return fmt.Errorf("delete machine rpc: %w", err)
	}
	if response == nil {
		return fmt.Errorf("delete machine rpc: empty response")
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return fmt.Errorf("%s", response.GetMessage())
	}
	return nil
}

func AddTunnelProfile(profile configmanager.TunnelProfile) error {
	if err := profile.Validate(); err != nil {
		return err
	}
	client, cleanup, ctx, cancel, err := newDaemonClient()
	if err != nil {
		return err
	}
	defer cleanup()
	defer cancel()

	response, err := client.AddTunnelProfile(ctx, &pb.TunnelProfileMutationRequest{Data: profileToRPC(profile)})
	if err != nil {
		return fmt.Errorf("add tunnel profile rpc: %w", err)
	}
	if response == nil {
		return fmt.Errorf("add tunnel profile rpc: empty response")
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return fmt.Errorf("%s", response.GetMessage())
	}
	return nil
}

func UpdateTunnelProfile(profile configmanager.TunnelProfile) error {
	if err := profile.Validate(); err != nil {
		return err
	}
	client, cleanup, ctx, cancel, err := newDaemonClient()
	if err != nil {
		return err
	}
	defer cleanup()
	defer cancel()

	response, err := client.UpdateTunnelProfile(ctx, &pb.TunnelProfileMutationRequest{Id: profile.ID, Data: profileToRPC(profile)})
	if err != nil {
		return fmt.Errorf("update tunnel profile rpc: %w", err)
	}
	if response == nil {
		return fmt.Errorf("update tunnel profile rpc: empty response")
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return fmt.Errorf("%s", response.GetMessage())
	}
	return nil
}

func DeleteTunnelProfile(id string) error {
	client, cleanup, ctx, cancel, err := newDaemonClient()
	if err != nil {
		return err
	}
	defer cleanup()
	defer cancel()

	response, err := client.DeleteTunnelProfile(ctx, &pb.DeleteTunnelProfileRequest{Id: id})
	if err != nil {
		return fmt.Errorf("delete tunnel profile rpc: %w", err)
	}
	if response == nil {
		return fmt.Errorf("delete tunnel profile rpc: empty response")
	}
	if response.GetStatus() == pb.ResponseStatus_Error {
		return fmt.Errorf("%s", response.GetMessage())
	}
	return nil
}

func LoadActive() ([]Active, error) {
	client, cleanup, err := lib.CreateDaemonServiceClient()
	if err != nil {
		return nil, fmt.Errorf("rpc connect: %w", err)
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	response, err := client.ListActiveTunnelsJSON(ctx, &pb.ListActiveTunnelsJSONRequest{})
	if err != nil {
		return nil, fmt.Errorf("list active tunnels (json) rpc: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("list active tunnels (json) rpc: empty response")
	}

	active := make([]Active, 0, len(response.GetTunnels()))
	for _, tunnel := range response.GetTunnels() {
		active = append(active, Active{Name: tunnel.GetName(), LocalPort: int(tunnel.GetLocalPort())})
	}
	return active, nil
}

func StartTunnel(name string, localPort int) (string, error) {
	client, cleanup, err := lib.CreateDaemonServiceClient()
	if err != nil {
		return "", fmt.Errorf("rpc connect: %w", err)
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	response, err := client.StartTunnel(ctx, &pb.StartTunnelRequest{ConfigName: name, LocalPort: int32(localPort)})
	if err != nil {
		return "", fmt.Errorf("start failed: %w", err)
	}
	if response == nil {
		return "", fmt.Errorf("start failed: empty response")
	}
	return response.GetResult(), nil
}

func KillTunnel(name string, localPort int) (string, error) {
	client, cleanup, err := lib.CreateDaemonServiceClient()
	if err != nil {
		return "", fmt.Errorf("rpc connect: %w", err)
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	response, err := client.KillTunnel(ctx, &pb.KillTunnelRequest{ConfigName: name, LocalPort: int32(localPort)})
	if err != nil {
		return "", fmt.Errorf("kill failed: %w", err)
	}
	if response == nil {
		return "", fmt.Errorf("kill failed: empty response")
	}
	return response.GetResult(), nil
}
