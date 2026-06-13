package core

import "time"

// Log level constants — match slog's text representations so the drain
// goroutine can switch on them without importing log/slog in pkg/core.
const (
	LevelDebug = "DEBUG"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

// Event codes emitted by backend components.
const (
	CodeCollision       = "collision"         // two characters share one log file
	CodeHeaderParseFail = "header_parse_fail" // could not read session header after retries
	CodeNoListener      = "no_listener"       // file has no character attached (header-only)
	CodeNoLogDir        = "no_log_dir"        // log directory not configured or invalid
)

// LogEvent is a structured diagnostic event produced by backend components.
// It is both written to the application log file (via the logger package) and
// forwarded to connected frontend clients over WebSocket for toast display.
type LogEvent struct {
	Level   string    `json:"level"`   // Level* constant
	Code    string    `json:"code"`    // Code* constant
	File    string    `json:"file"`    // source log file that triggered the event
	Message string    `json:"message"` // human-readable description
	At      time.Time `json:"at"`
}
