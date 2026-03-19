//go:build !windows

package main

import (
	"log"
	"runtime"
)

// performUpdate is not yet implemented on non-Windows platforms.
// Linux/macOS agents can be updated by replacing the binary and
// restarting the service via the system service manager.
func performUpdate(cfg *Config, cmd *command, relativeURL, version string) {
	log.Printf("Command %s: auto-update not implemented on %s (v%s available)",
		cmd.ID, runtime.GOOS, version)
}
