//go:build windows

package main

/*
#cgo LDFLAGS: -lgdi32 -luser32 -ldwmapi

#include <windows.h>
#include <dwmapi.h>
#include <stdlib.h>
#include <string.h>

#define WM_PILL_W 92
#define WM_PILL_H 30
#define WM_USER_SET_MODE (WM_USER + 1)

// ── Undocumented acrylic blur (SetWindowCompositionAttribute) ───────────────
// Produces the frosted-glass effect used by the Windows 10+ Start menu and
// notification center. ACCENT_ENABLE_ACRYLICBLURBEHIND requires Win10 1803+;
// we probe via GetProcAddress and fall back to DwmEnableBlurBehindWindow
// (Win7+ Aero blur — less frosted but still translucent).

typedef enum _ACCENT_STATE {
    ACCENT_DISABLED                  = 0,
    ACCENT_ENABLE_GRADIENT           = 1,
    ACCENT_ENABLE_TRANSPARENTGRADIENT = 2,
    ACCENT_ENABLE_BLURBEHIND         = 3,
    ACCENT_ENABLE_ACRYLICBLURBEHIND  = 4,
} ACCENT_STATE;

typedef struct _ACCENT_POLICY {
    ACCENT_STATE AccentState;
    DWORD AccentFlags;
    DWORD GradientColor; // ABGR when used with acrylic
    DWORD AnimationId;
} ACCENT_POLICY;

typedef enum _WINDOWCOMPOSITIONATTRIB {
    WCA_ACCENT_POLICY = 19,
} WINDOWCOMPOSITIONATTRIB;

typedef struct _WINDOWCOMPOSITIONATTRIBDATA {
    WINDOWCOMPOSITIONATTRIB Attribute;
    PVOID  pvData;
    SIZE_T cbData;
} WINDOWCOMPOSITIONATTRIBDATA;

typedef BOOL (WINAPI *pfnSetWindowCompositionAttribute)(HWND, WINDOWCOMPOSITIONATTRIBDATA*);

static void apply_acrylic(HWND hwnd) {
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (!user32) return;
    pfnSetWindowCompositionAttribute swca =
        (pfnSetWindowCompositionAttribute)GetProcAddress(user32, "SetWindowCompositionAttribute");
    if (swca) {
        ACCENT_POLICY ap = {0};
        ap.AccentState = ACCENT_ENABLE_ACRYLICBLURBEHIND;
        ap.AccentFlags = 2; // draw borders
        // ABGR: tint ~ #0d1117 at ~45% opacity → 73,0d,11,17 reversed = 0x73170E0D
        ap.GradientColor = 0x73170E0D;
        WINDOWCOMPOSITIONATTRIBDATA data = { WCA_ACCENT_POLICY, &ap, sizeof(ap) };
        if (swca(hwnd, &data)) return;
    }
    // Fallback: Aero blur (Win7+).
    DWM_BLURBEHIND bb = {0};
    bb.dwFlags  = DWM_BB_ENABLE;
    bb.fEnable  = TRUE;
    bb.hRgnBlur = NULL;
    DwmEnableBlurBehindWindow(hwnd, &bb);
}

#define WM_PILL_RADIUS 15 // height/2 — perfect capsule ends

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

        // Background: thin capsule. The acrylic blur (applied once at window
        // creation via apply_acrylic) provides the frosted-glass look; we
        // still need a GDI fill to define the capsule shape, but keep it
        // subtle — a near-charcoal with low chroma so the blur dominates.
        HBRUSH bgBr = CreateSolidBrush(RGB(22, 27, 34));
        HPEN bgPen = CreatePen(PS_SOLID, 1, RGB(48, 54, 61));
        HBRUSH oldBr = (HBRUSH)SelectObject(hdc, bgBr);
        HPEN   oldPen = (HPEN)SelectObject(hdc, bgPen);
        RoundRect(hdc, 0, 0, WM_PILL_W, WM_PILL_H, WM_PILL_RADIUS*2, WM_PILL_RADIUS*2);
        SelectObject(hdc, oldBr);
        SelectObject(hdc, oldPen);
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
        // Physically shape the window as a capsule via SetWindowRgn. Pixels
        // outside the region are fully absent (not just alpha-blended).
        // This is cleaner than an LWA_COLORKEY hack and plays nicely with
        // SetWindowCompositionAttribute's acrylic blur below.
        HRGN rgn = CreateRoundRectRgn(0, 0, WM_PILL_W + 1, WM_PILL_H + 1,
            WM_PILL_RADIUS * 2, WM_PILL_RADIUS * 2);
        SetWindowRgn(g_watermark, rgn, TRUE);
        SetLayeredWindowAttributes(g_watermark, 0, 230, LWA_ALPHA);
        apply_acrylic(g_watermark);
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
