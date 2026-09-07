package configmanager

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func validEntry(name string) Entry {
	return Entry{
		Name:       name,
		Server:     "bastion.example.com",
		User:       "alice",
		KeyFile:    "/home/alice/.ssh/id_ed25519",
		RemoteHost: "database.internal",
		RemotePort: 5432,
		LocalPort:  0,
	}
}

func TestNewManagerResolvesAndCreatesHomeRelativeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configManager, err := NewManagerWithError("~/.ssh-tunnel-manager")
	if err != nil {
		t.Fatalf("NewManagerWithError returned an error: %v", err)
	}
	internalManager := configManager.(*manager)
	expected := filepath.Join(home, ".ssh-tunnel-manager")

	if internalManager.dir != expected {
		t.Fatalf("expected manager directory %q, got %q", expected, internalManager.dir)
	}
	info, err := os.Stat(expected)
	if err != nil {
		t.Fatalf("expected manager directory to exist: %v", err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("expected directory mode 0700, got %o", info.Mode().Perm())
	}
}

func TestNewManagerWithErrorDoesNotPanicForInvalidDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}

	manager, err := NewManagerWithError(path)
	if err == nil {
		t.Fatal("expected an error for a file used as a configuration directory")
	}
	if manager != nil {
		t.Fatal("expected no manager on initialization failure")
	}
	compatibilityManager := NewManager(path)
	if _, err := compatibilityManager.GetConfigurations(); err == nil {
		t.Fatal("compatibility constructor should preserve the initialization error")
	}
}

func TestEntryValidate(t *testing.T) {
	for _, test := range []struct {
		name  string
		entry Entry
		valid bool
	}{
		{"valid", validEntry("production"), true},
		{"empty name", validEntry(""), false},
		{"traversal name", validEntry("../outside"), false},
		{"backslash name", validEntry(`..\outside`), false},
		{"remote port too low", func() Entry { e := validEntry("test"); e.RemotePort = 0; return e }(), false},
		{"remote port too high", func() Entry { e := validEntry("test"); e.RemotePort = 65536; return e }(), false},
		{"local port negative", func() Entry { e := validEntry("test"); e.LocalPort = -1; return e }(), false},
		{"local port too high", func() Entry { e := validEntry("test"); e.LocalPort = 65536; return e }(), false},
		{"missing server", func() Entry { e := validEntry("test"); e.Server = ""; return e }(), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.entry.Validate()
			if (err == nil) != test.valid {
				t.Fatalf("Validate() error = %v, valid = %v", err, test.valid)
			}
		})
	}
}

func TestManagerRejectsTraversalNames(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(dir), "outside.json")

	for _, name := range []string{"../outside", "nested/name", `nested\name`, ".", "..", ""} {
		t.Run(name, func(t *testing.T) {
			if err := manager.AddConfiguration(validEntry(name)); err == nil {
				t.Fatal("AddConfiguration accepted an unsafe name")
			}
			if err := manager.UpdateConfiguration(validEntry(name)); err == nil {
				t.Fatal("UpdateConfiguration accepted an unsafe name")
			}
			if _, err := manager.GetConfiguration(name); err == nil {
				t.Fatal("GetConfiguration accepted an unsafe name")
			}
			if err := manager.RemoveConfiguration(name); err == nil {
				t.Fatal("RemoveConfiguration accepted an unsafe name")
			}
		})
	}
	if _, err := os.Stat(outside); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsafe name wrote outside config directory: %v", err)
	}
}

func TestConfigurationWritesArePrivateAndAtomic(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	entry := validEntry("production")
	if err := manager.AddConfiguration(entry); err != nil {
		t.Fatal(err)
	}

	resolved, err := manager.GetConfiguration(entry.Name)
	if err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(dir, profilesDirectory, resolved.ID+".json")
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("expected file mode 0600, got %o", info.Mode().Perm())
	}
	entry.Description = "updated"
	if err := manager.UpdateConfiguration(entry); err != nil {
		t.Fatal(err)
	}
	loaded, err := manager.GetConfiguration(entry.Name)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Description != "updated" {
		t.Fatalf("expected atomically replaced configuration, got %#v", loaded)
	}
	matches, err := filepath.Glob(filepath.Join(dir, profilesDirectory, ".write-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after write: %v", matches)
	}
}

