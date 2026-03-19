//go:build windows

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// performUpdate downloads the new Oblireach MSI and runs msiexec /i in a
// detached process. msiexec will stop the running service, replace the binary,
// then restart it — so this function does not wait for completion.
func performUpdate(cfg *Config, cmd *command, relativeURL, version string) {
	log.Printf("Command %s: self-update to v%s", cmd.ID, version)

	// ── 1. Build absolute download URL ────────────────────────────────────
	base := strings.TrimRight(cfg.ServerURL, "/")
	rel := strings.TrimLeft(relativeURL, "/")
	fullURL := base + "/" + rel

	// ── 2. Download MSI ───────────────────────────────────────────────────
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		log.Printf("Command %s: update: build request: %v", cmd.ID, err)
		return
	}
	req.Header.Set("X-Api-Key", cfg.APIKey)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Command %s: update: download failed: %v", cmd.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Command %s: update: server returned %d", cmd.ID, resp.StatusCode)
		return
	}

	tmpPath := filepath.Join(os.TempDir(), "oblireach-update.msi")
	f, err := os.Create(tmpPath)
	if err != nil {
		log.Printf("Command %s: update: create temp file: %v", cmd.ID, err)
		return
	}
	_, copyErr := io.Copy(f, resp.Body)
	f.Close()
	if copyErr != nil {
		log.Printf("Command %s: update: write MSI: %v", cmd.ID, copyErr)
		return
	}

	log.Printf("Command %s: update: MSI ready at %s — launching msiexec", cmd.ID, tmpPath)

	// ── 3. Launch msiexec as detached process ─────────────────────────────
	// msiexec will: stop ObliReachAgent service → replace binary → start service.
	// We launch detached so the service process can be cleanly stopped by SCM.
	logPath := filepath.Join(os.TempDir(), "oblireach-update.log")
	msiexec := exec.Command("msiexec.exe",
		"/i", tmpPath,
		fmt.Sprintf("SERVERURL=%s", cfg.ServerURL),
		fmt.Sprintf("APIKEY=%s", cfg.APIKey),
		"/quiet", "/norestart",
		"/l*v", logPath,
	)
	msiexec.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008, // DETACHED_PROCESS — survive service stop
		HideWindow:    true,
	}
	if err := msiexec.Start(); err != nil {
		log.Printf("Command %s: update: launch msiexec: %v", cmd.ID, err)
		return
	}

	log.Printf("Command %s: update: msiexec running (PID %d) — service will restart with v%s",
		cmd.ID, msiexec.Process.Pid, version)
}
