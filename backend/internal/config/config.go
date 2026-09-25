// Package config is the environment contract shared by the two commands.
//
// It exists so the seeder and the server cannot disagree about where the CSVs are
// or how to reach the database, and so a wrong value is reported at startup, by
// name, instead of surfacing later as something that looks like a bug elsewhere.
package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// environment is the source of values, so the tests can supply one without touching
// the process's own.
type environment map[string]string

// Config is everything the commands need to know.
type Config struct {
	DatabaseURL string
	// Host is the interface the API listens on. Empty listens on all interfaces,
	// which is needed inside Docker; Compose controls the host-side exposure.
	Host string
	Port int
	// DataDir holds readings.csv and events.csv.
	DataDir string
	// StaticDir holds the built dashboard. When it exists it is served from the same
	// origin as the API, so there is one origin in production and no CORS anywhere.
	StaticDir string
	// APIKey enables the narrator. Empty is the supported case, not a misconfiguration:
	// the deterministic explanations are the product's floor.
	APIKey string
	Model  string
	// BaseURL points the narrator at a different endpoint, so a reviewer without an
	// OpenAI key can still exercise the model path against a local stub.
	BaseURL     string
	LogLevel    string
	ShowVersion bool
}

// Defaults for running directly on the host. Compose supplies its own database URL.
const (
	defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/energy?sslmode=disable"
	defaultPort        = 8080
	defaultModel       = "gpt-4o-mini"
	defaultLogLevel    = "info"
)

// Load reads the configuration, refusing anything unusable rather than substituting
// a default for it.
func Load(env environment) (Config, error) {
	cfg := Config{
		DatabaseURL: value(env, "DATABASE_URL", defaultDatabaseURL),
		Host:        strings.TrimSpace(env["HOST"]),
		DataDir:     value(env, "DATA_DIR", defaultDataDir()),
		StaticDir:   value(env, "STATIC_DIR", defaultStaticDir()),
		APIKey:      strings.TrimSpace(env["OPENAI_API_KEY"]),
		Model:       value(env, "OPENAI_MODEL", defaultModel),
		BaseURL:     strings.TrimSpace(env["OPENAI_BASE_URL"]),
		LogLevel:    strings.ToLower(value(env, "LOG_LEVEL", defaultLogLevel)),
		ShowVersion: env["SHOW_VERSION"] == "1",
	}

	port, err := port(env["PORT"])
	if err != nil {
		return Config{}, err
	}
	cfg.Port = port

	if _, err := parseLevel(cfg.LogLevel); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// FromEnviron reads the process's own environment. The commands use it, and the
// tests use Load, so the tested path and the running path differ only in where the
// values come from.
func FromEnviron() (Config, error) {
	env := environment{}
	for _, entry := range os.Environ() {
		if name, val, found := strings.Cut(entry, "="); found {
			env[name] = val
		}
	}
	return Load(env)
}

// port reads PORT, treating a blank or absent value as the default and anything
// unusable as an error naming the value.
func port(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultPort, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 1 || parsed > 65535 {
		return 0, fmt.Errorf("PORT=%q is not a port number between 1 and 65535", raw)
	}
	return parsed, nil
}

// value reads one variable, treating a blank as absent so an exported-but-empty
// secret cannot blank a setting.
func value(env environment, name, fallback string) string {
	if trimmed := strings.TrimSpace(env[name]); trimmed != "" {
		return trimmed
	}
	return fallback
}

// defaultDataDir is the repository's data directory, so the commands work from any
// working directory.
func defaultDataDir() string { return repoPath("data") }

// defaultStaticDir is the built dashboard, which is what the API serves in
// production.
func defaultStaticDir() string { return repoPath("frontend", "dist") }

// repoPath joins the parts to the repository root, so the commands work from
// backend/, from the repository root, and from the container's working directory
// alike.
func repoPath(parts ...string) string {
	root, err := repositoryRoot()
	if err != nil {
		// Nothing recognisable above here, which means the binary was built and
		// shipped rather than run from a checkout: DATA_DIR and STATIC_DIR must be set.
		return filepath.Join(parts...)
	}
	return filepath.Join(append([]string{root}, parts...)...)
}

// repositoryRoot is the nearest ancestor holding the delivered data directory.
//
// The data directory is the marker rather than go.mod, because go.mod sits in
// backend/ while the data sits beside it: anchoring on go.mod would make the default
// data path backend/data, which does not exist. Falling back to the parent of a
// go.mod keeps the guess reasonable when the data directory is missing entirely, as
// it is in a test fixture.
func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// The first pass takes the nearest ancestor that holds a data directory.
	for candidate := dir; ; candidate = filepath.Dir(candidate) {
		if _, err := dataDirAbove(candidate); err == nil {
			return candidate, nil
		}
		if filepath.Dir(candidate) == candidate {
			break
		}
	}
	// The second pass settles for a go.mod, whose directory is the backend.
	for candidate := dir; ; candidate = filepath.Dir(candidate) {
		if _, err := os.Stat(filepath.Join(candidate, "go.mod")); err == nil {
			return candidate, nil
		}
		if filepath.Dir(candidate) == candidate {
			return "", fmt.Errorf("neither a data directory nor a go.mod was found above %s", dir)
		}
	}
}

// dataDirAbove returns dir/data when it is a directory.
func dataDirAbove(dir string) (string, error) {
	candidate := filepath.Join(dir, "data")
	info, err := os.Stat(candidate)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", candidate)
	}
	return candidate, nil
}

// NarrationEnabled reports whether a narrator is configured. It is false with no
// key, which is a supported way to run the product rather than a misconfiguration:
// the deterministic explanations are the floor the narrator improves on.
func (c Config) NarrationEnabled() bool { return c.APIKey != "" }

// LogLevelValue is the level named by the configuration.
func (c Config) LogLevelValue() slog.Level {
	level, err := parseLevel(c.LogLevel)
	if err != nil {
		return slog.LevelInfo
	}
	return level
}

// parseLevel keeps the accepted names to the four a person would actually type,
// rather than whatever slog.Level happens to accept.
func parseLevel(name string) (slog.Level, error) {
	switch name {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL=%q is not one of debug, info, warn or error", name)
	}
}

// Address is where the server listens.
func (c Config) Address() string { return net.JoinHostPort(c.Host, strconv.Itoa(c.Port)) }

// ReadingsFile and EventsFile are the two delivered CSVs.
func (c Config) ReadingsFile() string { return filepath.Join(c.DataDir, "readings.csv") }
func (c Config) EventsFile() string   { return filepath.Join(c.DataDir, "events.csv") }
