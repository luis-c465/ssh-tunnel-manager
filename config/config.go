package config

import "os"

var AppVersion = "1.1.5"

const DefaultSSHPort = "22"

const Host = "127.0.0.1"

const Port = "50051"

const Address = Host + ":" + Port

const ConfigDirFlagName = "config-dir"

const ConfigDirEnvironment = "SSHTM_CONFIG_DIR"

const DefaultConfigDir = "~/.ssh-tunnel-manager"

// ConfigurationDir returns the configured persistence directory. The legacy
// environment name remains supported for backward compatibility and tests.
func ConfigurationDir() string {
	if dir := os.Getenv(ConfigDirEnvironment); dir != "" {
		return dir
	}
	if dir := os.Getenv(ConfigDirFlagName); dir != "" {
		return dir
	}
	return DefaultConfigDir
}

const ActiveTunnelsFile = "active_tunnels.json"