func TestSharedMachineIsReusedByLegacyEntries(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	first := validEntry("production")
	second := validEntry("staging")
	second.RemoteHost = "staging-database.internal"
	if err := manager.AddConfiguration(first); err != nil {
		t.Fatal(err)
	}
	if err := manager.AddConfiguration(second); err != nil {
		t.Fatal(err)
	}
	machines, err := manager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := manager.GetTunnelProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(machines) != 1 || len(profiles) != 2 {
		t.Fatalf("expected one shared machine and two profiles, got %d machines and %d profiles", len(machines), len(profiles))
	}
	if profiles[0].MachineID != profiles[1].MachineID {
		t.Fatalf("profiles did not share a machine: %#v", profiles)
	}
}

func TestLegacyMigrationIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	first := validEntry("production")
	second := validEntry("staging")
	second.RemoteHost = "staging-database.internal"
	for _, entry := range []Entry{first, second} {
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := manager.GetTunnelProfiles()
	if err != nil {
		t.Fatal(err)
	}
	machines, err := manager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || len(machines) != 1 {
		t.Fatalf("unexpected migrated entities: %d profiles, %d machines", len(profiles), len(machines))
	}
	profileIDs := []string{profiles[0].ID, profiles[1].ID}
	machineID := machines[0].ID

	manager, err = NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err = manager.GetTunnelProfiles()
	if err != nil {
		t.Fatal(err)
	}
	machines, err = manager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 || len(machines) != 1 || profiles[0].ID != profileIDs[0] || profiles[1].ID != profileIDs[1] || machines[0].ID != machineID {
		t.Fatalf("migration was not idempotent: profiles=%#v machines=%#v", profiles, machines)
	}
	if _, err := os.Stat(filepath.Join(dir, "production.json")); err != nil {
		t.Fatalf("migration deleted legacy configuration: %v", err)
	}
}

func TestLegacyMigrationRecoversFromPartialRun(t *testing.T) {
	dir := t.TempDir()
	first := validEntry("production")
	second := validEntry("staging")
	second.RemoteHost = "staging-database.internal"
	for _, entry := range []Entry{first, second} {
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	internalManager := &manager{dir: dir}
	if err := os.MkdirAll(internalManager.machinesPath(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(internalManager.profilesPath(), 0700); err != nil {
		t.Fatal(err)
	}
	machineID := deterministicLegacyID("legacy-machine:", normalizedMachineTuple(first.Server, first.User, first.KeyFile))
	machine := Machine{ID: machineID, Name: "alice@bastion.example.com", Server: first.Server, User: first.User, KeyFile: first.KeyFile}
	if err := internalManager.writeJSON(internalManager.machineFile(machine.ID), machine); err != nil {
		t.Fatal(err)
	}
	profile := TunnelProfile{ID: deterministicLegacyID("legacy-profile:", "production.json"), Name: first.Name, Description: first.Description, MachineID: machine.ID, RemoteHost: first.RemoteHost, RemotePort: first.RemotePort, LocalPort: first.LocalPort}
	if err := internalManager.writeJSON(internalManager.profileFile(profile.ID), profile); err != nil {
		t.Fatal(err)
	}

	configManager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	machines, err := configManager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := configManager.GetTunnelProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(machines) != 1 || len(profiles) != 2 {
		t.Fatalf("expected recovery to produce one machine and two profiles, got %d machines and %d profiles", len(machines), len(profiles))
	}
	if machines[0].ID != machineID {
		t.Fatalf("expected deterministic machine ID %q, got %q", machineID, machines[0].ID)
	}
	for _, filename := range []string{"production.json", "staging.json"} {
		id := deterministicLegacyID("legacy-profile:", filename)
		if _, err := configManager.GetTunnelProfile(id); err != nil {
			t.Fatalf("expected deterministic migrated profile %q: %v", id, err)
		}
	}
}

func TestLegacyMigrationSkipsUnrelatedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "unrelated.json"), []byte(`{"version":1}`), 0600); err != nil {
		t.Fatal(err)
	}

	configManager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	machines, err := configManager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := configManager.GetTunnelProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(machines) != 0 || len(profiles) != 0 {
		t.Fatalf("unrelated JSON was migrated: machines=%#v profiles=%#v", machines, profiles)
	}
}

func TestLegacyMigrationRejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "malformed.json"), []byte(`{"Name":`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManagerWithError(dir); err == nil {
		t.Fatal("expected malformed JSON to prevent migration")
	}
}

