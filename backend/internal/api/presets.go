package api

import (
	"EveTrace/internal/appconfig"
)

type LogDirPreset struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

func logDirPresets() []LogDirPreset {
	var presets []LogDirPreset
	for _, p := range appconfig.GetLogDirPresets() {
		presets = append(presets, LogDirPreset{
			Label: p.Label,
			Path:  p.Path,
		})
	}
	return presets
}
