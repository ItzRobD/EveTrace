package appconfig

import (
	"os"
	"path/filepath"
	"runtime"
)

// LogDirPreset represents a suggested log directory path.
type LogDirPreset struct {
	Label string
	Path  string
}

// GetLogDirPresets returns a list of potential EVE Online log directories with labels.
func GetLogDirPresets() []LogDirPreset {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	if runtime.GOOS == "windows" {
		return []LogDirPreset{
			{
				Label: "Windows (EVE Online)",
				Path:  filepath.Join(home, "Documents", "EVE", "logs", "Gamelogs"),
			},
			{
				Label: "Windows OneDrive (EVE Online)",
				Path:  filepath.Join(home, "OneDrive", "Documents", "EVE", "logs", "Gamelogs"),
			},
		}
	}

	username := filepath.Base(home)
	return []LogDirPreset{
		{
			Label: "Linux Steam (Proton)",
			Path:  filepath.Join(home, ".local", "share", "Steam", "steamapps", "compatdata", "8500", "pfx", "drive_c", "users", "steamuser", "My Documents", "EVE", "logs", "Gamelogs"),
		},
		{
			Label: "Linux Games (EVE Online)",
			Path:  filepath.Join(home, "Games", "eve-online", "drive_c", "users", username, "My Documents", "EVE", "logs", "Gamelogs"),
		},
	}
}

// GetDefaultLogDirs returns a list of potential EVE Online log directories
// in order of preference.
func GetDefaultLogDirs() []string {
	presets := GetLogDirPresets()
	dirs := make([]string, len(presets))
	for i, p := range presets {
		dirs[i] = p.Path
	}
	return dirs
}

// DetectLogDir scans the default locations and returns the first one that
// contains EVE log files. Returns (path, true) if found, ("", false) otherwise.
func DetectLogDir() (string, bool) {
	dirs := GetDefaultLogDirs()
	for _, dir := range dirs {
		if IsLogDirValid(dir) {
			return dir, true
		}
	}
	return "", false
}

// IsLogDirValid checks if the directory exists and contains at least one .txt file.
func IsLogDirValid(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return false
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".txt" {
			return true
		}
	}
	return false
}
