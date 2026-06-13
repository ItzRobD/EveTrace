package appconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetLogDirPresets(t *testing.T) {
	presets := GetLogDirPresets()
	if presets == nil {
		t.Fatal("presets should not be nil")
	}

	home, _ := os.UserHomeDir()
	username := filepath.Base(home)

	if runtime.GOOS == "linux" {
		foundCustom := false
		for _, p := range presets {
			if p.Label == "Linux Games (EVE Online)" {
				foundCustom = true
				expected := filepath.Join(home, "Games", "eve-online", "drive_c", "users", username, "My Documents", "EVE", "logs", "Gamelogs")
				if p.Path != expected {
					t.Errorf("expected path %s, got %s", expected, p.Path)
				}
			}
		}
		// This will fail before my changes
		if !foundCustom {
			t.Error("Linux Games (EVE Online) preset not found")
		}
	}
}
