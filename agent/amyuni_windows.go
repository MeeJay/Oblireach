//go:build windows

package main

// Amyuni USB Mobile Monitor Virtual Display (usbmmidd) integration.
//
// Amyuni's IDD is WHQL-signed by Microsoft Hardware Compatibility Publisher
// (ships with the zlib-style license that allows redistribution) and is
// what Apollo / Sunshine / RustDesk use for the pre-login / headless /
// RDP-disconnected scenarios where the OS refuses to render to a
// non-plugged display. Unlike MttVDD which installs a permanent
// always-on monitor, Amyuni supports plug/unplug on demand via:
//
//   deviceinstaller64.exe install usbmmidd.inf usbmmidd   (once, at boot)
//   deviceinstaller64.exe enableidd 1                     (plug a monitor)
//   deviceinstaller64.exe enableidd 0                     (unplug)
//
// The plug/unplug semantics matter: Windows treats a freshly-plugged
// monitor as "hot" and composites to it even on a no-user session, so
// Winlogon / login prompt ends up rendered on the virtual display and
// our capture picks it up.

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	amyuniBundleSubdir = "amyuni"
	amyuniInstalled    = "amyuni-installed.marker"
)

func amyuniAssetsDir(configDir string) string {
	return filepath.Join(configDir, amyuniBundleSubdir)
}

func amyuniBundleDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exeDir := filepath.Dir(exe)
	// MSI layout: exe/amyuni/
	if _, err := os.Stat(filepath.Join(exeDir, "amyuni", "usbmmIdd.inf")); err == nil {
		return filepath.Join(exeDir, "amyuni")
	}
	// Source tree layout for dev builds.
	if _, err := os.Stat(filepath.Join(exeDir, "driver-assets", "amyuni", "usbmmIdd.inf")); err == nil {
		return filepath.Join(exeDir, "driver-assets", "amyuni")
	}
	return ""
}

// amyuniEnsureInstalled copies Amyuni publisher cert to TrustedPublisher
// (for unattended install on machines that haven't trusted Microsoft's
// hardware compat publisher — normally always trusted but belt-and-braces),
// then runs `deviceinstaller64 install usbmmidd.inf usbmmidd`. Idempotent.
func amyuniEnsureInstalled(configDir string) error {
	bundle := amyuniBundleDir()
	if bundle == "" {
		return fmt.Errorf("amyuni: driver bundle missing from exe dir")
	}
	stateDir := amyuniAssetsDir(configDir)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("amyuni: mkdir %s: %w", stateDir, err)
	}
	marker := filepath.Join(stateDir, amyuniInstalled)
	if _, err := os.Stat(marker); err == nil {
		if amyuniDevicePresent() {
			return nil
		}
		log.Printf("amyuni: marker present but device missing — reinstalling")
		_ = os.Remove(marker)
	}

	installer := filepath.Join(bundle, "deviceinstaller64.exe")
	if _, err := os.Stat(installer); err != nil {
		return fmt.Errorf("amyuni: deviceinstaller64.exe not in bundle")
	}
	log.Printf("amyuni: running %s install usbmmidd.inf usbmmidd", installer)
	cmd := exec.Command(installer, "install", "usbmmIdd.inf", "usbmmidd")
	cmd.Dir = bundle
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	log.Printf("amyuni: installer output:\n%s", strings.TrimSpace(string(out)))
	if err != nil {
		return fmt.Errorf("amyuni: install failed: %w", err)
	}
	_ = os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)), 0644)
	return nil
}

// amyuniEnableMonitor plugs one virtual monitor into the system. Equivalent
// to `deviceinstaller64 enableidd 1`. Called at the start of a capture
// helper on no-user / disconnected sessions where the OS wouldn't otherwise
// have an active display to render to.
func amyuniEnableMonitor() error {
	return amyuniRunEnableIdd("1")
}

// amyuniDisableMonitor unplugs one virtual monitor. `deviceinstaller64
// enableidd 0`. Called at helper shutdown so the machine doesn't accumulate
// phantom monitors between sessions.
func amyuniDisableMonitor() error {
	return amyuniRunEnableIdd("0")
}

func amyuniRunEnableIdd(arg string) error {
	bundle := amyuniBundleDir()
	if bundle == "" {
		return fmt.Errorf("amyuni: bundle dir missing")
	}
	installer := filepath.Join(bundle, "deviceinstaller64.exe")
	cmd := exec.Command(installer, "enableidd", arg)
	cmd.Dir = bundle
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	log.Printf("amyuni: enableidd %s → %s (err=%v)", arg, strings.TrimSpace(string(out)), err)
	return err
}

// amyuniDevicePresent returns true if any USB Mobile Monitor Virtual
// Display device is currently enumerated. Used to decide whether to skip
// the one-time driver install.
func amyuniDevicePresent() bool {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command",
		`(Get-PnpDevice | Where-Object { $_.FriendlyName -like '*USB Mobile Monitor*' -or $_.FriendlyName -like '*usbmmidd*' } | Measure-Object).Count`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != "0"
}
