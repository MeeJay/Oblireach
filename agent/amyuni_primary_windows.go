//go:build windows

package main

// amyuniMakePrimary promotes the USB Mobile Monitor Virtual Display (Amyuni)
// to primary display so Windows composes the Winlogon / LogonUI / user
// desktop onto it. Required for no-user session capture: when a physical
// console adapter (Hyper-V Video) is present AND primary, Winlogon renders
// there — invisible to our capture pipeline — and Amyuni stays a blank
// secondary with nothing to magnify.

/*
#cgo LDFLAGS: -luser32

#include <windows.h>
#include <stdio.h>
#include <wchar.h>

// findAmyuniDeviceName writes the \\.\DISPLAYn name of an Amyuni adapter
// into out (UTF-16, nul-terminated). Returns 1 on success, 0 otherwise.
static int findAmyuniDeviceName(wchar_t *out, int outChars) {
    DISPLAY_DEVICEW dev;
    ZeroMemory(&dev, sizeof(dev));
    dev.cb = sizeof(dev);
    for (DWORD i = 0; EnumDisplayDevicesW(NULL, i, &dev, 0); i++) {
        if (wcsstr(dev.DeviceString, L"USB Mobile Monitor") != NULL ||
            wcsstr(dev.DeviceString, L"usbmmidd") != NULL) {
            wcsncpy(out, dev.DeviceName, outChars - 1);
            out[outChars - 1] = 0;
            return 1;
        }
        ZeroMemory(&dev, sizeof(dev));
        dev.cb = sizeof(dev);
    }
    return 0;
}

static int amyuniIsAlreadyPrimary(const wchar_t *name) {
    DEVMODEW dm;
    ZeroMemory(&dm, sizeof(dm));
    dm.dmSize = sizeof(dm);
    if (!EnumDisplaySettingsExW(name, ENUM_CURRENT_SETTINGS, &dm, 0)) return 0;
    return (dm.dmPosition.x == 0 && dm.dmPosition.y == 0) ? 1 : 0;
}

// promoteAmyuniPrimary moves the Amyuni monitor to (0,0) with CDS_SET_PRIMARY
// then applies. Winlogon / compositor routes rendering to whichever monitor
// is at (0,0). diagOut receives a short string for logging.
// Returns 0 ok, negative on failure.
static int promoteAmyuniPrimary(char *diagOut, int diagLen) {
    wchar_t name[64] = {0};
    if (!findAmyuniDeviceName(name, 64)) {
        if (diagOut) snprintf(diagOut, diagLen, "amyuni not found in EnumDisplayDevices");
        return -1;
    }
    char nameA[64] = {0};
    WideCharToMultiByte(CP_UTF8, 0, name, -1, nameA, sizeof(nameA) - 1, NULL, NULL);

    if (amyuniIsAlreadyPrimary(name)) {
        if (diagOut) snprintf(diagOut, diagLen, "amyuni %s already primary", nameA);
        return 0;
    }

    DEVMODEW dm;
    ZeroMemory(&dm, sizeof(dm));
    dm.dmSize = sizeof(dm);
    if (!EnumDisplaySettingsExW(name, ENUM_CURRENT_SETTINGS, &dm, 0)) {
        if (diagOut) snprintf(diagOut, diagLen, "EnumDisplaySettings(%s) failed err=%lu",
            nameA, GetLastError());
        return -2;
    }

    dm.dmFields |= DM_POSITION;
    dm.dmPosition.x = 0;
    dm.dmPosition.y = 0;

    LONG rc = ChangeDisplaySettingsExW(name, &dm, NULL,
        CDS_SET_PRIMARY | CDS_UPDATEREGISTRY | CDS_NORESET, NULL);
    if (rc != DISP_CHANGE_SUCCESSFUL) {
        if (diagOut) snprintf(diagOut, diagLen,
            "ChangeDisplaySettingsEx(%s, SET_PRIMARY|NORESET) = %ld", nameA, rc);
        return -3;
    }
    rc = ChangeDisplaySettingsExW(NULL, NULL, NULL, 0, NULL);
    if (rc != DISP_CHANGE_SUCCESSFUL) {
        if (diagOut) snprintf(diagOut, diagLen, "ChangeDisplaySettingsEx(apply) = %ld", rc);
        return -4;
    }
    if (diagOut) snprintf(diagOut, diagLen, "amyuni %s promoted to primary", nameA);
    return 0;
}
*/
import "C"

import (
	"log"
)

func amyuniMakePrimary() {
	var diag [256]C.char
	rc := int(C.promoteAmyuniPrimary(&diag[0], C.int(len(diag))))
	msg := C.GoString(&diag[0])
	if rc == 0 {
		log.Printf("amyuni_primary: %s", msg)
	} else {
		log.Printf("amyuni_primary: failed (code %d): %s", rc, msg)
	}
}
