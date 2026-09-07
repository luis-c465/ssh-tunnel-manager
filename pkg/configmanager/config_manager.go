package configmanager

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/besrabasant/ssh-tunnel-manager/config"
	"github.com/besrabasant/ssh-tunnel-manager/utils"
)

const (
	machinesDirectory = "machines"
	profilesDirectory = "profiles"
	schemaMarkerFile  = "schema-version.json"
	schemaVersion     = 1
)

var configMu sync.Mutex

// Machine contains the SSH connection settings shared by one or more tunnel profiles.
type Machine struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Server  string `json:"server"`
	User    string `json:"user"`
	KeyFile string `json:"keyFile"`
}

// TunnelProfile contains the per-tunnel settings for a Machine.
type TunnelProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MachineID   string `json:"machineId"`
	RemoteHost  string `json:"remoteHost"`
	RemotePort  int    `json:"remotePort"`
	LocalPort   int    `json:"localPort"`
}

// Entry is the legacy resolved SSH configuration view.
type Entry struct {
	ID          string `json:"id,omitempty"`
	MachineID   string `json:"machineId,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Server      string `json:"server"`
	User        string `json:"user"`
	KeyFile     string `json:"keyFile"`
	RemoteHost  string `json:"remoteHost"`
	RemotePort  int    `json:"remotePort"`
	LocalPort   int    `json:"localPort"`
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

func (m *Machine) Validate() error {
	var validationErrors []string
	if strings.TrimSpace(m.Name) == "" {
		validationErrors = append(validationErrors, "Name is required.")
	}
	for _, field := range []struct{ name, value string }{
		{"Server", m.Server}, {"User", m.User}, {"KeyFile", m.KeyFile},
	} {
		if strings.TrimSpace(field.value) == "" {
			validationErrors = append(validationErrors, field.name+" is required.")
		}
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return fmt.Errorf("machine is not valid: %s", strings.Join(validationErrors, " "))
}

func (p *TunnelProfile) Validate() error {
	var validationErrors []string
	if err := validateConfigurationName(p.Name); err != nil {
		validationErrors = append(validationErrors, err.Error())
	}
	if strings.TrimSpace(p.MachineID) == "" {
		validationErrors = append(validationErrors, "MachineID is required.")
	}
	if strings.TrimSpace(p.RemoteHost) == "" {
		validationErrors = append(validationErrors, "RemoteHost is required.")
	}
	if p.RemotePort < 1 || p.RemotePort > 65535 {
		validationErrors = append(validationErrors, "RemotePort must be between 1 and 65535.")
	}
	if p.LocalPort < 0 || p.LocalPort > 65535 {
		validationErrors = append(validationErrors, "LocalPort must be between 0 and 65535.")
	}
	if len(validationErrors) == 0 {
		return nil
	}
	return fmt.Errorf("tunnel profile is not valid: %s", strings.Join(validationErrors, " "))
}

func (e *Entry) Validate() error {
	machine := Machine{Name: "legacy", Server: e.Server, User: e.User, KeyFile: e.KeyFile}
	profile := TunnelProfile{Name: e.Name, MachineID: "legacy", RemoteHost: e.RemoteHost, RemotePort: e.RemotePort, LocalPort: e.LocalPort}
	var validationErrors []string
	if err := machine.Validate(); err != nil {
		validationErrors = append(validationErrors, strings.TrimPrefix(err.Error(), "machine is not valid: "))
	}
	if err := profile.Validate(); err != nil {
		validationErrors = append(validationErrors, strings.TrimPrefix(err.Error(), "tunnel profile is not valid: "))
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

// ConfigManager manages reusable machines and tunnel profiles. The Entry methods
// remain for callers that use the original flat configuration API.
type ConfigManager interface {
	GetConfiguration(entryName string) (Entry, error)
	GetConfigurations() ([]Entry, error)
	AddConfiguration(entry Entry) error
	UpdateConfiguration(entry Entry) error
	RemoveConfiguration(entryName string) error

	GetMachine(id string) (Machine, error)
	GetMachines() ([]Machine, error)
	AddMachine(machine Machine) error
	UpdateMachine(machine Machine) error
	RemoveMachine(id string) error
	GetTunnelProfile(id string) (TunnelProfile, error)
	GetTunnelProfiles() ([]TunnelProfile, error)
	AddTunnelProfile(profile TunnelProfile) error
	UpdateTunnelProfile(profile TunnelProfile) error
	RemoveTunnelProfile(id string) error
	ResolveTunnelProfile(id string) (Entry, error)
}

type manager struct {
	dir     string
	initErr error
}

// NewManager retains the original API for compatibility.
func NewManager(dir string) ConfigManager {
	m, err := NewManagerWithError(dir)
	if err != nil {
		return &manager{initErr: err}
	}
	return m
}

func NewManagerWithError(dir string) (ConfigManager, error) {
	resolvedDir, err := utils.ResolveDir(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve configuration directory %q: %w", dir, err)
	}
	m := &manager{dir: resolvedDir}
	if err := m.ensurePersistenceDirExists(); err != nil {
		return nil, err
	}
	if err := m.migrateLegacy(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *manager) ensurePersistenceDirExists() error {
	for _, dir := range []string{m.dir, m.machinesPath(), m.profilesPath()} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("create configuration directory %q: %w", dir, err)
		}
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("stat configuration directory %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("configuration path %q is not a directory", dir)
		}
		if err := os.Chmod(dir, 0700); err != nil {
			return fmt.Errorf("set permissions on configuration directory %q: %w", dir, err)
		}
	}
	return nil
}

func (m *manager) machinesPath() string { return filepath.Join(m.dir, machinesDirectory) }
func (m *manager) profilesPath() string { return filepath.Join(m.dir, profilesDirectory) }

func (m *manager) AddConfiguration(entry Entry) error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.addConfigurationUnlocked(entry)
}

func (m *manager) addConfigurationUnlocked(entry Entry) error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := entry.Validate(); err != nil {
		return err
	}
	if _, err := m.profileByName(entry.Name); err == nil {
		return fmt.Errorf("tunnel profile name %q already exists", entry.Name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	machine, err := m.findOrCreateMachineUnlocked(entry.Server, entry.User, entry.KeyFile)
	if err != nil {
		return err
	}
	return m.addTunnelProfileUnlocked(TunnelProfile{ID: entry.ID, Name: entry.Name, Description: entry.Description, MachineID: machine.ID, RemoteHost: entry.RemoteHost, RemotePort: entry.RemotePort, LocalPort: entry.LocalPort})
}

func (m *manager) GetConfigurations() ([]Entry, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	profiles, err := m.GetTunnelProfiles()
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(profiles))
	for _, profile := range profiles {
		entry, err := m.ResolveTunnelProfile(profile.ID)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func (m *manager) GetConfiguration(entryName string) (Entry, error) {
	if err := m.ready(); err != nil {
		return Entry{}, err
	}
	if err := validateConfigurationName(entryName); err != nil {
		return Entry{}, err
	}
	profile, err := m.profileByName(entryName)
	if err != nil {
		return Entry{}, err
	}
	return m.ResolveTunnelProfile(profile.ID)
}

func (m *manager) UpdateConfiguration(entry Entry) error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.updateConfigurationUnlocked(entry)
}

func (m *manager) updateConfigurationUnlocked(entry Entry) error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := entry.Validate(); err != nil {
		return err
	}
	var profile TunnelProfile
	var err error
	if entry.ID != "" {
		profile, err = m.GetTunnelProfile(entry.ID)
	} else {
		profile, err = m.profileByName(entry.Name)
	}
	if err != nil {
		return err
	}
	if existing, findErr := m.profileByName(entry.Name); findErr == nil && existing.ID != profile.ID {
		return fmt.Errorf("tunnel profile name %q already exists", entry.Name)
	} else if findErr != nil && !errors.Is(findErr, os.ErrNotExist) {
		return findErr
	}
	machine, err := m.findOrCreateMachineUnlocked(entry.Server, entry.User, entry.KeyFile)
	if err != nil {
		return err
	}
	profile.Name, profile.Description, profile.MachineID = entry.Name, entry.Description, machine.ID
	profile.RemoteHost, profile.RemotePort, profile.LocalPort = entry.RemoteHost, entry.RemotePort, entry.LocalPort
	return m.updateTunnelProfileUnlocked(profile)
}

func (m *manager) RemoveConfiguration(entryName string) error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.removeConfigurationUnlocked(entryName)
}

func (m *manager) removeConfigurationUnlocked(entryName string) error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := validateConfigurationName(entryName); err != nil {
		return err
	}
	profile, err := m.profileByName(entryName)
	if err != nil {
		return err
	}
	return m.removeTunnelProfileUnlocked(profile.ID)
}

func (m *manager) GetMachine(id string) (Machine, error) {
	if err := m.ready(); err != nil {
		return Machine{}, err
	}
	if err := validateID(id); err != nil {
		return Machine{}, err
	}
	var machine Machine
	if err := m.readJSON(m.machineFile(id), &machine); err != nil {
		return Machine{}, fmt.Errorf("read machine %q: %w", id, err)
	}
	return machine, nil
}

func (m *manager) GetMachines() ([]Machine, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	machines := make([]Machine, 0)
	if err := m.readEntities(m.machinesPath(), func(data []byte) error {
		var machine Machine
		if err := json.Unmarshal(data, &machine); err != nil {
			return err
		}
		machines = append(machines, machine)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("read machines: %w", err)
	}
	sort.Slice(machines, func(i, j int) bool { return machines[i].Name < machines[j].Name })
	return machines, nil
}

func (m *manager) AddMachine(machine Machine) error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.addMachineUnlocked(machine)
}

func (m *manager) addMachineUnlocked(machine Machine) error {
	if err := m.ready(); err != nil {
		return err
	}
	if machine.ID == "" {
		var err error
		machine.ID, err = newID()
		if err != nil {
			return err
		}
	}
	if err := validateID(machine.ID); err != nil {
		return err
	}
	if err := machine.Validate(); err != nil {
		return err
	}
	if err := m.ensureMachineNameAvailable(machine.Name, machine.ID); err != nil {
		return err
	}
	if _, err := os.Stat(m.machineFile(machine.ID)); err == nil {
		return fmt.Errorf("machine %q already exists", machine.ID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return m.writeJSON(m.machineFile(machine.ID), machine)
}

func (m *manager) UpdateMachine(machine Machine) error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.updateMachineUnlocked(machine)
}

func (m *manager) updateMachineUnlocked(machine Machine) error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := validateID(machine.ID); err != nil {
		return err
	}
	if err := machine.Validate(); err != nil {
		return err
	}
	if err := m.ensureMachineNameAvailable(machine.Name, machine.ID); err != nil {
		return err
	}
	if _, err := os.Stat(m.machineFile(machine.ID)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("machine %q: %w", machine.ID, err)
		}
		return err
	}
	return m.writeJSON(m.machineFile(machine.ID), machine)
}

func (m *manager) RemoveMachine(id string) error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.removeMachineUnlocked(id)
}

func (m *manager) removeMachineUnlocked(id string) error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	profiles, err := m.GetTunnelProfiles()
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		if profile.MachineID == id {
			return fmt.Errorf("machine %q is referenced by tunnel profile %q", id, profile.Name)
		}
	}
	if err := os.Remove(m.machineFile(id)); err != nil {
		return fmt.Errorf("remove machine %q: %w", id, err)
	}
	return nil
}

func (m *manager) GetTunnelProfile(id string) (TunnelProfile, error) {
	if err := m.ready(); err != nil {
		return TunnelProfile{}, err
	}
	if err := validateID(id); err != nil {
		return TunnelProfile{}, err
	}
	var profile TunnelProfile
	if err := m.readJSON(m.profileFile(id), &profile); err != nil {
		return TunnelProfile{}, fmt.Errorf("read tunnel profile %q: %w", id, err)
	}
	return profile, nil
}

func (m *manager) GetTunnelProfiles() ([]TunnelProfile, error) {
	if err := m.ready(); err != nil {
		return nil, err
	}
	profiles := make([]TunnelProfile, 0)
	if err := m.readEntities(m.profilesPath(), func(data []byte) error {
		var profile TunnelProfile
		if err := json.Unmarshal(data, &profile); err != nil {
			return err
		}
		profiles = append(profiles, profile)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("read tunnel profiles: %w", err)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}

func (m *manager) AddTunnelProfile(profile TunnelProfile) error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.addTunnelProfileUnlocked(profile)
}

func (m *manager) addTunnelProfileUnlocked(profile TunnelProfile) error {
	if err := m.ready(); err != nil {
		return err
	}
	if profile.ID == "" {
		var err error
		profile.ID, err = newID()
		if err != nil {
			return err
		}
	}
	if err := validateID(profile.ID); err != nil {
		return err
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	if _, err := m.GetMachine(profile.MachineID); err != nil {
		return fmt.Errorf("profile machine: %w", err)
	}
	if err := m.ensureProfileNameAvailable(profile.Name, profile.ID); err != nil {
		return err
	}
	if _, err := os.Stat(m.profileFile(profile.ID)); err == nil {
		return fmt.Errorf("tunnel profile %q already exists", profile.ID)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return m.writeJSON(m.profileFile(profile.ID), profile)
}

func (m *manager) UpdateTunnelProfile(profile TunnelProfile) error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.updateTunnelProfileUnlocked(profile)
}

func (m *manager) updateTunnelProfileUnlocked(profile TunnelProfile) error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := validateID(profile.ID); err != nil {
		return err
	}
	if err := profile.Validate(); err != nil {
		return err
	}
	if _, err := m.GetMachine(profile.MachineID); err != nil {
		return fmt.Errorf("profile machine: %w", err)
	}
	if err := m.ensureProfileNameAvailable(profile.Name, profile.ID); err != nil {
		return err
	}
	if _, err := os.Stat(m.profileFile(profile.ID)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("tunnel profile %q: %w", profile.ID, err)
		}
		return err
	}
	return m.writeJSON(m.profileFile(profile.ID), profile)
}

func (m *manager) RemoveTunnelProfile(id string) error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.removeTunnelProfileUnlocked(id)
}

func (m *manager) removeTunnelProfileUnlocked(id string) error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	if err := os.Remove(m.profileFile(id)); err != nil {
		return fmt.Errorf("remove tunnel profile %q: %w", id, err)
	}
	return nil
}

func (m *manager) ResolveTunnelProfile(id string) (Entry, error) {
	if err := m.ready(); err != nil {
		return Entry{}, err
	}
	profile, err := m.GetTunnelProfile(id)
	if err != nil {
		return Entry{}, err
	}
	machine, err := m.GetMachine(profile.MachineID)
	if err != nil {
		return Entry{}, fmt.Errorf("resolve machine for tunnel profile %q: %w", id, err)
	}
	return Entry{ID: profile.ID, MachineID: machine.ID, Name: profile.Name, Description: profile.Description, Server: machine.Server, User: machine.User, KeyFile: machine.KeyFile, RemoteHost: profile.RemoteHost, RemotePort: profile.RemotePort, LocalPort: profile.LocalPort}, nil
}

func (m *manager) findOrCreateMachineUnlocked(server, user, keyFile string) (Machine, error) {
	server, user, keyFile = strings.TrimSpace(server), strings.TrimSpace(user), strings.TrimSpace(keyFile)
	machines, err := m.GetMachines()
	if err != nil {
		return Machine{}, err
	}
	for _, machine := range machines {
		if strings.TrimSpace(machine.Server) == server && strings.TrimSpace(machine.User) == user && strings.TrimSpace(machine.KeyFile) == keyFile {
			return machine, nil
		}
	}
	machine := Machine{Server: server, User: user, KeyFile: keyFile, Name: m.uniqueMachineName(user, server, machines)}
	if err := m.addMachineUnlocked(machine); err != nil {
		return Machine{}, err
	}
	machines, err = m.GetMachines()
	if err != nil {
		return Machine{}, err
	}
	for _, candidate := range machines {
		if candidate.Server == server && candidate.User == user && candidate.KeyFile == keyFile {
			return candidate, nil
		}
	}
	return Machine{}, errors.New("created machine could not be found")
}

func (m *manager) uniqueMachineName(user, server string, machines []Machine) string {
	base := user + "@" + server
	if len(base) > 128 {
		base = base[:128]
	}
	name := base
	for n := 2; ; n++ {
		used := false
		for _, machine := range machines {
			if machine.Name == name {
				used = true
				break
			}
		}
		if !used {
			return name
		}
		suffix := fmt.Sprintf(" (%d)", n)
		prefix := base
		if len(prefix)+len(suffix) > 128 {
			prefix = prefix[:128-len(suffix)]
		}
		name = prefix + suffix
	}
}

func (m *manager) profileByName(name string) (TunnelProfile, error) {
	profiles, err := m.GetTunnelProfiles()
	if err != nil {
		return TunnelProfile{}, err
	}
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, nil
		}
	}
	return TunnelProfile{}, fmt.Errorf("tunnel profile %q: %w", name, os.ErrNotExist)
}

func (m *manager) ensureMachineNameAvailable(name, id string) error {
	machines, err := m.GetMachines()
	if err != nil {
		return err
	}
	for _, machine := range machines {
		if machine.Name == name && machine.ID != id {
			return fmt.Errorf("machine name %q already exists", name)
		}
	}
	return nil
}
func (m *manager) ensureProfileNameAvailable(name, id string) error {
	profiles, err := m.GetTunnelProfiles()
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		if profile.Name == name && profile.ID != id {
			return fmt.Errorf("tunnel profile name %q already exists", name)
		}
	}
	return nil
}

func (m *manager) migrateLegacy() error {
	configMu.Lock()
	defer configMu.Unlock()
	return m.migrateLegacyUnlocked()
}

func (m *manager) migrateLegacyUnlocked() error {
	marker := filepath.Join(m.dir, schemaMarkerFile)
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat schema marker: %w", err)
	}
	files, err := os.ReadDir(m.dir)
	if err != nil {
		return fmt.Errorf("read legacy configuration directory: %w", err)
	}
	machines := make(map[string]Machine)
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") || file.Name() == config.ActiveTunnelsFile || file.Name() == schemaMarkerFile {
			continue
		}
		data, err := os.ReadFile(filepath.Join(m.dir, file.Name()))
		if err != nil {
			return fmt.Errorf("read legacy configuration file %q: %w", file.Name(), err)
		}
		var document any
		if err := json.Unmarshal(data, &document); err != nil {
			return fmt.Errorf("parse legacy configuration file %q: %w", file.Name(), err)
		}
		object, ok := document.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := object["Name"]; !ok {
			if _, ok := object["name"]; !ok {
				continue
			}
		}
		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			return fmt.Errorf("parse legacy configuration file %q: %w", file.Name(), err)
		}
		server, user, keyFile := strings.TrimSpace(entry.Server), strings.TrimSpace(entry.User), strings.TrimSpace(entry.KeyFile)
		key := normalizedMachineTuple(server, user, keyFile)
		machine, ok := machines[key]
		if !ok {
			machine = Machine{
				ID:      deterministicLegacyID("legacy-machine:", key),
				Name:    m.uniqueMachineName(user, server, mapMachines(machines)),
				Server:  server,
				User:    user,
				KeyFile: keyFile,
			}
			machines[key] = machine
			if err := m.writeJSON(m.machineFile(machine.ID), machine); err != nil {
				return fmt.Errorf("write migrated machine: %w", err)
			}
		}
		profile := TunnelProfile{
			ID:          deterministicLegacyID("legacy-profile:", file.Name()),
			Name:        entry.Name,
			Description: entry.Description,
			MachineID:   machine.ID,
			RemoteHost:  entry.RemoteHost,
			RemotePort:  entry.RemotePort,
			LocalPort:   entry.LocalPort,
		}
		if err := m.writeJSON(m.profileFile(profile.ID), profile); err != nil {
			return fmt.Errorf("write migrated tunnel profile: %w", err)
		}
	}
	return m.writeJSON(marker, struct {
		Version int `json:"version"`
	}{Version: schemaVersion})
}

func normalizedMachineTuple(server, user, keyFile string) string {
	return strings.TrimSpace(server) + "\x00" + strings.TrimSpace(user) + "\x00" + strings.TrimSpace(keyFile)
}

func deterministicLegacyID(prefix, source string) string {
	sum := sha256.Sum256([]byte(prefix + source))
	return hex.EncodeToString(sum[:])
}

func mapMachines(byKey map[string]Machine) []Machine {
	machines := make([]Machine, 0, len(byKey))
	for _, machine := range byKey {
		machines = append(machines, machine)
	}
	return machines
}

func (m *manager) ready() error                 { return m.initErr }
func (m *manager) machineFile(id string) string { return filepath.Join(m.machinesPath(), id+".json") }
func (m *manager) profileFile(id string) string { return filepath.Join(m.profilesPath(), id+".json") }

func (m *manager) readEntities(dir string, consume func([]byte) error) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return err
		}
		if err := consume(data); err != nil {
			return fmt.Errorf("parse %q: %w", file.Name(), err)
		}
	}
	return nil
}
func (m *manager) readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
func (m *manager) writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", " ")
	if err != nil {
		return fmt.Errorf("marshal %q: %w", path, err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".write-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return fmt.Errorf("set permissions on temporary file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace %q: %w", path, err)
	}
	return nil
}

func newID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
func validateID(id string) error {
	if id == "" || filepath.Base(id) != id || strings.ContainsAny(id, "\\/\x00") {
		return fmt.Errorf("invalid ID %q", id)
	}
	return nil
}
