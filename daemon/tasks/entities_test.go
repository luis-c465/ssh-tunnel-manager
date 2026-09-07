package tasks

import (
	"context"
	"strings"
	"testing"

	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/tunnelmanager"
	"github.com/besrabasant/ssh-tunnel-manager/rpc"
)

func TestListTunnelProfilesTaskIncludesResolvedMachine(t *testing.T) {
	manager, err := configmanager.NewManagerWithError(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.AddMachine(configmanager.Machine{Name: "production", Server: "ssh.example.com:22", User: "root", KeyFile: "/tmp/id_ed25519"}); err != nil {
		t.Fatal(err)
	}
	machines, err := manager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.AddTunnelProfile(configmanager.TunnelProfile{Name: "database", MachineID: machines[0].ID, RemoteHost: "127.0.0.1", RemotePort: 5432, LocalPort: 15432}); err != nil {
		t.Fatal(err)
	}

	response, err := ListTunnelProfilesTask(context.Background(), &rpc.ListTunnelProfilesRequest{}, manager)
	if err != nil {
		t.Fatal(err)
	}
	if response.GetStatus() != rpc.ResponseStatus_Success {
		t.Fatalf("expected success, got %v: %s", response.GetStatus(), response.GetMessage())
	}
	if len(response.GetTunnelProfiles()) != 1 {
		t.Fatalf("expected one tunnel profile, got %d", len(response.GetTunnelProfiles()))
	}
	profile := response.GetTunnelProfiles()[0]
	if profile.GetMachine() == nil || profile.GetMachine().GetId() != machines[0].ID {
		t.Fatalf("expected resolved machine %q, got %#v", machines[0].ID, profile.GetMachine())
	}
}

func TestDeleteMachineTaskRejectsReferencedMachine(t *testing.T) {
	dir := t.TempDir()
	manager, err := configmanager.NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.AddMachine(configmanager.Machine{Name: "production", Server: "ssh.example.com:22", User: "root", KeyFile: "/tmp/id_ed25519"}); err != nil {
		t.Fatal(err)
	}
	machines, err := manager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.AddTunnelProfile(configmanager.TunnelProfile{Name: "database", MachineID: machines[0].ID, RemoteHost: "127.0.0.1", RemotePort: 5432, LocalPort: 15432}); err != nil {
		t.Fatal(err)
	}
	service := tunnelmanager.NewTunnelService(tunnelmanager.NewTunnelManager(), manager, dir)

	response, err := DeleteMachineTask(context.Background(), &rpc.DeleteMachineRequest{Id: machines[0].ID}, manager, service)
	if err != nil {
		t.Fatal(err)
	}
	if response.GetStatus() != rpc.ResponseStatus_Error {
		t.Fatalf("expected error status, got %v", response.GetStatus())
	}
	if !strings.Contains(response.GetMessage(), "referenced") {
		t.Fatalf("expected reference error, got %q", response.GetMessage())
	}
}
