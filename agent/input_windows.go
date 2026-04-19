//go:build windows

package main

/*
#cgo LDFLAGS: -luser32

#include <windows.h>
#include <stdio.h>

// Forward declaration
static void switch_to_active_desktop(void);

// g_use_virtual_desk: set to 1 for multi-monitor, 0 for single monitor.
// MOUSEEVENTF_VIRTUALDESK can be slow on RDP sessions — avoid when not needed.
static int g_use_virtual_desk = 0;

static void set_multi_monitor(int multiMon) {
    g_use_virtual_desk = multiMon;
}

// send_mouse_move: moves mouse to absolute coordinates (0..65535 range).
static void send_mouse_move(int screen_w, int screen_h, int x, int y) {
    switch_to_active_desktop();
    INPUT inp = {0};
    inp.type = INPUT_MOUSE;
    inp.mi.dwFlags = MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE;
    if (g_use_virtual_desk) inp.mi.dwFlags |= MOUSEEVENTF_VIRTUALDESK;
    inp.mi.dx = (LONG)(x * 65535 / (screen_w > 1 ? screen_w - 1 : 1));
    inp.mi.dy = (LONG)(y * 65535 / (screen_h > 1 ? screen_h - 1 : 1));
    SendInput(1, &inp, sizeof(INPUT));
}

// send_mouse_button: press/release a mouse button.
// button values match browser MouseEvent.button: 0=left, 1=middle, 2=right.
static void send_mouse_button(int button, int down, int screen_w, int screen_h, int x, int y) {
    switch_to_active_desktop();
    INPUT inp = {0};
    inp.type = INPUT_MOUSE;
    inp.mi.dwFlags = MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE;
    if (g_use_virtual_desk) inp.mi.dwFlags |= MOUSEEVENTF_VIRTUALDESK;
    inp.mi.dx = (LONG)(x * 65535 / (screen_w > 1 ? screen_w - 1 : 1));
    inp.mi.dy = (LONG)(y * 65535 / (screen_h > 1 ? screen_h - 1 : 1));

    switch (button) {
    case 0: inp.mi.dwFlags |= down ? MOUSEEVENTF_LEFTDOWN   : MOUSEEVENTF_LEFTUP;   break;
    case 1: inp.mi.dwFlags |= down ? MOUSEEVENTF_MIDDLEDOWN : MOUSEEVENTF_MIDDLEUP; break;
    case 2: inp.mi.dwFlags |= down ? MOUSEEVENTF_RIGHTDOWN  : MOUSEEVENTF_RIGHTUP;  break;
    }
    SendInput(1, &inp, sizeof(INPUT));
}

// send_mouse_scroll: wheel delta (positive = up, negative = down), scaled by WHEEL_DELTA.
static void send_mouse_scroll(int delta) {
    INPUT inp = {0};
    inp.type = INPUT_MOUSE;
    inp.mi.dwFlags = MOUSEEVENTF_WHEEL;
    // Each browser "tick" is 3 units; multiply to get WHEEL_DELTA (120)
    inp.mi.mouseData = (DWORD)(delta * WHEEL_DELTA);
    SendInput(1, &inp, sizeof(INPUT));
}

// send_key: press or release a virtual key code.
static void send_key(int vk, int down) {
    switch_to_active_desktop();
    INPUT inp = {0};
    inp.type = INPUT_KEYBOARD;
    inp.ki.wVk = (WORD)vk;
    if (!down) inp.ki.dwFlags = KEYEVENTF_KEYUP;
    SendInput(1, &inp, sizeof(INPUT));
}

// switch_to_active_desktop: ensures the current thread is attached to the
// input desktop (either "Default" for normal use or "Winlogon" for the
// login screen). This allows SendInput to work on the logon screen.
// Called periodically — the cache is THREAD-LOCAL because SetThreadDesktop
// is per-thread; a global cache would let one thread race another into
// seeing a "fresh" g_currentDesk while its own thread is still on an old
// desktop, breaking SendInput for that thread. This was the regression
// introduced when capture + input were split onto separate goroutines.
static __thread HDESK t_currentDesk = NULL;
static __thread DWORD t_deskCheckTime = 0;

// Legacy global (still read by force_switch_active_desktop for backward
// compatibility), but no longer authoritative — the per-thread state is.
static HDESK g_currentDesk = NULL;

static void switch_to_active_desktop(void) {
    DWORD now = GetTickCount();
    if (now - t_deskCheckTime < 500 && t_currentDesk != NULL) return;
    t_deskCheckTime = now;

    // Try to open the input desktop (the one receiving user input right now).
    // Use DESKTOP_SWITCHDESKTOP for the access check — GENERIC_ALL can fail
    // on the Secure Desktop (UAC prompt / Winlogon).
    HDESK inputDesk = OpenInputDesktop(0, FALSE,
        DESKTOP_READOBJECTS | DESKTOP_WRITEOBJECTS | DESKTOP_SWITCHDESKTOP |
        DESKTOP_CREATEWINDOW | DESKTOP_CREATEMENU);
    if (!inputDesk) {
        // Fallback: try with minimal rights (enough for SetThreadDesktop + SendInput)
        inputDesk = OpenInputDesktop(0, FALSE, DESKTOP_SWITCHDESKTOP);
    }
    if (!inputDesk) return;

    // Per-thread attachment: SetThreadDesktop moves THIS thread's desktop.
    if (t_currentDesk != inputDesk) {
        SetThreadDesktop(inputDesk);
        if (t_currentDesk) CloseDesktop(t_currentDesk);
        t_currentDesk = inputDesk;
        g_currentDesk = inputDesk; // mirror for diagnostics
    } else {
        CloseDesktop(inputDesk);
    }
}

// force_switch_active_desktop: bypasses the 500ms cache and re-attaches the
// CALLING thread to the current input desktop. Thread-local so callers
// from different goroutines can't stomp each other's cached view.
static void force_switch_active_desktop(void) {
    t_deskCheckTime = 0;
    switch_to_active_desktop();
}

// attach_to_default_desktop forces the calling process to winsta0 and the
// current thread to the Default desktop. Required for the helper's capture
// thread when spawned with a token (e.g. winlogon.exe) whose primary
// WindowStation is not winsta0 — DXGI Desktop Duplication returns
// E_ACCESSDENIED from the Winlogon WinSta, but succeeds on WinSta0\Default
// even when no user is logged in. Fills outName with "WinSta\Desktop" for
// diagnostic logging.
static void attach_to_default_desktop(char *outName, int outLen) {
    char wsname[128] = {0};
    char deskname[128] = {0};

    HWINSTA hWinsta = OpenWindowStationW(L"winsta0", FALSE,
        WINSTA_ENUMDESKTOPS | WINSTA_READATTRIBUTES | WINSTA_ACCESSCLIPBOARD |
        WINSTA_CREATEDESKTOP | WINSTA_WRITEATTRIBUTES | WINSTA_ACCESSGLOBALATOMS |
        WINSTA_EXITWINDOWS | WINSTA_ENUMERATE | WINSTA_READSCREEN);
    if (!hWinsta) {
        hWinsta = OpenWindowStationW(L"winsta0", FALSE, WINSTA_READATTRIBUTES);
    }
    if (hWinsta) {
        SetProcessWindowStation(hWinsta);
        DWORD size = 0;
        GetUserObjectInformationA(hWinsta, UOI_NAME, wsname, sizeof(wsname), &size);
    }

    HDESK hDesk = OpenDesktopW(L"Default", 0, FALSE,
        DESKTOP_READOBJECTS | DESKTOP_WRITEOBJECTS | DESKTOP_SWITCHDESKTOP |
        DESKTOP_CREATEWINDOW | DESKTOP_CREATEMENU | DESKTOP_HOOKCONTROL |
        DESKTOP_JOURNALRECORD | DESKTOP_JOURNALPLAYBACK | DESKTOP_ENUMERATE);
    if (!hDesk) {
        hDesk = OpenDesktopW(L"Default", 0, FALSE, GENERIC_READ | GENERIC_WRITE);
    }
    if (hDesk) {
        SetThreadDesktop(hDesk);
        if (g_currentDesk) CloseDesktop(g_currentDesk);
        g_currentDesk = hDesk;
        DWORD size = 0;
        GetUserObjectInformationA(hDesk, UOI_NAME, deskname, sizeof(deskname), &size);
    }

    if (outName && outLen > 1) {
        snprintf(outName, outLen, "%s\\%s",
            wsname[0] ? wsname : "?",
            deskname[0] ? deskname : "?");
        outName[outLen-1] = 0;
    }
}

// get_virtual_screen_size: returns the full virtual desktop dimensions.
static void get_virtual_screen_size(int *w, int *h, int *ox, int *oy) {
    *w  = GetSystemMetrics(SM_CXVIRTUALSCREEN);
    *h  = GetSystemMetrics(SM_CYVIRTUALSCREEN);
    *ox = GetSystemMetrics(SM_XVIRTUALSCREEN);
    *oy = GetSystemMetrics(SM_YVIRTUALSCREEN);
}

// block_user_input: calls BlockInput(TRUE) to block local mouse/keyboard.
static BOOL block_user_input(int block) {
    return BlockInput(block ? TRUE : FALSE);
}

// vk_from_char: uses VkKeyScanW to find the VK code for a Unicode character,
// respecting the remote system's keyboard layout.
// Returns the VK code (low byte), or 0 if unmapped.
static int vk_from_char(unsigned short ch) {
    SHORT res = VkKeyScanW((WCHAR)ch);
    if (res == -1) return 0;
    return (int)(res & 0xFF);
}

// vk_shift_from_char: returns modifier flags for the character.
// bit 0 = Shift, bit 1 = Ctrl, bit 2 = Alt
static int vk_mods_from_char(unsigned short ch) {
    SHORT res = VkKeyScanW((WCHAR)ch);
    if (res == -1) return 0;
    return (int)((res >> 8) & 0x07);
}

// clipboard_get_text: reads Unicode text from the clipboard.
// Returns a malloc'd UTF-8 string, or NULL if no text.
static char* clipboard_get_text(void) {
    if (!OpenClipboard(NULL)) return NULL;
    HANDLE hData = GetClipboardData(CF_UNICODETEXT);
    if (!hData) { CloseClipboard(); return NULL; }
    wchar_t *wstr = (wchar_t*)GlobalLock(hData);
    if (!wstr) { CloseClipboard(); return NULL; }
    int len = WideCharToMultiByte(CP_UTF8, 0, wstr, -1, NULL, 0, NULL, NULL);
    char *utf8 = (char*)malloc(len);
    WideCharToMultiByte(CP_UTF8, 0, wstr, -1, utf8, len, NULL, NULL);
    GlobalUnlock(hData);
    CloseClipboard();
    return utf8;
}

// clipboard_set_text: writes UTF-8 text to the clipboard.
static int clipboard_set_text(const char *utf8) {
    int wlen = MultiByteToWideChar(CP_UTF8, 0, utf8, -1, NULL, 0);
    if (wlen <= 0) return -1;
    HGLOBAL hMem = GlobalAlloc(GMEM_MOVEABLE, wlen * sizeof(wchar_t));
    if (!hMem) return -2;
    wchar_t *wstr = (wchar_t*)GlobalLock(hMem);
    MultiByteToWideChar(CP_UTF8, 0, utf8, -1, wstr, wlen);
    GlobalUnlock(hMem);
    if (!OpenClipboard(NULL)) { GlobalFree(hMem); return -3; }
    EmptyClipboard();
    SetClipboardData(CF_UNICODETEXT, hMem);
    CloseClipboard();
    return 0;
}
*/
import "C"

