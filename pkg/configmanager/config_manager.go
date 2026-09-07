package configmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/besrabasant/ssh-tunnel-manager/config"
	"github.com/besrabasant/ssh-tunnel-manager/utils"
)

// Entry is an SSH configuration entry
type Entry struct {
	Name        string
	Description string
	Server      string
	User        string
	KeyFile     string
	RemoteHost  string
	RemotePort  int
	LocalPort   int
}

type Entries []Entry

func (e Entries) Filter(predicate func(*Entry) bool) Entries {
	newEntries := make([]Entry, 0)

	for _, entry := range e {
		entry := entry
		if predicate(&entry) {
			newEntries = append(newEntries, entry)
		}
	}

	return newEntries
}

func (e *Entry) Validate() error {
	var validationErrors []string

	if err := validateConfigurationName(e.Name); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"Server", e.Server},
		{"User", e.User},
		{"KeyFile", e.KeyFile},
		{"RemoteHost", e.RemoteHost},
	} {
		if strings.TrimSpace(field.value) == "" {
			validationErrors = append(validationErrors, field.name+" is required.")
		}
	}
	if e.RemotePort < 1 || e.RemotePort > 65535 {
		validationErrors = append(validationErrors, "RemotePort must be between 1 and 65535.")
	}
	if e.LocalPort < 0 || e.LocalPort > 65535 {
		validationErrors = append(validationErrors, "LocalPort must be between 0 and 65535.")
	}

	if len(validationErrors) == 0 {
		return nil
	}
	return fmt.Errorf("entry is not valid: %s", strings.Join(validationErrors, " "))
}

func validateConfigurationName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("Name is required.")
	}
	if len(name) > 128 {
		return errors.New("Name must not exceed 128 characters.")
	}
	if strings.TrimSpace(name) != name || name == "." || name == ".." || filepath.Base(name) != name || strings.Contains(name, "\\") || strings.ContainsRune(name, '\x00') {
		return fmt.Errorf("Name %q is not safe", name)
	}
	return nil
}

// ConfigManager manages SSH entries
type ConfigManager interface {
	GetConfiguration(entryName string) (Entry, error)
	GetConfigurations() ([]Entry, error)
	AddConfiguration(entry Entry) error
	UpdateConfiguration(entry Entry) error
	RemoveConfiguration(entryName string) error
}

type manager struct {
	// dir is a path to the directory containing the configurations.
	dir     string
	initErr error
}

// NewManager retains the original API for compatibility. New code should use
// NewManagerWithError so initialization failures can be handled explicitly.
// A manager returned after failed initialization reports that error from every
// operation rather than panicking or returning a nil interface.
func NewManager(dir string) ConfigManager {
	m, err := NewManagerWithError(dir)
	if err != nil {
		return &manager{initErr: err}
	}
	return m
}

// NewManagerWithError creates a manager and reports directory resolution or setup errors.
func NewManagerWithError(dir string) (ConfigManager, error) {
	resolvedDir, err := utils.ResolveDir(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve configuration directory %q: %w", dir, err)
	}

	m := &manager{dir: resolvedDir}
	if err := m.ensurePersistenceDirExists(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *manager) ensurePersistenceDirExists() error {
	if err := os.MkdirAll(m.dir, 0700); err != nil {
		return fmt.Errorf("create configuration directory %q: %w", m.dir, err)
	}
	info, err := os.Stat(m.dir)
	if err != nil {
		return fmt.Errorf("stat configuration directory %q: %w", m.dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("configuration path %q is not a directory", m.dir)
	}
	if err := os.Chmod(m.dir, 0700); err != nil {
		return fmt.Errorf("set permissions on configuration directory %q: %w", m.dir, err)
	}
	return nil
}

func (m *manager) AddConfiguration(entry Entry) error {
	if m.initErr != nil {
		return m.initErr
	}
	if err := entry.Validate(); err != nil {
		return err
	}
	return m.writeConfiguration(entry)
}

func (m *manager) GetConfigurations() ([]Entry, error) {
	if m.initErr != nil {
		return nil, m.initErr
	}

	entries := make([]Entry, 0)
	files, err := os.ReadDir(m.dir)
	if err != nil {
		return entries, fmt.Errorf("read configuration directory %q: %w", m.dir, err)
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") || file.Name() == config.ActiveTunnelsFile {
			continue
		}

		byteValue, err := os.ReadFile(filepath.Join(m.dir, file.Name()))
		if err != nil {
			return entries, fmt.Errorf("read configuration file %q: %w", file.Name(), err)
		}

		var entry Entry
		if err := json.Unmarshal(byteValue, &entry); err != nil {
			return []Entry{}, fmt.Errorf("parse configuration file %q: %w", file.Name(), err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (m *manager) RemoveConfiguration(entryName string) error {
	if m.initErr != nil {
		return m.initErr
	}
	if err := validateConfigurationName(entryName); err != nil {
		return err
	}
	filename := filepath.Join(m.dir, entryName+".json")
	if err := os.Remove(filename); err != nil {
		return fmt.Errorf("remove configuration file %q: %w", filename, err)
	}
	return nil
}

func (m *manager) GetConfiguration(entryName string) (Entry, error) {
	if m.initErr != nil {
		return Entry{}, m.initErr
	}
	if err := validateConfigurationName(entryName); err != nil {
		return Entry{}, err
	}
	filename := filepath.Join(m.dir, entryName+".json")
	byteValue, err := os.ReadFile(filename)
	if err != nil {
		return Entry{}, fmt.Errorf("read configuration file %q: %w", filename, err)
	}

	var entry Entry
	if err := json.Unmarshal(byteValue, &entry); err != nil {
		return Entry{}, fmt.Errorf("parse configuration file %q: %w", filename, err)
	}
	return entry, nil
}

func (m *manager) UpdateConfiguration(entry Entry) error {
	if m.initErr != nil {
		return m.initErr
	}
	if err := entry.Validate(); err != nil {
		return err
	}
	return m.writeConfiguration(entry)
}

func (m *manager) writeConfiguration(entry Entry) error {
	file, err := json.MarshalIndent(entry, "", " ")
	if err != nil {
		return fmt.Errorf("marshal configuration %q: %w", entry.Name, err)
	}

	tempFile, err := os.CreateTemp(m.dir, "."+entry.Name+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary configuration file: %w", err)
	}
	tempName := tempFile.Name()
	defer os.Remove(tempName)

	if err := tempFile.Chmod(0600); err != nil {
		tempFile.Close()
		return fmt.Errorf("set permissions on temporary configuration file: %w", err)
	}
	if _, err := tempFile.Write(file); err != nil {
		tempFile.Close()
		return fmt.Errorf("write temporary configuration file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary configuration file: %w", err)
	}

	filename := filepath.Join(m.dir, entry.Name+".json")
	if err := os.Rename(tempName, filename); err != nil {
		return fmt.Errorf("replace configuration file %q: %w", filename, err)
	}
	return nil
}
