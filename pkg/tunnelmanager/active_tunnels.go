package tunnelmanager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ActiveTunnel represents a tunnel that should be restored on restart.
type ActiveTunnel struct {
	ConfigName string `json:"config_name"`
	LocalPort  int    `json:"local_port"`
	LocalAddr  string `json:"local_addr,omitempty"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	Server     string `json:"server,omitempty"`
	User       string `json:"user,omitempty"`
	SavedAt    string `json:"saved_at,omitempty"`
}

// SaveActiveTunnels persists current active tunnels to the given file.
func (m *tunnelManager) SaveActiveTunnels(path string) error {
	tunnels := make([]ActiveTunnel, 0)
	savedAt := time.Now().UTC().Format(time.RFC3339)
	m.Mutex.RLock()
	for port, ci := range m.Connections {
		tunnels = append(tunnels, ActiveTunnel{
			ConfigName: ci.Config.Name,
			LocalPort:  port,
			LocalAddr:  ci.LocalAddr,
			RemoteAddr: ci.RemoteAddr,
			Server:     ci.Config.Server,
			User:       ci.Config.User,
			SavedAt:    savedAt,
		})
	}
	m.Mutex.RUnlock()

	if len(tunnels) == 0 {
		// Removing a missing persistence file is already the desired state.
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	data, err := json.MarshalIndent(tunnels, "", " ")
	if err != nil {
		return fmt.Errorf("marshal active tunnels: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".active-tunnels-*.tmp")
	if err != nil {
		return fmt.Errorf("create active tunnels temp file: %w", err)
	}
	tempName := tempFile.Name()
	defer os.Remove(tempName)
	if err := tempFile.Chmod(0600); err != nil {
		tempFile.Close()
		return fmt.Errorf("set active tunnels permissions: %w", err)
	}
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return fmt.Errorf("write active tunnels temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close active tunnels temp file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace active tunnels file %q: %w", path, err)
	}
	return nil
}

// LoadActiveTunnels reads persisted tunnels from the given file.
func LoadActiveTunnels(path string) ([]ActiveTunnel, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []ActiveTunnel{}, nil
		}
		return nil, err
	}
	var tunnels []ActiveTunnel
	if err := json.Unmarshal(b, &tunnels); err != nil {
		return nil, err
	}
	return tunnels, nil
}
