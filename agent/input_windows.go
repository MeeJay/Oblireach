//go:build windows

package main

/*
#cgo LDFLAGS: -luser32

#include <windows.h>

// send_mouse_move: moves mouse to absolute screen coordinates (0..65535 range).
static void send_mouse_move(int screen_w, int screen_h, int x, int y) {
    INPUT inp = {0};
    inp.type = INPUT_MOUSE;
    inp.mi.dwFlags = MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE;
    inp.mi.dx = (LONG)(x * 65535 / (screen_w > 1 ? screen_w - 1 : 1));
    inp.mi.dy = (LONG)(y * 65535 / (screen_h > 1 ? screen_h - 1 : 1));
    SendInput(1, &inp, sizeof(INPUT));
}

// send_mouse_button: press/release a mouse button (button: 0=left, 1=right, 2=middle).
static void send_mouse_button(int button, int down, int screen_w, int screen_h, int x, int y) {
    INPUT inp = {0};
    inp.type = INPUT_MOUSE;
    // Also move to the click position
    inp.mi.dwFlags = MOUSEEVENTF_MOVE | MOUSEEVENTF_ABSOLUTE;
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
    INPUT inp = {0};
    inp.type = INPUT_KEYBOARD;
    inp.ki.wVk = (WORD)vk;
    if (!down) inp.ki.dwFlags = KEYEVENTF_KEYUP;
    SendInput(1, &inp, sizeof(INPUT));
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
*/
import "C"

// g_monOffX/Y are set by the stream code when a monitor is selected,
// so mouse coordinates are offset to the correct monitor.
var g_monOffX, g_monOffY int

func setInputMonitorOffset(x, y int) {
	g_monOffX = x
	g_monOffY = y
}

func inputMouseMove(x, y int) {
	var vw, vh, vox, voy C.int
	C.get_virtual_screen_size(&vw, &vh, &vox, &voy)
	// x/y are relative to the captured monitor. Add monitor offset to get virtual coords.
	absX := g_monOffX + x - int(vox)
	absY := g_monOffY + y - int(voy)
	C.send_mouse_move(C.int(vw), C.int(vh), C.int(absX), C.int(absY))
}

func inputMouseButton(button int, down bool, x, y int) {
	var vw, vh, vox, voy C.int
	C.get_virtual_screen_size(&vw, &vh, &vox, &voy)
	absX := g_monOffX + x - int(vox)
	absY := g_monOffY + y - int(voy)
	d := C.int(0)
	if down { d = 1 }
	C.send_mouse_button(C.int(button), d, C.int(vw), C.int(vh), C.int(absX), C.int(absY))
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