import (
	"syscall"
	"unsafe"
)

// Cached virtual screen dimensions — refreshed once when monitor offset changes.
// Avoids calling GetSystemMetrics on every mouse move (slow on RDP servers).
var (
	g_monOffX, g_monOffY int
	g_vsW, g_vsH         int
	g_vsOX, g_vsOY       int
	g_vsInited           bool
)

func refreshVirtualScreenCache() {
	var vw, vh, vox, voy C.int
	C.get_virtual_screen_size(&vw, &vh, &vox, &voy)
	g_vsW = int(vw)
	g_vsH = int(vh)
	g_vsOX = int(vox)
	g_vsOY = int(voy)
	g_vsInited = true

	// Detect multi-monitor: if virtual screen origin is non-zero or
	// virtual screen is larger than primary, we have multiple monitors.
	primaryW := int(C.GetSystemMetrics(0))  // SM_CXSCREEN
	primaryH := int(C.GetSystemMetrics(1))  // SM_CYSCREEN
	multiMon := g_vsW > primaryW || g_vsH > primaryH || g_vsOX != 0 || g_vsOY != 0
	if multiMon {
		C.set_multi_monitor(1)
	} else {
		C.set_multi_monitor(0)
	}
}

func setInputMonitorOffset(x, y int) {
	g_monOffX = x
	g_monOffY = y
	refreshVirtualScreenCache()
}

