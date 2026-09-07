package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigurationDirPrecedence(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(t.TempDir(), "xdg")

	for _, test := range []struct {
		name       string
		sshtmDir   string
		legacyDir  string
		expected   string
		overridden bool
	}{
		{
			name:     "XDG default",
			expected: filepath.Join(xdg, DefaultConfigDirName),
		},
		{
			name:       "legacy override",
			legacyDir:  "/legacy/config",
			expected:   "/legacy/config",
			overridden: true,
		},
		{
			name:       "SSHTM override takes precedence",
			sshtmDir:   "/preferred/config",
			legacyDir:  "/legacy/config",
			expected:   "/preferred/config",
			overridden: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			t.Setenv(XDGConfigHomeEnvironment, xdg)
			t.Setenv(ConfigDirEnvironment, test.sshtmDir)
			t.Setenv(ConfigDirFlagName, test.legacyDir)

			if dir := ConfigurationDir(); dir != test.expected {
				t.Fatalf("ConfigurationDir() = %q, want %q", dir, test.expected)
			}
			if overridden := ConfigurationDirOverridden(); overridden != test.overridden {
				t.Fatalf("ConfigurationDirOverridden() = %t, want %t", overridden, test.overridden)
			}
		})
	}

	t.Run("HOME fallback", func(t *testing.T) {
		t.Setenv("HOME", home)
		t.Setenv(XDGConfigHomeEnvironment, "")
		t.Setenv(ConfigDirEnvironment, "")
		t.Setenv(ConfigDirFlagName, "")

		expected := filepath.Join(home, ".config", DefaultConfigDirName)
		if dir := ConfigurationDir(); dir != expected {
			t.Fatalf("ConfigurationDir() = %q, want %q", dir, expected)
		}
	})
}

func TestMigrateLegacyConfigDirCopiesContentsAndIsIdempotent(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("HOME", home)
	t.Setenv(XDGConfigHomeEnvironment, xdg)
	t.Setenv(ConfigDirEnvironment, "")
	t.Setenv(ConfigDirFlagName, "")

	legacyDir := filepath.Join(home, legacyConfigDirName)
	if err := os.MkdirAll(filepath.Join(legacyDir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	activeTunnels := []byte(`[{"name":"production","pid":42}]`)
	if err := os.WriteFile(filepath.Join(legacyDir, ActiveTunnelsFile), activeTunnels, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "nested", "configuration.json"), []byte(`{"Name":"production"}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := MigrateLegacyConfigDir(); err != nil {
		t.Fatalf("MigrateLegacyConfigDir() returned an error: %v", err)
	}
	targetDir := filepath.Join(xdg, DefaultConfigDirName)
	copiedActiveTunnels, err := os.ReadFile(filepath.Join(targetDir, ActiveTunnelsFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(copiedActiveTunnels) != string(activeTunnels) {
		t.Fatalf("active tunnels were not preserved: got %q, want %q", copiedActiveTunnels, activeTunnels)
	}
	if _, err := os.Stat(filepath.Join(targetDir, "nested", "configuration.json")); err != nil {
		t.Fatalf("nested legacy content was not copied: %v", err)
	}
	info, err := os.Stat(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0700 {
		t.Fatalf("migrated directory mode = %o, want 0700", info.Mode().Perm())
	}
	info, err = os.Stat(filepath.Join(targetDir, ActiveTunnelsFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("migrated file mode = %o, want 0600", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(legacyDir, ActiveTunnelsFile)); err != nil {
		t.Fatalf("migration removed legacy data: %v", err)
	}

	if err := MigrateLegacyConfigDir(); err != nil {
		t.Fatalf("second MigrateLegacyConfigDir() returned an error: %v", err)
	}
	copiedActiveTunnels, err = os.ReadFile(filepath.Join(targetDir, ActiveTunnelsFile))
	if err != nil || string(copiedActiveTunnels) != string(activeTunnels) {
		t.Fatalf("idempotent migration changed active tunnels: data=%q err=%v", copiedActiveTunnels, err)
	}
}

func TestMigrateLegacyConfigDirSkipsOverridesAndPopulatedTarget(t *testing.T) {
	for _, test := range []struct {
		name      string
		sshtmDir  string
		legacyDir string
	}{
		{name: "SSHTM_CONFIG_DIR", sshtmDir: filepath.Join(t.TempDir(), "override")},
		{name: "legacy config-dir", legacyDir: filepath.Join(t.TempDir(), "override")},
	} {
		t.Run("override/"+test.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv(XDGConfigHomeEnvironment, filepath.Join(t.TempDir(), "xdg"))
			t.Setenv(ConfigDirEnvironment, test.sshtmDir)
			t.Setenv(ConfigDirFlagName, test.legacyDir)

			legacyDir := filepath.Join(home, legacyConfigDirName)
			if err := os.MkdirAll(legacyDir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(legacyDir, ActiveTunnelsFile), []byte("legacy"), 0600); err != nil {
				t.Fatal(err)
			}

			if err := MigrateLegacyConfigDir(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(ConfigurationDir()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("override path was migrated: %v", err)
			}
		})
	}

	t.Run("populated target", func(t *testing.T) {
		home := t.TempDir()
		xdg := filepath.Join(t.TempDir(), "xdg")
		t.Setenv("HOME", home)
		t.Setenv(XDGConfigHomeEnvironment, xdg)
		t.Setenv(ConfigDirEnvironment, "")
		t.Setenv(ConfigDirFlagName, "")

		legacyDir := filepath.Join(home, legacyConfigDirName)
		if err := os.MkdirAll(legacyDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(legacyDir, ActiveTunnelsFile), []byte("legacy"), 0600); err != nil {
			t.Fatal(err)
		}
		targetDir := ConfigurationDir()
		if err := os.MkdirAll(targetDir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, "existing"), []byte("new"), 0600); err != nil {
			t.Fatal(err)
		}

		if err := MigrateLegacyConfigDir(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(targetDir, ActiveTunnelsFile)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("legacy content was merged into populated target: %v", err)
		}
		if _, err := os.Stat(filepath.Join(legacyDir, ActiveTunnelsFile)); err != nil {
			t.Fatalf("legacy content was removed after conflict: %v", err)
		}
	})
}
