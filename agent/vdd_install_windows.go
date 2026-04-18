//go:build windows

package main

/*
#cgo LDFLAGS: -lsetupapi -lnewdev -ladvapi32

#include <windows.h>
#include <setupapi.h>
#include <newdev.h>
#include <string.h>

// VDD display class GUID: {4D36E968-E325-11CE-BFC1-08002BE10318}
static const GUID GUID_DEVCLASS_DISPLAY = {
    0x4D36E968, 0xE325, 0x11CE, { 0xBF, 0xC1, 0x08, 0x00, 0x2B, 0xE1, 0x03, 0x18 }
};

// vddDeviceExists returns 1 if any device with hardware id "Root\MttVDD"
// is already enumerated in the system, 0 otherwise. Used to skip the
// instantiation step on subsequent service starts.
static int vddDeviceExists(void) {
    HDEVINFO devInfo = SetupDiGetClassDevsW(&GUID_DEVCLASS_DISPLAY, NULL, NULL, DIGCF_PRESENT);
    if (devInfo == INVALID_HANDLE_VALUE) return 0;
    SP_DEVINFO_DATA devData;
    devData.cbSize = sizeof(devData);
    int found = 0;
    for (DWORD i = 0; SetupDiEnumDeviceInfo(devInfo, i, &devData); i++) {
        wchar_t hwid[512] = {0};
        DWORD hwType = 0;
        if (SetupDiGetDeviceRegistryPropertyW(devInfo, &devData, SPDRP_HARDWAREID,
                &hwType, (PBYTE)hwid, sizeof(hwid), NULL)) {
            // Multi-string (REG_MULTI_SZ): iterate until empty string.
            wchar_t *p = hwid;
            while (*p) {
                if (_wcsicmp(p, L"Root\\MttVDD") == 0 || _wcsicmp(p, L"MttVDD") == 0) {
                    found = 1;
                    break;
                }
                p += wcslen(p) + 1;
            }
            if (found) break;
        }
    }
    SetupDiDestroyDeviceInfoList(devInfo);
    return found;
}

// vddCreateAndInstall instantiates a new Root\MttVDD device and binds the
// driver package pointed to by infPath to it. This is the equivalent of
// `devcon install MttVDD.inf Root\MttVDD`, using the documented SetupAPI
// sequence that all IDD sample installers follow.
//
// Returns 0 on success, negative Windows error code on failure.
// outReboot receives 1 if the install needs a reboot, else 0.
static int vddCreateAndInstall(const wchar_t *infPath, int *outReboot) {
    *outReboot = 0;

    HDEVINFO devInfo = SetupDiCreateDeviceInfoList(&GUID_DEVCLASS_DISPLAY, NULL);
    if (devInfo == INVALID_HANDLE_VALUE) return -(int)GetLastError();

    SP_DEVINFO_DATA devData;
    devData.cbSize = sizeof(devData);
    if (!SetupDiCreateDeviceInfoW(devInfo, L"Display", &GUID_DEVCLASS_DISPLAY,
            NULL, NULL, DICD_GENERATE_ID, &devData)) {
        DWORD err = GetLastError();
        SetupDiDestroyDeviceInfoList(devInfo);
        return -(int)err;
    }

    // Hardware ID must be a double-null-terminated REG_MULTI_SZ.
    static const wchar_t hwid[] = L"Root\\MttVDD\0";
    if (!SetupDiSetDeviceRegistryPropertyW(devInfo, &devData, SPDRP_HARDWAREID,
            (const BYTE *)hwid, sizeof(hwid))) {
        DWORD err = GetLastError();
        SetupDiDestroyDeviceInfoList(devInfo);
        return -(int)err;
    }

    // Register the device node with PnP so Windows knows about it.
    if (!SetupDiCallClassInstaller(DIF_REGISTERDEVICE, devInfo, &devData)) {
        DWORD err = GetLastError();
        SetupDiDestroyDeviceInfoList(devInfo);
        return -(int)err;
    }

    SetupDiDestroyDeviceInfoList(devInfo);

    // Now that the node exists, point it at the driver package. The INF
    // is already in the driver store (via pnputil /add-driver), so this
    // just matches the hardware id and installs.
    BOOL reboot = FALSE;
    if (!UpdateDriverForPlugAndPlayDevicesW(NULL, L"Root\\MttVDD",
            infPath, INSTALLFLAG_FORCE, &reboot)) {
        return -(int)GetLastError();
    }
    *outReboot = reboot ? 1 : 0;
    return 0;
}
*/
import "C"

import (
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

// vddDevicePresent reports whether a Root\MttVDD device already exists
// in the system. Cheap check (enumerates Display class PnP devices).
func vddDevicePresent() bool {
	return C.vddDeviceExists() != 0
}

// vddEnsureDevicePresent instantiates a Root\MttVDD device node (if none
// exists) and binds the MttVDD driver to it. Without this step, pnputil
// /add-driver only puts the package in the driver store; no monitor
// actually appears because Root\ devices are software-only and need
// explicit registration.
func vddEnsureDevicePresent(infPath string) error {
	if vddDevicePresent() {
		log.Printf("vdd: device Root\\MttVDD already present — skipping instantiation")
		return nil
	}
	infW, err := syscall.UTF16PtrFromString(infPath)
	if err != nil {
		return fmt.Errorf("vdd: utf16 conv: %w", err)
	}
	var reboot C.int
	rc := int(C.vddCreateAndInstall((*C.wchar_t)(unsafe.Pointer(infW)), &reboot))
	if rc < 0 {
		return fmt.Errorf("vdd: device create+install failed (err=%d / 0x%x)", -rc, uint32(-rc))
	}
	if reboot != 0 {
		log.Printf("vdd: device created, reboot requested by driver (not fatal)")
	} else {
		log.Printf("vdd: device Root\\MttVDD instantiated successfully")
	}
	return nil
}
