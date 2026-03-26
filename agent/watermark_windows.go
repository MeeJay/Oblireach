//go:build windows

package main

/*
#cgo LDFLAGS: -lgdi32 -luser32

#include <windows.h>
#include <stdlib.h>
#include <string.h>

#define WM_REC_W 90
#define WM_REC_H 28

static HWND g_watermark = NULL;
static HFONT g_wm_font = NULL;

static LRESULT CALLBACK wmWndProc(HWND hwnd, UINT msg, WPARAM wp, LPARAM lp) {
    switch (msg) {
    case WM_PAINT: {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);

        // Background: dark semi-transparent rounded pill
        HBRUSH bgBr = CreateSolidBrush(RGB(20, 20, 30));
        HPEN bgPen = CreatePen(PS_SOLID, 1, RGB(20, 20, 30));
        SelectObject(hdc, bgBr);
        SelectObject(hdc, bgPen);
        RoundRect(hdc, 0, 0, WM_REC_W, WM_REC_H, 14, 14);
        DeleteObject(bgBr);
        DeleteObject(bgPen);

        // Red dot (recording indicator)
        HBRUSH redBr = CreateSolidBrush(RGB(239, 68, 68));
        HPEN redPen = CreatePen(PS_SOLID, 1, RGB(239, 68, 68));
        SelectObject(hdc, redBr);
        SelectObject(hdc, redPen);
        Ellipse(hdc, 10, 8, 22, 20);
        DeleteObject(redBr);
        DeleteObject(redPen);

        // "REC" text
        SetBkMode(hdc, TRANSPARENT);
        if (!g_wm_font)
            g_wm_font = CreateFontW(-12, 0, 0, 0, FW_BOLD, 0, 0, 0,
                DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, DEFAULT_PITCH, L"Segoe UI");
        SelectObject(hdc, g_wm_font);
        SetTextColor(hdc, RGB(239, 68, 68));
        RECT rc = {26, 5, WM_REC_W - 4, WM_REC_H - 4};
        DrawTextW(hdc, L"REC", -1, &rc, DT_LEFT | DT_SINGLELINE);

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
    int x = wa.right - WM_REC_W - 12;
    int y = wa.top + 8;

    g_watermark = CreateWindowExW(
        WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_NOACTIVATE,
        L"OblireachRec", NULL,
        WS_POPUP | WS_VISIBLE,
        x, y, WM_REC_W, WM_REC_H,
        NULL, NULL, wc.hInstance, NULL);

    if (g_watermark) {
        SetLayeredWindowAttributes(g_watermark, 0, 200, LWA_ALPHA);
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
*/
import "C"

func showWatermark(operatorName string) {
	C.show_watermark_rec()
}

func hideWatermark() {
	C.hide_watermark_rec()
}
