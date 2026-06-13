//go:build tray && linux

package main

import "context"

// runTray is a no-op on Linux — systray requires libayatana-appindicator3
// which is not a dependency we want to force on users.
// The server is still accessible at http://localhost:<addr>.
func runTray(_ string, _ context.CancelFunc) {}
