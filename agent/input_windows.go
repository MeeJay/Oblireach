//go:build windows

package main

/*
#cgo LDFLAGS: -luser32

#include <windows.h>

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

// send_mouse_button: press/release a mouse button (button: 0=left, 1=right, 2=middle).
static void send_mouse_button(int button, int down, int screen_w, int screen_h, int x, int y) {
    switch_to_active_desktop();
    INPUT inp = {0};
    inp.type = INPUT_MOUSE;
    inp.mi.dwFlags = MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE;
    if (g_use_virtual_desk) inp.mi.dwFlags |= MOUSEEVENTF_VIRTUALDESK;
    inp.mi.dx = (LONG)(x * 65535 / (screen_w > 1 ? screen_w - 1 : 1));
    inp.mi.dy = (LONG)(y * 65535 / (screen_h > 1 ? screen_h - 1 : 1));

    switch (button) {
    case 0: inp.mi.dwFlags |= down ? MOUSEEVENTF_LEFTDOWN  : MOUSEEVENTF_LEFTUP;   break;
    case 1: inp.mi.dwFlags |= down ? MOUSEEVENTF_RIGHTDOWN : MOUSEEVENTF_RIGHTUP;  break;
    case 2: inp.mi.dwFlags |= down ? MOUSEEVENTF_MIDDLEDOWN: MOUSEEVENTF_MIDDLEUP; break;
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
// Called periodically (not on every input — cached for 2 seconds).
static HDESK g_currentDesk = NULL;
static DWORD g_deskCheckTime = 0;

static void switch_to_active_desktop(void) {
    DWORD now = GetTickCount();
    if (now - g_deskCheckTime < 2000 && g_currentDesk != NULL) return;
    g_deskCheckTime = now;

    // Try to open the input desktop (the one receiving user input right now)
    HDESK inputDesk = OpenInputDesktop(0, FALSE, GENERIC_ALL);
    if (!inputDesk) return;

    // If it's different from our current desktop, switch
    if (g_currentDesk != inputDesk) {
        SetThreadDesktop(inputDesk);
        if (g_currentDesk) CloseDesktop(g_currentDesk);
        g_currentDesk = inputDesk;
    } else {
        CloseDesktop(inputDesk);
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
import "unsafe"

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
