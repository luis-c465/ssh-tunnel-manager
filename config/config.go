package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var AppVersion = "1.1.5"

const DefaultSSHPort = "22"

const Host = "127.0.0.1"

const Port = "50051"

const Address = Host + ":" + Port

const ConfigDirFlagName = "config-dir"

const ConfigDirEnvironment = "SSHTM_CONFIG_DIR"

const XDGConfigHomeEnvironment = "XDG_CONFIG_HOME"

const DefaultConfigDirName = "sshtm"

const legacyConfigDirName = ".ssh-tunnel-manager"

// ConfigurationDir returns the configured persistence directory. The legacy
// environment name remains supported for backward compatibility and tests.
func ConfigurationDir() string {
	if dir := os.Getenv(ConfigDirEnvironment); dir != "" {
		return dir
	}
	if dir := os.Getenv(ConfigDirFlagName); dir != "" {
		return dir
	}
	return defaultConfigurationDir()
}

// ConfigurationDirOverridden reports whether either supported override is set.
func ConfigurationDirOverridden() bool {
	return os.Getenv(ConfigDirEnvironment) != "" || os.Getenv(ConfigDirFlagName) != ""
}

func defaultConfigurationDir() string {
	if configHome := os.Getenv(XDGConfigHomeEnvironment); configHome != "" {
		return filepath.Join(configHome, DefaultConfigDirName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", DefaultConfigDirName)
	}
	return filepath.Join(home, ".config", DefaultConfigDirName)
}

func legacyConfigurationDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for legacy configuration: %w", err)
	}
	return filepath.Join(home, legacyConfigDirName), nil
}

// MigrateLegacyConfigDir copies the pre-XDG configuration directory into the
// default root. Overrides are never migrated, and a populated target is left
// untouched so configurations are never merged or overwritten.
func MigrateLegacyConfigDir() error {
	if ConfigurationDirOverridden() {
		return nil
	}
	legacyDir, err := legacyConfigurationDir()
	if err != nil {
		return err
	}
	return migrateLegacyConfigDir(legacyDir, ConfigurationDir())
}

func migrateLegacyConfigDir(legacyDir, targetDir string) error {
	legacyInfo, err := os.Stat(legacyDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat legacy configuration directory %q: %w", legacyDir, err)
	}
	if !legacyInfo.IsDir() {
		return fmt.Errorf("legacy configuration path %q is not a directory", legacyDir)
	}

	targetEmpty, err := directoryIsEmpty(targetDir)
	if err == nil && !targetEmpty {
		return nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	parentDir := filepath.Dir(targetDir)
	if err := os.MkdirAll(parentDir, 0700); err != nil {
		return fmt.Errorf("create configuration parent directory %q: %w", parentDir, err)
	}
	stagingDir, err := os.MkdirTemp(parentDir, ".sshtm-migrate-*")
	if err != nil {
		return fmt.Errorf("create migration staging directory: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	if err := copyDirectory(legacyDir, stagingDir); err != nil {
		return fmt.Errorf("copy legacy configuration: %w", err)
	}
	if err := os.Chmod(stagingDir, 0700); err != nil {
		return fmt.Errorf("set permissions on migrated configuration directory: %w", err)
	}

	if targetEmpty {
		if err := os.Remove(targetDir); err != nil {
			return fmt.Errorf("replace empty configuration directory %q: %w", targetDir, err)
		}
	}
	if err := os.Rename(stagingDir, targetDir); err != nil {
		return fmt.Errorf("install migrated configuration directory %q: %w", targetDir, err)
	}
	return nil
}

// directoryIsEmpty reports whether path exists and is empty. A non-directory
// path is treated as a migration conflict and is never replaced.
func directoryIsEmpty(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("read configuration directory %q: %w", path, err)
	}
	return len(entries) == 0, nil
}

func copyDirectory(sourceDir, destinationDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("read directory %q: %w", sourceDir, err)
	}
	for _, entry := range entries {
		if err := copyEntry(filepath.Join(sourceDir, entry.Name()), filepath.Join(destinationDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyEntry(sourcePath, destinationPath string) error {
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return fmt.Errorf("stat %q: %w", sourcePath, err)
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(sourcePath)
		if err != nil {
			return fmt.Errorf("read symbolic link %q: %w", sourcePath, err)
		}
		if err := os.Symlink(target, destinationPath); err != nil {
			return fmt.Errorf("copy symbolic link %q: %w", sourcePath, err)
		}
	case info.IsDir():
		if err := os.Mkdir(destinationPath, 0700); err != nil {
			return fmt.Errorf("create directory %q: %w", destinationPath, err)
		}
		if err := copyDirectory(sourcePath, destinationPath); err != nil {
			return err
		}
	case info.Mode().IsRegular():
		if err := copyFile(sourcePath, destinationPath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported file type %q", sourcePath)
	}
	return nil
}

func copyFile(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open %q: %w", sourcePath, err)
	}
	defer source.Close()

	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create %q: %w", destinationPath, err)
	}
	if _, err := io.Copy(destination, source); err != nil {
		destination.Close()
		return fmt.Errorf("copy %q: %w", sourcePath, err)
	}
	if err := destination.Close(); err != nil {
		return fmt.Errorf("close %q: %w", destinationPath, err)
	}
	return nil
}

const ActiveTunnelsFile = "active_tunnels.json"
