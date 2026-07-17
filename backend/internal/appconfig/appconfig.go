package appconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultIdleTimeoutSeconds is used when the config has no saved value.
const DefaultIdleTimeoutSeconds = 30

// Config holds all user-persisted settings.
type Config struct {
	LogDir  string `json:"logDir"`
	MinDate string `json:"minDate"` // ISO8601 or similar
	// IdleTimeoutSeconds is how long the backend keeps running after the last
	// dashboard window closes (0 = keep running). A nil pointer means the field
	// was never set, so DefaultIdleTimeoutSeconds applies — distinct from an
	// explicit 0.
	IdleTimeoutSeconds *int `json:"idleTimeoutSeconds,omitempty"`
}

// IdleTimeoutSecs returns the effective idle-shutdown timeout in seconds
// (0 = keep running in the background). An unset value uses the default.
func (c Config) IdleTimeoutSecs() int {
	if c.IdleTimeoutSeconds == nil {
		return DefaultIdleTimeoutSeconds
	}
	if *c.IdleTimeoutSeconds < 0 {
		return 0
	}
	return *c.IdleTimeoutSeconds
}

// IdleTimeout returns IdleTimeoutSecs as a duration (0 = keep running).
func (c Config) IdleTimeout() time.Duration {
	return time.Duration(c.IdleTimeoutSecs()) * time.Second
}

var (
	mu       sync.RWMutex
	filePath string
	current  Config
)

// Init loads the config from disk (or initialises defaults if absent).
// Must be called once at startup before any other function.
func Init(path string) error {
	if path == "" {
		path = defaultPath()
	}
	filePath = path

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &current)
}

// Get returns a copy of the current config.
func Get() Config {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// SetLogDir updates the log directory and persists to disk.
func SetLogDir(dir string) error {
	mu.Lock()
	current.LogDir = dir
	cfg := current
	mu.Unlock()
	return save(cfg)
}

// SetMinDate updates the minimum log date and persists to disk.
func SetMinDate(date string) error {
	mu.Lock()
	current.MinDate = date
	cfg := current
	mu.Unlock()
	return save(cfg)
}

// SetIdleTimeoutSeconds updates the idle-shutdown timeout (0 = keep running)
// and persists to disk.
func SetIdleTimeoutSeconds(seconds int) error {
	if seconds < 0 {
		seconds = 0
	}
	mu.Lock()
	current.IdleTimeoutSeconds = &seconds
	cfg := current
	mu.Unlock()
	return save(cfg)
}

func save(cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0o644)
}

// defaultPath returns config.json in the same directory as the running binary.
func defaultPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}
