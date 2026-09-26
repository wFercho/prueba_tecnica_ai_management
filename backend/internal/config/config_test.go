package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The commands read the environment, so what matters is that a missing variable has
// a working default and a wrong one is refused at startup rather than halfway
// through a run.

func TestDefaultsMakeTheStackRunWithNoEnvironmentAtAll(t *testing.T) {
	cfg, err := Load(environment{})
	if err != nil {
		t.Fatalf("Load with an empty environment: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("port = %d, want 8080", cfg.Port)
	}
	if cfg.Address() != ":8080" {
		t.Errorf("address = %q, want :8080", cfg.Address())
	}
	if cfg.DatabaseURL == "" {
		t.Error("no default database URL, so `go run ./cmd/server` cannot connect to anything")
	}
	if cfg.DataDir == "" {
		t.Error("no default data directory, so the seeder needs a flag to find the CSVs")
	}
	if cfg.StaticDir == "" {
		t.Error("no default static directory, so the built dashboard is not served")
	}
	// No key is the normal case for a reviewer, and it must fall back to the
	// deterministic narrator rather than refusing to start.
	if cfg.NarrationEnabled() {
		t.Error("narration is enabled without an API key")
	}
	if cfg.DemoPassword != "admin" {
		t.Errorf("demo password = %q, want the documented default", cfg.DemoPassword)
	}
}

func TestTheEnvironmentOverridesTheDefaults(t *testing.T) {
	cfg, err := Load(environment{
		"DATABASE_URL":   "postgres://someone@elsewhere:5433/other",
		"PORT":           "9090",
		"HOST":           "127.0.0.1",
		"DATA_DIR":       "/tmp/csv",
		"STATIC_DIR":     "/srv/dashboard",
		"OPENAI_API_KEY": "sk-test",
		"OPENAI_MODEL":   "gpt-4o-mini",
		"DEMO_PASSWORD":  "cambiada",
		"LOG_LEVEL":      "debug",
	})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.DemoPassword != "cambiada" {
		t.Errorf("demo password = %q, want the configured value", cfg.DemoPassword)
	}

	if cfg.DatabaseURL != "postgres://someone@elsewhere:5433/other" {
		t.Errorf("database = %q", cfg.DatabaseURL)
	}
	if cfg.Port != 9090 {
		t.Errorf("port = %d, want 9090", cfg.Port)
	}
	if cfg.Address() != "127.0.0.1:9090" {
		t.Errorf("address = %q, want 127.0.0.1:9090", cfg.Address())
	}
	if cfg.DataDir != "/tmp/csv" {
		t.Errorf("data dir = %q", cfg.DataDir)
	}
	if cfg.StaticDir != "/srv/dashboard" {
		t.Errorf("static dir = %q", cfg.StaticDir)
	}
	if !cfg.NarrationEnabled() {
		t.Error("narration is disabled with an API key set")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("log level = %q, want debug", cfg.LogLevel)
	}
}

func TestAnUnusablePortIsRefusedWithTheValueThatBrokeIt(t *testing.T) {
	// A typo in PORT should say so at startup. Falling back to 8080 would start a
	// server on a port nobody asked for, and the error would surface much later as
	// something else entirely.
	for _, raw := range []string{"not-a-number", "0", "-1", "70000"} {
		_, err := Load(environment{"PORT": raw})
		if err == nil {
			t.Errorf("PORT=%q was accepted", raw)
			continue
		}
		if !strings.Contains(err.Error(), raw) {
			t.Errorf("the error for PORT=%q does not quote the value: %v", raw, err)
		}
	}
}

func TestHostSupportsIPv6AndBlankUsesTheDefault(t *testing.T) {
	for _, tt := range []struct {
		name    string
		host    string
		address string
	}{
		{name: "ipv6 address", host: "::1", address: "[::1]:8080"},
		{name: "blank host", host: "  ", address: ":8080"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(environment{"HOST": tt.host})
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if cfg.Address() != tt.address {
				t.Errorf("address = %q, want %q", cfg.Address(), tt.address)
			}
		})
	}
}

func TestAnUnknownLogLevelIsRefusedRatherThanSilentlyDowngraded(t *testing.T) {
	if _, err := Load(environment{"LOG_LEVEL": "chatty"}); err == nil {
		t.Error("LOG_LEVEL=chatty was accepted")
	}
}

func TestAnEmptyValueMeansTheDefaultRatherThanEmpty(t *testing.T) {
	// A variable exported but blank is a common accident — a leftover in a .env, a
	// CI secret that resolved to nothing — and it must not blank the setting.
	cfg, err := Load(environment{"PORT": "", "DATA_DIR": ""})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("port = %d, want the default with a blank PORT", cfg.Port)
	}
	if cfg.DataDir == "" {
		t.Error("a blank DATA_DIR blanked the default")
	}
}

func TestTheRepositoryRootIsFoundFromAnywhereInsideIt(t *testing.T) {
	// The data directory is beside backend/, not inside it, so anchoring on go.mod
	// would produce a path that does not exist. This is the case that broke the
	// seeder when it was run from backend/.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatalf("create data: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "backend", "cmd", "server"), 0o755); err != nil {
		t.Fatalf("create backend: %v", err)
	}
	writeFile(t, filepath.Join(root, "backend", "go.mod"), "module example.com/backend\n")

	for _, from := range []string{
		filepath.Join(root, "backend", "cmd", "server"),
		filepath.Join(root, "backend"),
		root,
	} {
		t.Chdir(from)
		got, err := repositoryRoot()
		if err != nil {
			t.Fatalf("from %s: %v", from, err)
		}
		if got != root {
			t.Errorf("from %s the root is %s, want %s", from, got, root)
		}
	}
}

func TestTheDefaultsPointAtTheDeliveredData(t *testing.T) {
	cfg, err := Load(environment{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// The point of the lookup above: the seeder must find the CSVs with no flags.
	if _, err := os.Stat(cfg.ReadingsFile()); err != nil {
		t.Errorf("the default readings file is not there: %v", err)
	}
	if _, err := os.Stat(cfg.EventsFile()); err != nil {
		t.Errorf("the default events file is not there: %v", err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
