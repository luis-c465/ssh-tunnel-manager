package tunnelmanager

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
)

func TestConnectionsSnapshotDoesNotExposeMap(t *testing.T) {
	manager := NewTunnelManager().(*tunnelManager)
	manager.RegisterConnection(1234, &ConnectionInfo{Config: configmanager.Entry{Name: "one"}})

	snapshot := manager.ConnectionsSnapshot()
	delete(snapshot, 1234)
	if _, ok := manager.GetConnection(1234); !ok {
		t.Fatal("mutating a snapshot changed manager state")
	}
}

func TestRegisterConnectionRejectsDuplicatePort(t *testing.T) {
	manager := NewTunnelManager().(*tunnelManager)
	first := &ConnectionInfo{Config: configmanager.Entry{Name: "first"}}
	if !manager.RegisterConnection(1234, first) {
		t.Fatal("first registration was rejected")
	}
	if manager.RegisterConnection(1234, &ConnectionInfo{Config: configmanager.Entry{Name: "second"}}) {
		t.Fatal("duplicate registration was accepted")
	}
	got, _ := manager.GetConnection(1234)
	if got.Config.Name != "first" {
		t.Fatalf("duplicate registration replaced connection with %q", got.Config.Name)
	}
}

func TestConnectionsSnapshotConcurrentWithStop(t *testing.T) {
	manager := NewTunnelManager().(*tunnelManager)
	for port := 10000; port < 10100; port++ {
		manager.RegisterConnection(port, &ConnectionInfo{Cancel: func() {}})
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 100; n++ {
				_ = manager.ConnectionsSnapshot()
			}
		}()
	}
	for port := 10000; port < 10100; port++ {
		manager.StopTunneling(port)
	}
	wg.Wait()
}

func TestShutdownIsIdempotent(t *testing.T) {
	manager := NewTunnelManager().(*tunnelManager)
	var calls int
	manager.RegisterConnection(1234, &ConnectionInfo{Cancel: func() { calls++ }})
	manager.Shutdown()
	manager.Shutdown()
	if calls != 1 {
		t.Fatalf("cancel called %d times, want once", calls)
	}
	if len(manager.ConnectionsSnapshot()) != 0 {
		t.Fatal("shutdown did not clear connections")
	}
}

func TestSaveActiveTunnelsUsesPrivatePermissions(t *testing.T) {
	manager := NewTunnelManager().(*tunnelManager)
	manager.RegisterConnection(1234, &ConnectionInfo{Config: configmanager.Entry{Name: "test"}})
	path := filepath.Join(t.TempDir(), "active.json")
	if err := manager.SaveActiveTunnels(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("active tunnel permissions = %o, want 600", got)
	}
}

func TestSaveActiveTunnelsReportsRemoveError(t *testing.T) {
	manager := NewTunnelManager().(*tunnelManager)
	path := filepath.Join(t.TempDir(), "non-empty")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "entry"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := manager.SaveActiveTunnels(path); err == nil {
		t.Fatal("expected remove error")
	}
}