func TestConcurrentManagerConstructionMigratesOnce(t *testing.T) {
	dir := t.TempDir()
	for _, entry := range []Entry{validEntry("production"), validEntry("staging")} {
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}

	const managerCount = 10
	start := make(chan struct{})
	errs := make(chan error, managerCount)
	var wg sync.WaitGroup
	for range managerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := NewManagerWithError(dir)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("NewManagerWithError returned an error: %v", err)
		}
	}

	configManager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	machines, err := configManager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := configManager.GetTunnelProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(machines) != 1 || len(profiles) != 2 {
		t.Fatalf("expected one machine and two profiles after concurrent migration, got %d machines and %d profiles", len(machines), len(profiles))
	}
}

func TestRemoveReferencedMachineFails(t *testing.T) {
	dir := t.TempDir()
	manager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	entry := validEntry("production")
	if err := manager.AddConfiguration(entry); err != nil {
		t.Fatal(err)
	}
	resolved, err := manager.GetConfiguration(entry.Name)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.RemoveMachine(resolved.MachineID); err == nil {
		t.Fatal("expected removal of referenced machine to fail")
	}
}

func TestConcurrentAddTunnelProfileRejectsDuplicateNames(t *testing.T) {
	dir := t.TempDir()
	firstManager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	secondManager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	machine := Machine{Name: "bastion", Server: "bastion.example.com", User: "alice", KeyFile: "/home/alice/.ssh/id_ed25519"}
	if err := firstManager.AddMachine(machine); err != nil {
		t.Fatal(err)
	}
	machines, err := firstManager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	machineID := machines[0].ID

	const attempts = 20
	start := make(chan struct{})
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		configManager := firstManager
		if i%2 == 1 {
			configManager = secondManager
		}
		wg.Add(1)
		go func(configManager ConfigManager) {
			defer wg.Done()
			<-start
			errs <- configManager.AddTunnelProfile(TunnelProfile{Name: "production", MachineID: machineID, RemoteHost: "database.internal", RemotePort: 5432, LocalPort: 0})
		}(configManager)
	}
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful profile add, got %d", successes)
	}
	profiles, err := firstManager.GetTunnelProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || profiles[0].Name != "production" {
		t.Fatalf("expected one production profile, got %#v", profiles)
	}
}

func TestConcurrentAddTunnelProfileAndRemoveMachinePreservesReferences(t *testing.T) {
	dir := t.TempDir()
	firstManager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	secondManager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	machine := Machine{Name: "bastion", Server: "bastion.example.com", User: "alice", KeyFile: "/home/alice/.ssh/id_ed25519"}
	if err := firstManager.AddMachine(machine); err != nil {
		t.Fatal(err)
	}
	machines, err := firstManager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	machineID := machines[0].ID

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		errs <- firstManager.AddTunnelProfile(TunnelProfile{Name: "production", MachineID: machineID, RemoteHost: "database.internal", RemotePort: 5432, LocalPort: 0})
	}()
	go func() {
		defer wg.Done()
		<-start
		errs <- secondManager.RemoveMachine(machineID)
	}()
	close(start)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected one successful operation, got %d", successes)
	}
	profiles, err := firstManager.GetTunnelProfiles()
	if err != nil {
		t.Fatal(err)
	}
	machines, err = firstManager.GetMachines()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) == 0 {
		if len(machines) != 0 {
			t.Fatalf("expected removed machine when no profile was added, got %#v", machines)
		}
		return
	}
	if len(profiles) != 1 || len(machines) != 1 || profiles[0].MachineID != machines[0].ID {
		t.Fatalf("found dangling profile or unexpected entities: machines=%#v profiles=%#v", machines, profiles)
	}
}

func TestExistingConfigurationsRemainReadable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "legacy.json"), []byte(`{"Name":"legacy"}`), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManagerWithError(dir)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := manager.GetConfiguration("legacy")
	if err != nil {
		t.Fatalf("expected legacy configuration to remain readable: %v", err)
	}
	if entry.Name != "legacy" {
		t.Fatalf("unexpected legacy entry: %#v", entry)
	}
	entries, err := manager.GetConfigurations()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected legacy configurations to remain listable, entries=%#v err=%v", entries, err)
	}
}