func inputMouseMove(x, y int) {
	if !g_vsInited { refreshVirtualScreenCache() }
	absX := g_monOffX + x - g_vsOX
	absY := g_monOffY + y - g_vsOY
	C.send_mouse_move(C.int(g_vsW), C.int(g_vsH), C.int(absX), C.int(absY))
}

func inputMouseButton(button int, down bool, x, y int) {
	if !g_vsInited { refreshVirtualScreenCache() }
	absX := g_monOffX + x - g_vsOX
	absY := g_monOffY + y - g_vsOY
	d := C.int(0)
	if down { d = 1 }
	C.send_mouse_button(C.int(button), d, C.int(g_vsW), C.int(g_vsH), C.int(absX), C.int(absY))
}

func inputMouseScroll(delta int) {
	C.send_mouse_scroll(C.int(delta))
}

func inputKey(vk int, down bool) {
	d := C.int(0)
	if down { d = 1 }
	C.send_key(C.int(vk), d)
}

// inputSAS sends a Secure Attention Sequence (Ctrl+Alt+Del).
// Requires the process to run as SYSTEM (service or helper spawned with SYSTEM token).
func inputSAS() {
	dll := syscall.NewLazyDLL("sas.dll")
	proc := dll.NewProc("SendSAS")
	if proc.Find() == nil {
		proc.Call(0) // FALSE = software SAS
	}
}

