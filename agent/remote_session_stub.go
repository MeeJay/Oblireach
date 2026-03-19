//go:build !windows

package main

import (
	"fmt"
	"log"
	"runtime"
)

func runHelperMode(addr string) {
	log.Fatalf("capture helper mode not supported on %s", runtime.GOOS)
}

func startCrossSessionStream(cfg *Config, token string, sessionID int) error {
	return fmt.Errorf("cross-session streaming not supported on %s", runtime.GOOS)
}
