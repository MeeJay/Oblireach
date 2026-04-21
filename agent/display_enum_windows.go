//go:build windows

package main

/*
#cgo LDFLAGS: -luser32 -lgdi32

#include <windows.h>
#include <string.h>

// get_display_device returns at outName/outDesc the DeviceName and
// DeviceString of the n-th adapter (EnumDisplayDevicesW). Returns 1 on
// success, 0 if n out of range.
static int get_display_device(int idx, wchar_t *outName, int nameLen, wchar_t *outDesc, int descLen) {
    DISPLAY_DEVICEW dev;
    ZeroMemory(&dev, sizeof(dev));
    dev.cb = sizeof(dev);
    if (!EnumDisplayDevicesW(NULL, (DWORD)idx, &dev, 0)) return 0;
    wcsncpy(outName, dev.DeviceName, nameLen - 1);
    outName[nameLen - 1] = 0;
    wcsncpy(outDesc, dev.DeviceString, descLen - 1);
    outDesc[descLen - 1] = 0;
    return 1;
}

// display_current_rect returns the current desktop coordinates of the given
// adapter (via EnumDisplaySettingsExW ENUM_CURRENT_SETTINGS). 1 on success.
static int display_current_rect(const wchar_t *deviceName,
                                 int *out_x, int *out_y, int *out_w, int *out_h) {
    DEVMODEW dm;
    ZeroMemory(&dm, sizeof(dm));
    dm.dmSize = sizeof(dm);
    if (!EnumDisplaySettingsExW(deviceName, ENUM_CURRENT_SETTINGS, &dm, 0)) return 0;
    *out_x = dm.dmPosition.x;
    *out_y = dm.dmPosition.y;
    *out_w = dm.dmPelsWidth;
    *out_h = dm.dmPelsHeight;
    return 1;
}
*/
import "C"

import (
	"syscall"
	"unsafe"
)

// enumDisplayDeviceAt wraps EnumDisplayDevicesW(idx) and returns name+desc.
func enumDisplayDeviceAt(idx uint32) (name, desc string, ok bool) {
	var n, d [128]uint16
	r := int(C.get_display_device(
		C.int(idx),
		(*C.wchar_t)(unsafe.Pointer(&n[0])), C.int(len(n)),
		(*C.wchar_t)(unsafe.Pointer(&d[0])), C.int(len(d)),
	))
	if r == 0 {
		return "", "", false
	}
	return syscall.UTF16ToString(n[:]), syscall.UTF16ToString(d[:]), true
}

// displayCurrentRect returns the (x, y, w, h) of the given "\\.\DISPLAYn".
func displayCurrentRect(deviceName string) (x, y, w, h int, ok bool) {
	nameW, err := syscall.UTF16PtrFromString(deviceName)
	if err != nil {
		return 0, 0, 0, 0, false
	}
	var cx, cy, cw, ch C.int
	r := int(C.display_current_rect(
		(*C.wchar_t)(unsafe.Pointer(nameW)),
		&cx, &cy, &cw, &ch,
	))
	if r == 0 {
		return 0, 0, 0, 0, false
	}
	return int(cx), int(cy), int(cw), int(ch), true
}
