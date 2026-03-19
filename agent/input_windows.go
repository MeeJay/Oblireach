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

// get_screen_size: fills *w and *h with GetSystemMetrics dimensions.
static void get_screen_size(int *w, int *h) {
    *w = GetSystemMetrics(SM_CXSCREEN);
    *h = GetSystemMetrics(SM_CYSCREEN);
}
*/
import "C"

func inputScreenSize() (w, h int) {
	var cw, ch C.int
	C.get_screen_size(&cw, &ch)
	return int(cw), int(ch)
}

func inputMouseMove(x, y int) {
	sw, sh := inputScreenSize()
	C.send_mouse_move(C.int(sw), C.int(sh), C.int(x), C.int(y))
}

func inputMouseButton(button int, down bool, x, y int) {
	sw, sh := inputScreenSize()
	d := C.int(0)
	if down {
		d = 1
	}
	C.send_mouse_button(C.int(button), d, C.int(sw), C.int(sh), C.int(x), C.int(y))
}

func inputMouseScroll(delta int) {
	C.send_mouse_scroll(C.int(delta))
}

func inputKey(vk int, down bool) {
	d := C.int(0)
	if down {
		d = 1
	}
	C.send_key(C.int(vk), d)
}
