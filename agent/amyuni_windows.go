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

// amyuniEnsureInstalled runs `deviceinstaller64 install usbmmidd.inf usbmmidd`
// once. Idempotent — the installer returns non-zero exit codes when the
// driver is already present (exit 1 "already installed", exit 2 after an
// interrupted re-install), so we treat "install succeeded" as "device is
// actually enumerable in the system" via amyuniDevicePresent().
// Also cleans up any phantom monitors left from previous crashed helpers.
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
	markerPresent := false
	if _, err := os.Stat(marker); err == nil {
		markerPresent = true
	}
	deviceAlready := amyuniDevicePresent()

	if !markerPresent || !deviceAlready {
		installer := filepath.Join(bundle, "deviceinstaller64.exe")
		if _, err := os.Stat(installer); err != nil {
			return fmt.Errorf("amyuni: deviceinstaller64.exe not in bundle")
		}
		log.Printf("amyuni: running deviceinstaller64 install usbmmIdd.inf usbmmidd")
		cmd := exec.Command(installer, "install", "usbmmIdd.inf", "usbmmidd")
		cmd.Dir = bundle
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		out, _ := cmd.CombinedOutput()
		log.Printf("amyuni: installer output:\n%s", strings.TrimSpace(string(out)))
	}

	// Verify device is in the system — deviceinstaller64 exit code is
	// unreliable, check PnP state directly.
	if !amyuniDevicePresent() {
		return fmt.Errorf("amyuni: install ran but no usbmmidd device present")
	}

	// Cleanup: remove every phantom monitor from previous helpers that
	// crashed before their deferred unplug. enableidd 0 removes one at a
	// time; loop until no more exist, cap to avoid infinite loop if
	// something goes wrong.
	amyuniUnplugAll()

	_ = os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)), 0644)
	log.Printf("amyuni: driver ready, phantom monitors cleared")
	return nil
}

// amyuniUnplugAll calls enableidd 0 until the next call reports there are
// no more monitors to remove (or a safety cap is reached).
func amyuniUnplugAll() {
	for i := 0; i < 8; i++ {
		if amyuniActiveMonitorCount() == 0 {
			return
		}
		if err := amyuniRunEnableIdd("0"); err != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// amyuniActiveMonitorCount returns how many USB Mobile Monitor Virtual
// Display devices are currently enumerated AND OK. PnP "OK" state
// correlates with "a monitor is actively plugged from the driver".
func amyuniActiveMonitorCount() int {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command",
		`(Get-PnpDevice | Where-Object { ($_.FriendlyName -like '*USB Mobile Monitor*' -or $_.InstanceId -like '*usbmmidd*') -and $_.Status -eq 'OK' } | Measure-Object).Count`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	n := 0
	_, _ = fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

// amyuniEnableMonitor ensures exactly ONE Amyuni monitor is plugged, no
// more no less. Clears any phantoms first (from previous crashed helpers)
// then plugs one fresh monitor. Keeping count==1 is critical because the
// capture path captures the whole virtual screen, which grows with each
// active monitor (800+1920+... = several-K-wide rect) and crashes both the
// Magnification API and the H.264 encoder at extreme widths.
func amyuniEnableMonitor() error {
	amyuniUnplugAll()
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
