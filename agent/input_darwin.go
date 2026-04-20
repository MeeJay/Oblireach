//go:build darwin

package main

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation

#include <CoreGraphics/CoreGraphics.h>

static void mac_mouse_move(int x, int y) {
    CGEventRef ev = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved,
        CGPointMake((CGFloat)x, (CGFloat)y), kCGMouseButtonLeft);
    if (ev) { CGEventPost(kCGHIDEventTap, ev); CFRelease(ev); }
}

static void mac_mouse_button(int button, int down, int x, int y) {
    CGEventType type;
    CGMouseButton btn = kCGMouseButtonLeft;
    if (button == 0) {
        type = down ? kCGEventLeftMouseDown : kCGEventLeftMouseUp;
        btn = kCGMouseButtonLeft;
    } else if (button == 1) {
        type = down ? kCGEventRightMouseDown : kCGEventRightMouseUp;
        btn = kCGMouseButtonRight;
    } else {
        type = down ? kCGEventOtherMouseDown : kCGEventOtherMouseUp;
        btn = kCGMouseButtonCenter;
    }
    CGEventRef ev = CGEventCreateMouseEvent(NULL, type,
        CGPointMake((CGFloat)x, (CGFloat)y), btn);
    if (ev) { CGEventPost(kCGHIDEventTap, ev); CFRelease(ev); }
}

static void mac_mouse_scroll(int delta) {
    CGEventRef ev = CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitLine, 1, delta);
    if (ev) { CGEventPost(kCGHIDEventTap, ev); CFRelease(ev); }
}

static void mac_key(int keycode, int down) {
    CGEventRef ev = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keycode, down ? true : false);
    if (ev) { CGEventPost(kCGHIDEventTap, ev); CFRelease(ev); }
}

static void mac_type_char(unsigned int utf32) {
    CGEventRef ev = CGEventCreateKeyboardEvent(NULL, 0, true);
    if (!ev) return;
    UniChar uc = (UniChar)utf32;
    CGEventKeyboardSetUnicodeString(ev, 1, &uc);
    CGEventPost(kCGHIDEventTap, ev);
    CFRelease(ev);
    // Key up
    ev = CGEventCreateKeyboardEvent(NULL, 0, false);
    if (ev) { CGEventPost(kCGHIDEventTap, ev); CFRelease(ev); }
}
*/
import "C"

var g_monOffX, g_monOffY int

func setInputMonitorOffset(x, y int) {
	g_monOffX = x
	g_monOffY = y
}

func inputMouseMove(x, y int) {
	C.mac_mouse_move(C.int(g_monOffX+x), C.int(g_monOffY+y))
}

func inputMouseButton(button int, down bool, x, y int) {
	C.mac_mouse_move(C.int(g_monOffX+x), C.int(g_monOffY+y))
	d := C.int(0)
	if down {
		d = 1
	}
	C.mac_mouse_button(C.int(button), d, C.int(g_monOffX+x), C.int(g_monOffY+y))
}

func inputMouseScroll(delta int) {
	C.mac_mouse_scroll(C.int(delta))
}

func inputKey(vk int, down bool) {
	// Map Windows VK codes to macOS keycodes
	kc := vkToMacKeycode(vk)
	if kc < 0 {
		return
	}
	d := C.int(0)
	if down {
		d = 1
	}
	C.mac_key(C.int(kc), d)
}

func inputSAS() {} // not applicable on macOS

func inputVKFromKey(key string) (int, int) {
	runes := []rune(key)
	if len(runes) != 1 {
		return 0, 0
	}
	// On macOS, we type Unicode characters directly
	return int(runes[0]), 0
}

func clipboardGet() string  { return "" } // TODO: NSPasteboard
func clipboardSet(text string) {}         // TODO: NSPasteboard
func inputBlock(block bool) {}
func inputUnblock()         {}

func inputSwitchActiveDesktop() {} // Windows-only concept
func inputAttachToDefaultDesktop() string { return "" }
func logInputEvent(msg string)            {}

// vkToMacKeycode maps Windows VK codes to macOS key codes.
func vkToMacKeycode(vk int) int {
	m := map[int]int{
		0x08: 51,  // Backspace
		0x09: 48,  // Tab
		0x0D: 36,  // Return
		0x1B: 53,  // Escape
		0x20: 49,  // Space
		0x25: 123, // Left
		0x26: 126, // Up
		0x27: 124, // Right
		0x28: 125, // Down
		0x2E: 117, // Delete (forward)
		0x24: 115, // Home
		0x23: 119, // End
		0x21: 116, // PageUp
		0x22: 121, // PageDown
		0x70: 122, // F1
		0x71: 120, // F2
		0x72: 99,  // F3
		0x73: 118, // F4
		0x74: 96,  // F5
		0x75: 97,  // F6
		0x76: 98,  // F7
		0x77: 100, // F8
		0x78: 101, // F9
		0x79: 109, // F10
		0x7A: 103, // F11
		0x7B: 111, // F12
		0x10: 56,  // Shift
		0x11: 59,  // Control
		0x12: 58,  // Option/Alt
		0x5B: 55,  // Command
	}
	if kc, ok := m[vk]; ok {
		return kc
	}
	// A-Z → macOS keycodes 0-11, 13-46 (QWERTY layout)
	qwerty := map[int]int{
		0x41: 0, 0x42: 11, 0x43: 8, 0x44: 2, 0x45: 14, 0x46: 3,
		0x47: 5, 0x48: 4, 0x49: 34, 0x4A: 38, 0x4B: 40, 0x4C: 37,
		0x4D: 46, 0x4E: 45, 0x4F: 31, 0x50: 35, 0x51: 12, 0x52: 15,
		0x53: 1, 0x54: 17, 0x55: 32, 0x56: 9, 0x57: 13, 0x58: 7,
		0x59: 16, 0x5A: 6,
	}
	if kc, ok := qwerty[vk]; ok {
		return kc
	}
	// 0-9
	digits := map[int]int{
		0x30: 29, 0x31: 18, 0x32: 19, 0x33: 20, 0x34: 21,
		0x35: 23, 0x36: 22, 0x37: 26, 0x38: 28, 0x39: 25,
	}
	if kc, ok := digits[vk]; ok {
		return kc
	}
	return -1
}
