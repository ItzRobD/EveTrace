//go:build !tray

package main

import "context"

// runTray is a no-op when built without -tags tray.
// The server still starts and can be accessed at http://localhost:<addr>.
func runTray(_ string, _ context.CancelFunc) {}
