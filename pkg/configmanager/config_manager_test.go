package configmanager

import (
	"errors"
	"os"
	"path/filepath"
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

	filename := filepath.Join(dir, "production.json")
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
	matches, err := filepath.Glob(filepath.Join(dir, ".production-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remain after write: %v", matches)
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
