package tui

import (
	"testing"

	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
)

func TestActiveProfilesIgnoresOneOffTunnel(t *testing.T) {
	configs := []configmanager.Entry{{ID: "profile-id", Name: "database"}}
	active := []Active{
		{Name: "one-off on server", IsOneOff: true, LocalPort: 15432},
		{Name: "database", ProfileID: "profile-id", LocalPort: 25432},
	}

	profiles := activeProfiles(configs, active)
	if got := profiles["profile-id"]; got != 25432 {
		t.Fatalf("active local port = %d, want 25432", got)
	}
}
