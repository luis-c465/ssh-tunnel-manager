package tui

import (
	"testing"

	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"github.com/rivo/tview"
)

func TestProfileFormKeepsMachineSelectionSeparate(t *testing.T) {
	machines := []configmanager.Machine{
		{ID: "machine-a", Name: "A"},
		{ID: "machine-b", Name: "B"},
	}
	initial := configmanager.Entry{
		ID:         "profile-1",
		MachineID:  "machine-b",
		Name:       "database",
		RemoteHost: "db.internal",
		RemotePort: 5432,
		LocalPort:  15432,
	}

	form, collect := buildProfileForm(initial, machines, true)
	for index := 0; index < form.GetFormItemCount(); index++ {
		switch form.GetFormItem(index).GetLabel() {
		case "Server (host[:port])", "User", "KeyFile":
			t.Fatalf("profile form contains machine field %q", form.GetFormItem(index).GetLabel())
		}
	}

	profile, err := collect()
	if err != nil {
		t.Fatal(err)
	}
	if profile.MachineID != "machine-b" {
		t.Fatalf("machine selection = %q, want machine-b", profile.MachineID)
	}
	if profile.ID != initial.ID {
		t.Fatalf("profile ID = %q, want %q", profile.ID, initial.ID)
	}
}

func TestProfileFormRequiresMachine(t *testing.T) {
	_, collect := buildProfileForm(configmanager.Entry{
		Name:       "database",
		RemoteHost: "db.internal",
		RemotePort: 5432,
	}, nil, false)

	if _, err := collect(); err == nil {
		t.Fatal("expected profile form without a machine to fail")
	}
}

func TestOneOffTunnelFormPreservesEmptyLocalHost(t *testing.T) {
	form, collect := buildOneOffTunnelForm()
	form.GetFormItem(1).(*tview.InputField).SetText("5432")

	localHost, remotePort, localPort, err := collect()
	if err != nil {
		t.Fatal(err)
	}
	if localHost != "" {
		t.Fatalf("local host = %q, want empty", localHost)
	}
	if remotePort != 5432 || localPort != 5432 {
		t.Fatalf("tunnel = (%d, %d), want (5432, 5432)", remotePort, localPort)
	}
}
