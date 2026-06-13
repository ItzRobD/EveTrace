package appconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config holds all user-persisted settings.
type Config struct {
	LogDir string `json:"logDir"`
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
