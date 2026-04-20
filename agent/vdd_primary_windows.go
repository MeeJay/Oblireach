//go:build windows

package main

// vddMakePrimary — on a no-user session the MttVDD virtual monitor is
// present but secondary, so Windows renders Winlogon / sign-in UI on the
// physical console adapter (Hyper-V Video) which our helper can't reach
// via DXGI nor see as part of the virtual-screen rectangle used by the
// Magnification API. Promoting the VDD to primary makes the compositor
// render everything there, and our capture path picks it up.

/*
#cgo LDFLAGS: -luser32 -ladvapi32

#include <windows.h>
#include <stdio.h>
#include <wchar.h>

// findVddDeviceName enumerates display devices and returns the \\.\DISPLAYn
// name of a monitor whose driver is the MttVDD virtual display. Writes the
// name into out (UTF-16, null-terminated). Returns 1 on success, 0 if not
// found.
static int findVddDeviceName(wchar_t *out, int outChars) {
    DISPLAY_DEVICEW dev;
    ZeroMemory(&dev, sizeof(dev));
    dev.cb = sizeof(dev);
    for (DWORD i = 0; EnumDisplayDevicesW(NULL, i, &dev, 0); i++) {
        // DeviceString is the driver name (e.g. "Virtual Display Driver",
        // "Microsoft Hyper-V Video", "Microsoft Remote Display Adapter").
        if (wcsstr(dev.DeviceString, L"Virtual Display Driver") != NULL ||
            wcsstr(dev.DeviceString, L"MttVDD") != NULL) {
            wcsncpy(out, dev.DeviceName, outChars - 1);
            out[outChars - 1] = 0;
            return 1;
        }
        ZeroMemory(&dev, sizeof(dev));
        dev.cb = sizeof(dev);
    }
    return 0;
}

// isAlreadyPrimary returns 1 if the device at position (0,0) is already
// the VDD (i.e. VDD is already primary, no work needed).
static int isAlreadyPrimary(const wchar_t *vddName) {
    DEVMODEW dm;
    ZeroMemory(&dm, sizeof(dm));
    dm.dmSize = sizeof(dm);
    if (!EnumDisplaySettingsExW(vddName, ENUM_CURRENT_SETTINGS, &dm, 0)) return 0;
    // Primary monitor has dmPosition == (0,0).
    return (dm.dmPosition.x == 0 && dm.dmPosition.y == 0) ? 1 : 0;
}

// promoteVddPrimary issues a two-pass ChangeDisplaySettingsEx: first mark
// the VDD as the new primary (position 0,0 with CDS_SET_PRIMARY), then
// apply. The call is registry-updating so it survives resolution changes.
// Returns 0 on success, negative on failure.
static int promoteVddPrimary(char *diagOut, int diagLen) {
    wchar_t vddName[64] = {0};
    if (!findVddDeviceName(vddName, 64)) {
        if (diagOut) snprintf(diagOut, diagLen, "vdd not found in EnumDisplayDevices");
        return -1;
    }

    // Convert UTF-16 name to UTF-8 for the diagnostic message.
    char nameA[64] = {0};
    WideCharToMultiByte(CP_UTF8, 0, vddName, -1, nameA, sizeof(nameA) - 1, NULL, NULL);

    if (isAlreadyPrimary(vddName)) {
        if (diagOut) snprintf(diagOut, diagLen, "vdd %s already primary", nameA);
        return 0;
    }

    DEVMODEW dm;
    ZeroMemory(&dm, sizeof(dm));
    dm.dmSize = sizeof(dm);
    if (!EnumDisplaySettingsExW(vddName, ENUM_CURRENT_SETTINGS, &dm, 0)) {
        if (diagOut) snprintf(diagOut, diagLen, "EnumDisplaySettings(%s) failed err=%lu",
            nameA, GetLastError());
        return -2;
    }

    dm.dmFields |= DM_POSITION;
    dm.dmPosition.x = 0;
    dm.dmPosition.y = 0;

    LONG rc = ChangeDisplaySettingsExW(vddName, &dm, NULL,
        CDS_SET_PRIMARY | CDS_UPDATEREGISTRY | CDS_NORESET, NULL);
    if (rc != DISP_CHANGE_SUCCESSFUL) {
        if (diagOut) snprintf(diagOut, diagLen,
            "ChangeDisplaySettingsEx(%s, SET_PRIMARY|NORESET) = %ld", nameA, rc);
        return -3;
    }
    rc = ChangeDisplaySettingsExW(NULL, NULL, NULL, 0, NULL); // apply
    if (rc != DISP_CHANGE_SUCCESSFUL) {
        if (diagOut) snprintf(diagOut, diagLen,
            "ChangeDisplaySettingsEx(apply) = %ld", rc);
        return -4;
    }
    if (diagOut) snprintf(diagOut, diagLen, "vdd %s promoted to primary", nameA);
    return 0;
}
*/
import "C"

import (
	"fmt"
	"log"
	"unsafe"
)

// vddMakePrimary promotes the MttVDD virtual monitor to the Windows primary
// display. Required on no-user sessions where Winlogon renders on the
// physical adapter (Hyper-V Video) which is invisible to the helper
// process. Logs the outcome but never returns an error — a missing VDD or
// a ChangeDisplaySettings refusal should not block the stream.
func vddMakePrimary() {
	var diag [256]C.char
	rc := int(C.promoteVddPrimary(&diag[0], C.int(len(diag))))
	msg := C.GoString(&diag[0])
	if rc == 0 {
		log.Printf("vdd_primary: %s", msg)
	} else {
		log.Printf("vdd_primary: failed (code %d): %s", rc, msg)
	}
	_ = unsafe.Sizeof(rc) // silence unused warning paranoia
	_ = fmt.Sprint(rc)
}
