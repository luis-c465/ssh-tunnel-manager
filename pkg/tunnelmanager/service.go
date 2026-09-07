package tunnelmanager

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"

	"github.com/besrabasant/ssh-tunnel-manager/config"
	"github.com/besrabasant/ssh-tunnel-manager/pkg/configmanager"
	"github.com/besrabasant/ssh-tunnel-manager/utils"
)

type tunnelService struct {
	manager   TunnelManager
	cfgMgr    configmanager.ConfigManager
	configDir string
	lifetime  context.Context
	persistMu sync.Mutex
}

func NewTunnelService(manager TunnelManager, cfgMgr configmanager.ConfigManager, configDir string) TunnelService {
	return &tunnelService{manager: manager, cfgMgr: cfgMgr, configDir: configDir, lifetime: context.Background()}
}

func (s *tunnelService) StartTunnel(ctx context.Context, configName string, localPort int32) (string, error) {
	cfg, err := s.cfgMgr.GetConfiguration(configName)
	if err != nil {
		return "", fmt.Errorf("couldn't get configuration %q: %w", configName, err)
	}
	actualPort := localPort
	if actualPort == -1 {
		actualPort = int32(cfg.LocalPort)
	}
	if actualPort == 0 {
		port, err := s.generateRandomPort()
		if err != nil {
			return "", fmt.Errorf("failed to generate random port: %w", err)
		}
		actualPort = int32(port)
	}
	if _, exists := s.manager.GetConnection(int(actualPort)); exists {
		return fmt.Sprintf("\nCannot start tunnel as connection is already open on port %d\n", actualPort), nil
	}
	resultChan, errChan := make(chan string, 16), make(chan error, 1)
	// The request context bounds setup. Once setup succeeds, the manager-owned
	// tunnel context keeps the established tunnel alive beyond the RPC.
	go s.manager.StartTunneling(ctx, cfg, int(actualPort), resultChan, errChan)
	var output strings.Builder
	var tunnelErr error
	for resultChan != nil || errChan != nil {
		select {
		case result, ok := <-resultChan:
			if !ok {
				resultChan = nil
				continue
			}
			output.WriteString(result)
			output.WriteByte('\n')
		case startErr, ok := <-errChan:
			if !ok {
				errChan = nil
				continue
			}
			if startErr != nil {
				tunnelErr = startErr
				output.WriteString(fmt.Sprintf("Failed to start tunneling: %v\n", startErr))
			}
		case <-ctx.Done():
			return output.String(), ctx.Err()
		}
	}
	if err := s.PersistTunnels(); err != nil {
		return output.String(), err
	}
	if tunnelErr != nil {
		return output.String(), fmt.Errorf("start tunnel %q: %w", configName, tunnelErr)
	}
	return output.String(), nil
}

func (s *tunnelService) StopTunnel(ctx context.Context, configName string, localPort int32) (string, error) {
	var connPort int
	var info ConnectionSnapshot
	var found bool
	if configName != "" {
		for port, candidate := range s.manager.ConnectionsSnapshot() {
			if candidate.Config.Name == configName {
				connPort, info, found = port, candidate, true
				break
			}
		}
	} else {
		connPort = int(localPort)
		info, found = s.manager.GetConnection(connPort)
	}
	if !found {
		if configName != "" {
			return fmt.Sprintf("Did not find any connection for configuration %s", configName), nil
		}
		return fmt.Sprintf("Did not find any connection on port %d", localPort), nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if !s.manager.StopTunneling(connPort) {
		return fmt.Sprintf("Did not find any connection on port %d", connPort), nil
	}
	output := fmt.Sprintf("\nClosing existing connection on port %d for %s\n", connPort, info.Config.Name)
	if err := s.PersistTunnels(); err != nil {
		return output, err
	}
	return output, nil
}

func (s *tunnelService) ListActiveTunnels(ctx context.Context) ([]ActiveTunnel, error) {
	tunnels := make([]ActiveTunnel, 0)
	for port, ci := range s.manager.ConnectionsSnapshot() {
		tunnels = append(tunnels, ActiveTunnel{ConfigName: ci.Config.Name, LocalPort: port, LocalAddr: ci.LocalAddr, RemoteAddr: ci.RemoteAddr, Server: ci.Config.Server, User: ci.Config.User})
	}
	return tunnels, nil
}

func (s *tunnelService) RestoreTunnels(ctx context.Context) error {
	configdir, err := utils.ResolveDir(s.configDir)
	if err != nil {
		return err
	}
	tunnels, err := LoadActiveTunnels(filepath.Join(configdir, config.ActiveTunnelsFile))
	if err != nil {
		return err
	}
	for _, t := range tunnels {
		go s.StartTunnel(s.lifetime, t.ConfigName, int32(t.LocalPort))
	}
	return nil
}
func (s *tunnelService) PersistTunnels() error {
	s.persistMu.Lock()
	defer s.persistMu.Unlock()

	configdir, err := utils.ResolveDir(s.configDir)
	if err != nil {
		return err
	}
	return s.manager.SaveActiveTunnels(filepath.Join(configdir, config.ActiveTunnelsFile))
}
func (s *tunnelService) GetManager() TunnelManager { return s.manager }
func (s *tunnelService) generateRandomPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return 0, err
	}
	return net.LookupPort("tcp", port)
}
