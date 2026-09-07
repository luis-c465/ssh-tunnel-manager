package tunnelmanager

import (
	"context"

	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
)

// ConnectionSnapshot is an immutable view of a managed tunnel.
type ConnectionSnapshot struct {
	LocalAddr  string
	RemoteAddr string
	Config     configmanager.Entry
}

// TunnelManager defines the low-level operations for managing individual SSH tunnels.
type TunnelManager interface {
	StartTunneling(ctx context.Context, entry configmanager.Entry, localPort int, resultChan chan<- string, errChan chan<- error)
	RegisterConnection(localPort int, connection *ConnectionInfo) bool
	StopTunneling(localPort int) bool
	SaveActiveTunnels(path string) error
	ConnectionsSnapshot() map[int]ConnectionSnapshot
	GetConnection(localPort int) (ConnectionSnapshot, bool)
	Shutdown()
}

// TunnelService defines the high-level operations for managing tunnels and their persistence.
type TunnelService interface {
	StartTunnel(ctx context.Context, configName string, localPort int32) (string, error)
	StopTunnel(ctx context.Context, configName string, localPort int32) (string, error)
	ListActiveTunnels(ctx context.Context) ([]ActiveTunnel, error)
	RestoreTunnels(ctx context.Context) error
	PersistTunnels() error
	GetManager() TunnelManager
}
