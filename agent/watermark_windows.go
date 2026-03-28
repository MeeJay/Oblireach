//go:build windows

package main

/*
#cgo LDFLAGS: -lgdi32 -luser32

#include <windows.h>
#include <stdlib.h>
#include <string.h>

#define WM_PILL_W 90
#define WM_PILL_H 28
#define WM_USER_SET_MODE (WM_USER + 1)

static HWND g_watermark = NULL;
static HFONT g_wm_font = NULL;
static int g_wm_recording = 0; // 0 = LIVE, 1 = REC

static LRESULT CALLBACK wmWndProc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
    switch (msg) {
    case WM_USER_SET_MODE:
        g_wm_recording = (int)wp;
        InvalidateRect(hwnd, NULL, TRUE);
        return 0;
    case WM_PAINT: {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);

        // Background: dark semi-transparent rounded pill
        HBRUSH bgBr = CreateSolidBrush(RGB(20, 20, 30));
        HPEN bgPen = CreatePen(PS_SOLID, 1, RGB(20, 20, 30));
        SelectObject(hdc, bgBr);
        SelectObject(hdc, bgPen);
        RoundRect(hdc, 0, 0, WM_PILL_W, WM_PILL_H, 14, 14);
        DeleteObject(bgBr);
        DeleteObject(bgPen);

        // Dot color: red for REC, green for LIVE
        COLORREF dotColor = g_wm_recording ? RGB(239, 68, 68) : RGB(74, 222, 128);
        HBRUSH dotBr = CreateSolidBrush(dotColor);
        HPEN dotPen = CreatePen(PS_SOLID, 1, dotColor);
        SelectObject(hdc, dotBr);
        SelectObject(hdc, dotPen);
        Ellipse(hdc, 10, 8, 22, 20);
        DeleteObject(dotBr);
        DeleteObject(dotPen);

        // Text
        SetBkMode(hdc, TRANSPARENT);
        if (!g_wm_font)
            g_wm_font = CreateFontW(-12, 0, 0, 0, FW_BOLD, 0, 0, 0,
                DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, DEFAULT_PITCH, L"Segoe UI");
        SelectObject(hdc, g_wm_font);
        SetTextColor(hdc, dotColor);
        RECT rc = {26, 5, WM_PILL_W - 4, WM_PILL_H - 4};
        DrawTextW(hdc, g_wm_recording ? L"REC" : L"LIVE", -1, &rc, DT_LEFT | DT_SINGLELINE);

        EndPaint(hwnd, &ps);
        return 0;
    }
    case WM_NCHITTEST:
        return HTTRANSPARENT;
    }
    return DefWindowProcW(hwnd, msg, wp, lp);
}

static void show_watermark_rec(void) {
    if (g_watermark) return;

    WNDCLASSEXW wc;
    ZeroMemory(&wc, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = wmWndProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hbrBackground = NULL;
    wc.lpszClassName = L"OblireachRec";
    RegisterClassExW(&wc);

    RECT wa;
    SystemParametersInfoW(SPI_GETWORKAREA, 0, &wa, 0);
    int x = wa.right - WM_PILL_W - 12;
    int y = wa.top + 8;

    g_watermark = CreateWindowExW(
        WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_NOACTIVATE,
        L"OblireachRec", NULL,
        WS_POPUP | WS_VISIBLE,
        x, y, WM_PILL_W, WM_PILL_H,
        NULL, NULL, wc.hInstance, NULL);

    if (g_watermark) {
        SetLayeredWindowAttributes(g_watermark, 0, 200, LWA_ALPHA);
    }
}

// set_watermark_mode: switch between LIVE (0) and REC (1).
// Safe to call from any thread.
static void set_watermark_mode(int recording) {
    if (g_watermark) {
        PostMessageW(g_watermark, WM_USER_SET_MODE, (WPARAM)recording, 0);
    }
}

static void hide_watermark_rec(void) {
    if (g_watermark) {
        DestroyWindow(g_watermark);
        g_watermark = NULL;
    }
    if (g_wm_font) {
        DeleteObject(g_wm_font);
        g_wm_font = NULL;
    }
}
// g_watermark_tid: thread ID of the watermark message pump thread.
static DWORD g_watermark_tid = 0;

// watermark_run: creates the watermark and runs a message pump.
// Blocks until WM_QUIT is posted to this thread.
static void watermark_run(void) {
    g_watermark_tid = GetCurrentThreadId();
    show_watermark_rec();
    MSG msg;
    while (GetMessage(&msg, NULL, 0, 0) > 0) {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    hide_watermark_rec();
    g_watermark_tid = 0;
}

// stop_watermark_pump: posts WM_QUIT to the watermark thread.
// Safe to call from any thread.
static void stop_watermark_pump(void) {
    DWORD tid = g_watermark_tid;
    if (tid != 0) {
        PostThreadMessageW(tid, WM_QUIT, 0, 0);
    }
}
*/
import "C"

import "runtime"

var watermarkRunning bool

func showWatermark(operatorName string) {
	if watermarkRunning {
		return
	}
	watermarkRunning = true
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		C.watermark_run() // blocks until stop_watermark_pump is called
		watermarkRunning = false
	}()
}

func hideWatermark() {
	if watermarkRunning {
		C.stop_watermark_pump()
	}
}

func setWatermarkRecording(recording bool) {
	if watermarkRunning {
		v := C.int(0)
		if recording {
			v = 1
		}
		C.set_watermark_mode(v)
	}
}
