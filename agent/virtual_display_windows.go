//go:build windows

package main

// Virtual Display Driver integration.
//
// Oblireach bundles the VirtualDrivers/Virtual-Display-Driver (MttVDD) so
// that sessions without a renderable real display (headless VMs, console
// session on Hyper-V basic video, RDP disconnected, pre-login Winlogon)
// still have a DXGI-capable output for capture.
//
// At service start we:
//   1. Drop bundled driver files into ProgramData\ObliReachAgent\vdd\
//   2. pnputil /add-driver MttVDD.inf /install  (SYSTEM-context, idempotent)
//   3. Copy vdd_settings.xml to ProgramData\VirtualDisplayDriver\
// The driver then registers an always-on UMDF device providing one virtual
// monitor; DXGI enumerates it alongside the real adapters.
//
// This file runs only on Windows (build tag) and uses no CGo — pnputil and
// file copies are enough for Phase 1. Runtime enable/disable will come in
// Phase 2 when we wire SetupDi for on-demand monitor lifecycle.

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	vddHardwareID    = "Root\\MttVDD"
	vddFriendlyName  = "Virtual Display Driver"
	vddSettingsDir   = `C:\ProgramData\VirtualDisplayDriver`
	vddSettingsFile  = "vdd_settings.xml"
	vddInstallMarker = "vdd-installed.marker"
)

// vddEnsureInstalled guarantees the Virtual Display Driver is present in the
// driver store and that its settings XML is in place. Idempotent — safe to
// call at every service start. Returns nil on success or a benign skip.
//
// Requires SYSTEM context (agent service). On failure returns the error but
// the agent keeps running — capture still works for sessions that have a
// real active display; only the headless/login-screen cases degrade.
func vddEnsureInstalled(configDir string) error {
	vddDir := filepath.Join(configDir, "vdd")
	if err := os.MkdirAll(vddDir, 0755); err != nil {
		return fmt.Errorf("vdd: mkdir %s: %w", vddDir, err)
	}

	exeDir, err := vddExeDir()
	if err != nil {
		return fmt.Errorf("vdd: locate exe dir: %w", err)
	}
	// Where the MSI places the bundled files (next to the agent exe).
	bundleDir := filepath.Join(exeDir, "vdd")
	if _, err := os.Stat(bundleDir); err != nil {
		// Fallback for dev builds where the assets live in the source tree.
		bundleDir = filepath.Join(exeDir, "driver-assets", "vdd")
	}
	for _, name := range []string{"MttVDD.inf", "MttVDD.dll", "mttvdd.cat", "signpath-publisher.cer"} {
		src := filepath.Join(bundleDir, name)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("vdd: bundled file missing: %s (ship it next to the agent exe)", src)
		}
	}

	// Windows requires the driver's signing publisher to be in the machine's
	// Trusted Publishers store before pnputil will silently install, even
	// when the Authenticode chain itself is valid. pnputil otherwise bails
	// with 0xE0000242 = SPAPI_E_AUTHENTICODE_PUBLISHER_NOT_TRUSTED.
	// The VDD is signed by SignPath Foundation; we pre-trust its public
	// certificate so the install is fully unattended.
	certPath := filepath.Join(bundleDir, "signpath-publisher.cer")
	if err := vddTrustPublisher(certPath); err != nil {
		log.Printf("vdd: trust publisher: %v (continuing — install may still fail)", err)
	}

	// Write the settings XML to the path the driver reads at startup.
	if err := os.MkdirAll(vddSettingsDir, 0755); err != nil {
		return fmt.Errorf("vdd: mkdir settings dir: %w", err)
	}
	settingsSrc := filepath.Join(bundleDir, vddSettingsFile)
	settingsDst := filepath.Join(vddSettingsDir, vddSettingsFile)
	if err := copyFileIfDifferent(settingsSrc, settingsDst); err != nil {
		log.Printf("vdd: copy settings.xml: %v (continuing)", err)
	}

	// Fast-path: if the driver is already known AND the device node exists,
	// skip the install dance. We verify the device actually exists (not just
	// the driver store entry) because earlier builds could end up with a
	// package installed but no device instantiated.
	marker := filepath.Join(vddDir, vddInstallMarker)
	if _, err := os.Stat(marker); err == nil {
		if vddDevicePresent() {
			log.Printf("vdd: already installed (marker + device present)")
			return nil
		}
		log.Printf("vdd: marker present but device missing — re-running install")
		_ = os.Remove(marker)
	}

	infPath := filepath.Join(bundleDir, "MttVDD.inf")
	log.Printf("vdd: adding driver package to driver store from %s", infPath)
	cmd := exec.Command("pnputil.exe", "/add-driver", infPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	log.Printf("vdd: pnputil output:\n%s", strings.TrimSpace(string(out)))
	if err != nil {
		// 259 = ERROR_NO_MORE_ITEMS (package identical to one already in store)
		// 3010 = ERROR_SUCCESS_REBOOT_REQUIRED — both benign.
		if ee, ok := err.(*exec.ExitError); ok {
			code := ee.ExitCode()
			if code != 259 && code != 3010 {
				return fmt.Errorf("vdd: pnputil failed: %w", err)
			}
			log.Printf("vdd: pnputil exit=%d (treating as success)", code)
		} else {
			return fmt.Errorf("vdd: pnputil failed: %w", err)
		}
	}

	// pnputil /add-driver only places the package in the driver store.
	// For Root-enumerated software devices (Root\MttVDD), no PnP event
	// ever fires to create an actual device instance — we must register
	// one ourselves via SetupAPI, then point it at the INF.
	if err := vddEnsureDevicePresent(infPath); err != nil {
		return fmt.Errorf("vdd: device instantiation: %w", err)
	}
	_ = os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)), 0644)
	log.Printf("vdd: driver installed and device instantiated successfully")
	return nil
}

// vddExeDir returns the directory containing the running exe, resolving
// symlinks and junctions the same way the MSI does.
func vddExeDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// vddTrustPublisher adds cert (a .cer file) to the LocalMachine Trusted
// Publishers store. Required for unattended driver installation — without
// this pnputil refuses with SPAPI_E_AUTHENTICODE_PUBLISHER_NOT_TRUSTED
// (0xE0000242) even though the signature chain is cryptographically valid.
// Idempotent: certutil returns success if the cert is already in the store.
func vddTrustPublisher(cerPath string) error {
	cmd := exec.Command("certutil.exe", "-addstore", "-f", "TrustedPublisher", cerPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		return fmt.Errorf("certutil: %w (output: %s)", err, trimmed)
	}
	log.Printf("vdd: added publisher to TrustedPublisher store (certutil: %s)", trimmed)
	return nil
}

func copyFileIfDifferent(src, dst string) error {
	sIn, err := os.Stat(src)
	if err != nil {
		return err
	}
	if sOut, err := os.Stat(dst); err == nil {
		if sOut.Size() == sIn.Size() && sOut.ModTime().Equal(sIn.ModTime()) {
			return nil
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Chtimes(dst, sIn.ModTime(), sIn.ModTime())
}
