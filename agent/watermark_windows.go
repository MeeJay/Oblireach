//go:build windows

package main

/*
#cgo LDFLAGS: -lgdi32 -luser32

#include <windows.h>
#include <stdlib.h>
#include <string.h>

#define WM_W 400
#define WM_H 28

static HWND g_watermark = NULL;
static wchar_t g_wm_text[256] = L"";
static HFONT g_wm_font = NULL;

static LRESULT CALLBACK wmWndProc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
    switch (msg) {
    case WM_PAINT: {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        SetBkMode(hdc, TRANSPARENT);
        if (!g_wm_font)
            g_wm_font = CreateFontW(-12, 0, 0, 0, FW_NORMAL, 0, 0, 0,
                DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, DEFAULT_PITCH, L"Segoe UI");
        SelectObject(hdc, g_wm_font);
        SetTextColor(hdc, RGB(255, 255, 255));
        RECT rc = {8, 4, WM_W - 8, WM_H - 4};
        DrawTextW(hdc, g_wm_text, -1, &rc, DT_LEFT | DT_SINGLELINE | DT_END_ELLIPSIS);
        EndPaint(hwnd, &ps);
        return 0;
    }
    case WM_NCHITTEST:
        return HTTRANSPARENT; // click-through
    }
    return DefWindowProcW(hwnd, msg, wp, lp);
}

static void show_watermark(const wchar_t *text) {
    if (g_watermark) return; // already showing
    wcsncpy(g_wm_text, text, 255);

    WNDCLASSEXW wc;
    ZeroMemory(&wc, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = wmWndProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hbrBackground = NULL;
    wc.lpszClassName = L"ObliReachWatermark";
    RegisterClassExW(&wc);

    RECT wa;
    SystemParametersInfoW(SPI_GETWORKAREA, 0, &wa, 0);
    int x = wa.left + 8;
    int y = wa.top + 4;

    g_watermark = CreateWindowExW(
        WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_NOACTIVATE,
        L"ObliReachWatermark", NULL,
        WS_POPUP | WS_VISIBLE,
        x, y, WM_W, WM_H,
        NULL, NULL, wc.hInstance, NULL);

    if (g_watermark) {
        // Semi-transparent black background
        SetLayeredWindowAttributes(g_watermark, 0, 140, LWA_ALPHA);
    }
}

static void hide_watermark(void) {
    if (g_watermark) {
        DestroyWindow(g_watermark);
        g_watermark = NULL;
    }
    if (g_wm_font) {
        DeleteObject(g_wm_font);
        g_wm_font = NULL;
    }
}
*/
import "C"
import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

func showWatermark(operatorName string) {
	text := fmt.Sprintf("Remote session by %s — %s", operatorName, time.Now().Format("15:04"))
	textW, _ := syscall.UTF16FromString(text)
	C.show_watermark((*C.wchar_t)(unsafe.Pointer(&textW[0])))
}

func hideWatermark() {
	C.hide_watermark()
}
