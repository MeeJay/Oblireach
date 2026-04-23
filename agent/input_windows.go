//go:build windows

package main

/*
#cgo LDFLAGS: -luser32

#include <windows.h>
#include <stdio.h>
#include <stdarg.h>

// Input diagnostic log — same pattern as mag log, flushes per line so we
// can trace even when SendInput silently no-ops (most common on desktop
// mismatch).
static void inputLog(const char *fmt, ...) {
    FILE *f = fopen("C:\\Windows\\Temp\\oblireach-input.log", "a");
    if (!f) return;
    SYSTEMTIME st; GetLocalTime(&st);
    fprintf(f, "[%04d-%02d-%02d %02d:%02d:%02d.%03d pid=%lu tid=%lu] ",
        st.wYear, st.wMonth, st.wDay, st.wHour, st.wMinute, st.wSecond, st.wMilliseconds,
        (unsigned long)GetCurrentProcessId(), (unsigned long)GetCurrentThreadId());
    va_list ap; va_start(ap, fmt);
    vfprintf(f, fmt, ap);
    va_end(ap);
    fputc('\n', f);
    fflush(f);
    fclose(f);
}

// inputLogGo: non-variadic wrapper callable from Go via CGo (Go can't call
// C varargs functions).
static void inputLogGo(const char *msg) {
    inputLog("%s", msg);
}

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
    UINT sent = SendInput(1, &inp, sizeof(INPUT));
    static int log_count = 0;
    if (log_count < 5 || log_count % 100 == 0) {
        inputLog("mouse_move: sent=%u err=%lu x=%d y=%d screen=%dx%d vdk=%d",
            sent, sent ? 0 : GetLastError(), x, y, screen_w, screen_h, g_use_virtual_desk);
    }
    log_count++;
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

// nudge_input_for_display_wake: relative zero-delta mouse move. Produces no
// visible cursor motion but counts as user input to the display driver,
// which brings a sleeping Hyper-V Video framebuffer back online. Used
// alongside SetThreadExecutionState(ES_DISPLAY_REQUIRED) at helper start.
static void nudge_input_for_display_wake(void) {
    INPUT inp = {0};
    inp.type = INPUT_MOUSE;
    inp.mi.dwFlags = MOUSEEVENTF_MOVE; // relative, no absolute flag, dx=dy=0
    inp.mi.dx = 0;
    inp.mi.dy = 0;
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
    UINT sent = SendInput(1, &inp, sizeof(INPUT));
    inputLog("send_key: vk=0x%02x down=%d sent=%u err=%lu",
        vk, down, sent, sent ? 0 : GetLastError());
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
    DWORD openErr = inputDesk ? 0 : GetLastError();
    if (!inputDesk) {
        // Fallback: try with minimal rights (enough for SetThreadDesktop + SendInput)
        inputDesk = OpenInputDesktop(0, FALSE, DESKTOP_SWITCHDESKTOP);
    }
    if (!inputDesk) {
        inputLog("switch_desk: OpenInputDesktop failed fullAccess_err=%lu minAccess_err=%lu",
            openErr, GetLastError());
        return;
    }

    char name[128] = {0};
    DWORD needed = 0;
    GetUserObjectInformationA(inputDesk, UOI_NAME, name, sizeof(name), &needed);

    // Per-thread attachment: SetThreadDesktop moves THIS thread's desktop.
    if (t_currentDesk != inputDesk) {
        BOOL ok = SetThreadDesktop(inputDesk);
        DWORD setErr = ok ? 0 : GetLastError();
        if (t_currentDesk) CloseDesktop(t_currentDesk);
        t_currentDesk = inputDesk;
        g_currentDesk = inputDesk; // mirror for diagnostics
        inputLog("switch_desk: attached to '%s' (SetThreadDesktop ok=%d err=%lu)",
            name, ok, setErr);
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
	"fmt"
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
// Must be called from the Session 0 service process: SendSAS from Session N
// (even with a SYSTEM token) returns TRUE but no secure-desktop transition
// occurs. The helper redirects here via pipeTypeSAS.
//
// Also defensively ensures HKLM\...\Policies\System!SoftwareSASGeneration is
// set to 1 (or 3) — if it's 0 or missing, SendSAS silently no-ops. We flip
// it for the call and restore after. Same belt-and-suspenders pattern as
// RustDesk's send_sas() in src/platform/windows.rs.
func inputSAS() {
	origValue, origPresent, restore := ensureSASPolicy()
	defer restore()
	_ = origValue
	_ = origPresent

	dll := syscall.NewLazyDLL("sas.dll")
	proc := dll.NewProc("SendSAS")
	findErr := proc.Find()
	if findErr != nil {
		logInputEvent(fmt.Sprintf("SAS: sas.dll SendSAS unavailable: %v", findErr))
		return
	}
	r1, _, callErr := proc.Call(0) // FALSE = called from service (not user)
	logInputEvent(fmt.Sprintf("SAS: SendSAS(FALSE) → r1=%d err=%v", r1, callErr))
}

// ensureSASPolicy reads HKLM\Software\Microsoft\Windows\CurrentVersion\Policies\System!SoftwareSASGeneration,
// promotes it to 1 if needed, and returns a restore func that writes back the
// original value (or deletes it if originally absent). Returns (origValue,
// origPresent, restore). origValue is meaningless when origPresent is false.
//
// Values: 0 = none, 1 = services only, 2 = ease-of-access only, 3 = both.
// SendSAS requires 1 or 3.
func ensureSASPolicy() (uint32, bool, func()) {
	const subKey = `Software\Microsoft\Windows\CurrentVersion\Policies\System`
	const valName = "SoftwareSASGeneration"

	k, _, err := registryCreateKey(subKey)
	if err != nil {
		logInputEvent(fmt.Sprintf("SAS: open policy key failed: %v", err))
		return 0, false, func() {}
	}
	defer registryCloseKey(k)

	orig, present, _ := registryGetDWORD(k, valName)
	if present && (orig == 1 || orig == 3) {
		return orig, true, func() {} // already allowed, nothing to do
	}
	if err := registrySetDWORD(k, valName, 1); err != nil {
		logInputEvent(fmt.Sprintf("SAS: set SoftwareSASGeneration=1 failed: %v", err))
		return orig, present, func() {}
	}
	logInputEvent(fmt.Sprintf("SAS: temporarily set SoftwareSASGeneration 0x%x→1 (was present=%v)", orig, present))
	return orig, present, func() {
		kk, _, err := registryOpenKey(subKey)
		if err != nil {
			return
		}
		defer registryCloseKey(kk)
		if present {
			_ = registrySetDWORD(kk, valName, orig)
		} else {
			_ = registryDeleteValue(kk, valName)
		}
	}
}

// ── Minimal registry helpers (syscall → advapi32) ───────────────────────────
// Used by ensureSASPolicy to flip SoftwareSASGeneration. Kept inline to avoid
// a go.mod bump for golang.org/x/sys/windows/registry.

const (
	_HKEY_LOCAL_MACHINE   = 0x80000002
	_KEY_READ             = 0x20019
	_KEY_SET_VALUE        = 0x0002
	_REG_DWORD            = 4
	_REG_OPTION_NON_VOLATILE = 0
)

var (
	advapi32                = syscall.NewLazyDLL("advapi32.dll")
	procRegCreateKeyExW     = advapi32.NewProc("RegCreateKeyExW")
	procRegOpenKeyExW       = advapi32.NewProc("RegOpenKeyExW")
	procRegCloseKey         = advapi32.NewProc("RegCloseKey")
	procRegQueryValueExW    = advapi32.NewProc("RegQueryValueExW")
	procRegSetValueExW      = advapi32.NewProc("RegSetValueExW")
	procRegDeleteValueW     = advapi32.NewProc("RegDeleteValueW")
)

func registryCreateKey(subKey string) (uintptr, bool, error) {
	p, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return 0, false, err
	}
	var hKey uintptr
	var disp uint32
	r, _, callErr := procRegCreateKeyExW.Call(
		uintptr(_HKEY_LOCAL_MACHINE),
		uintptr(unsafe.Pointer(p)),
		0, 0,
		uintptr(_REG_OPTION_NON_VOLATILE),
		uintptr(_KEY_READ|_KEY_SET_VALUE),
		0,
		uintptr(unsafe.Pointer(&hKey)),
		uintptr(unsafe.Pointer(&disp)),
	)
	if r != 0 {
		return 0, false, fmt.Errorf("RegCreateKeyExW: %v (code=%d)", callErr, r)
	}
	return hKey, disp == 1, nil // disp=1 = REG_CREATED_NEW_KEY
}

func registryOpenKey(subKey string) (uintptr, bool, error) {
	p, err := syscall.UTF16PtrFromString(subKey)
	if err != nil {
		return 0, false, err
	}
	var hKey uintptr
	r, _, callErr := procRegOpenKeyExW.Call(
		uintptr(_HKEY_LOCAL_MACHINE),
		uintptr(unsafe.Pointer(p)),
		0,
		uintptr(_KEY_READ|_KEY_SET_VALUE),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if r != 0 {
		return 0, false, fmt.Errorf("RegOpenKeyExW: %v (code=%d)", callErr, r)
	}
	return hKey, true, nil
}

func registryCloseKey(hKey uintptr) {
	procRegCloseKey.Call(hKey)
}

func registryGetDWORD(hKey uintptr, name string) (uint32, bool, error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, false, err
	}
	var regType uint32
	var data uint32
	size := uint32(4)
	r, _, _ := procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(p)),
		0,
		uintptr(unsafe.Pointer(&regType)),
		uintptr(unsafe.Pointer(&data)),
		uintptr(unsafe.Pointer(&size)),
	)
	if r != 0 {
		return 0, false, nil // missing value — not an error for our use
	}
	if regType != _REG_DWORD {
		return 0, false, fmt.Errorf("value is not REG_DWORD (type=%d)", regType)
	}
	return data, true, nil
}

func registrySetDWORD(hKey uintptr, name string, value uint32) error {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	v := value
	r, _, callErr := procRegSetValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(p)),
		0,
		uintptr(_REG_DWORD),
		uintptr(unsafe.Pointer(&v)),
		4,
	)
	if r != 0 {
		return fmt.Errorf("RegSetValueExW: %v (code=%d)", callErr, r)
	}
	return nil
}

func registryDeleteValue(hKey uintptr, name string) error {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	r, _, callErr := procRegDeleteValueW.Call(
		hKey,
		uintptr(unsafe.Pointer(p)),
	)
	if r != 0 {
		return fmt.Errorf("RegDeleteValueW: %v (code=%d)", callErr, r)
	}
	return nil
}

// logInputEvent writes a short line to the shared input log so we can trace
// what the helper is actually doing with operator input.
func logInputEvent(msg string) {
	cstr := C.CString(msg)
	C.inputLogGo(cstr)
	C.free(unsafe.Pointer(cstr))
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

// inputKeepDisplayAwake tells Windows to keep the display powered on for
// the duration of this process. Required when capturing the console session
// of a Hyper-V VM (or any physical host whose monitor-timeout power policy
// has kicked in): DXGI Desktop Duplication against a "sleeping" display
// returns a blank/stale framebuffer, so the operator sees a never-
// transitioning "Waiting for agent to connect…" because init is sent but no
// frame ever decodes. Opening Hyper-V Manager's console window or launching
// RustDesk had the same wake effect — both hold ES_DISPLAY_REQUIRED.
//
// Must be called from a thread in the target session (the helper's thread
// after spawn is correct). The state persists until the process exits or
// another SetThreadExecutionState(ES_CONTINUOUS) resets it.
//
// Also injects a zero-delta mouse move: some display drivers (Hyper-V
// Video specifically) need an input event on the wire to bring the
// framebuffer out of its idle power state, not just a policy flag.
func inputKeepDisplayAwake() {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	proc := k32.NewProc("SetThreadExecutionState")
	const (
		esContinuous      = 0x80000000
		esDisplayRequired = 0x00000002
		esSystemRequired  = 0x00000001
	)
	r1, _, callErr := proc.Call(uintptr(esContinuous | esDisplayRequired | esSystemRequired))
	logInputEvent(fmt.Sprintf("wake: SetThreadExecutionState → prev=0x%x err=%v", r1, callErr))

	// Zero-delta mouse nudge — brings the Hyper-V Video framebuffer out of
	// its idle power state when the ES_DISPLAY_REQUIRED flag alone isn't
	// enough to force rendering.
	C.nudge_input_for_display_wake()
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