// inputVKFromKey converts a browser e.key string (single character) to
// the correct Windows VK code for the remote system's keyboard layout.
// Returns (vk, needsShift). If the character can't be mapped, returns (0, false).
func inputVKFromKey(key string) (vk int, mods int) {
	runes := []rune(key)
	if len(runes) != 1 {
		return 0, 0
	}
	ch := runes[0]
	vk = int(C.vk_from_char(C.ushort(ch)))
	mods = int(C.vk_mods_from_char(C.ushort(ch)))
	return vk, mods
}

func clipboardGet() string {
	cstr := C.clipboard_get_text()
	if cstr == nil {
		return ""
	}
	s := C.GoString(cstr)
	C.free(unsafe.Pointer(cstr))
	return s
}

func clipboardSet(text string) {
	cstr := C.CString(text)
	C.clipboard_set_text(cstr)
	C.free(unsafe.Pointer(cstr))
}

var inputIsBlocked bool

func inputBlock(block bool) {
	b := C.int(0)
	if block { b = 1 }
	C.block_user_input(b)
	inputIsBlocked = block
}

func inputUnblock() {
	if inputIsBlocked {
		C.block_user_input(0)
		inputIsBlocked = false
	}
}

// inputSwitchActiveDesktop re-attaches the current OS thread to whichever
// desktop is currently receiving user input (Default, Winlogon/Secure Desktop,
// or the screensaver). Must be called on a LockOSThread goroutine. Used by
// the capture layer before re-initialising DXGI after DXGI_ERROR_ACCESS_LOST.
func inputSwitchActiveDesktop() {
	C.force_switch_active_desktop()
}

// inputAttachToDefaultDesktop pins the current process to WinSta0 and the
// current OS thread to the Default desktop. Returns "WinSta\Desktop" for
// the logger. Used by the helper's capture thread at startup: DXGI Desktop
// Duplication returns E_ACCESSDENIED from the Winlogon WinSta (which is
// where the helper lands when spawned with winlogon.exe's token) but works
// from WinSta0\Default — even on a session with no user logged in.
func inputAttachToDefaultDesktop() string {
	var buf [320]C.char
	C.attach_to_default_desktop(&buf[0], C.int(len(buf)))
	return C.GoString(&buf[0])
}
