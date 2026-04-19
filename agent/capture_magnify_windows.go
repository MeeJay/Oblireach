//go:build windows

package main

// Windows Magnification API capture — RustDesk's hidden fallback path.
//
// The Magnification API (Magnification.dll) is an accessibility framework.
// It captures screen content at a layer that bypasses DXGI Desktop
// Duplication's Secure-Desktop / no-user / token-affinity restrictions.
// When winlogon.exe's token forces DXGI to WARP-only and E_ACCESSDENIED,
// MagSetWindowSource still renders pixels into its callback regardless
// of the context — which is why RustDesk can capture the sign-in screen
// on Windows 10/11 and Server 2016–2022 where plain DXGI fails.
//
// Ported 1:1 from RustDesk's libs/scrap/src/dxgi/mag.rs which itself
// credits shiguredo/libwebrtc screen_capturer_win_magnifier.cc.

/*
#cgo LDFLAGS: -luser32 -lgdi32

#include <windows.h>
#include <stdint.h>
#include <string.h>

// ── Magnification API types (from magnification.h — not in all MinGW SDKs) ──
typedef struct tagMAGIMAGEHEADER {
    UINT   width;
    UINT   height;
    GUID   format;
    UINT   stride;
    UINT   offset;
    SIZE_T cbSize;
} MAGIMAGEHEADER;

typedef BOOL (WINAPI *MagImageScalingCallback)(
    HWND hwnd,
    void *srcdata, MAGIMAGEHEADER srcheader,
    void *destdata, MAGIMAGEHEADER destheader,
    RECT unclipped, RECT clipped, HRGN dirty);

typedef BOOL (WINAPI *PFN_MagInitialize)(void);
typedef BOOL (WINAPI *PFN_MagUninitialize)(void);
typedef BOOL (WINAPI *PFN_MagSetWindowSource)(HWND, RECT);
typedef BOOL (WINAPI *PFN_MagSetImageScalingCallback)(HWND, MAGIMAGEHEADER *);
// Real signature for SetImageScalingCallback takes a callback function:
typedef BOOL (WINAPI *PFN_MagSetImageScalingCallback_Real)(HWND, MagImageScalingCallback);

// WIC 32bpp RGBA GUID: {F5C7AD2D-6A8D-43dd-A7A8-A29935261AE9}
static const GUID GUID_WICPixelFormat32bppRGBA_Local = {
    0xf5c7ad2d, 0x6a8d, 0x43dd, { 0xa7, 0xa8, 0xa2, 0x99, 0x35, 0x26, 0x1a, 0xe9 }
};

static HMODULE g_mag_lib = NULL;
static PFN_MagInitialize              g_mag_init     = NULL;
static PFN_MagUninitialize            g_mag_uninit   = NULL;
static PFN_MagSetWindowSource         g_mag_setsrc   = NULL;
static PFN_MagSetImageScalingCallback_Real g_mag_setcb = NULL;
static HWND g_mag_host = NULL;
static HWND g_mag_magnifier = NULL;
static int  g_mag_w = 0;
static int  g_mag_h = 0;
static RECT g_mag_rect;

// Callback writes here. Protected by the fact that MagSetWindowSource is
// synchronous (callback runs and returns before it does) and we only drive
// it from a single thread (the capture thread).
static unsigned char *g_mag_buffer = NULL;
static SIZE_T         g_mag_buffer_cap = 0;
static SIZE_T         g_mag_buffer_size = 0;
static int            g_mag_buffer_valid = 0;

// Callback invoked by MagSetWindowSource with the captured pixels.
static BOOL WINAPI mag_callback(
    HWND hwnd,
    void *srcdata, MAGIMAGEHEADER srcheader,
    void *destdata, MAGIMAGEHEADER destheader,
    RECT unclipped, RECT clipped, HRGN dirty)
{
    (void)hwnd; (void)destdata; (void)destheader;
    (void)unclipped; (void)clipped; (void)dirty;
    g_mag_buffer_valid = 0;
    if (memcmp(&srcheader.format, &GUID_WICPixelFormat32bppRGBA_Local, sizeof(GUID)) != 0) {
        return FALSE; // unexpected format
    }
    if (srcheader.cbSize > g_mag_buffer_cap) {
        free(g_mag_buffer);
        g_mag_buffer = (unsigned char*)malloc(srcheader.cbSize);
        if (!g_mag_buffer) { g_mag_buffer_cap = 0; return FALSE; }
        g_mag_buffer_cap = srcheader.cbSize;
    }
    memcpy(g_mag_buffer, srcdata, srcheader.cbSize);
    g_mag_buffer_size = srcheader.cbSize;
    g_mag_buffer_valid = 1;
    return TRUE;
}

static int mag_load_library(void) {
    if (g_mag_lib) return 0;
    // LOAD_LIBRARY_SEARCH_SYSTEM32 = 0x00000800
    g_mag_lib = LoadLibraryExA("Magnification.dll", NULL, 0x00000800);
    if (!g_mag_lib) return -1;
    g_mag_init   = (PFN_MagInitialize)            GetProcAddress(g_mag_lib, "MagInitialize");
    g_mag_uninit = (PFN_MagUninitialize)          GetProcAddress(g_mag_lib, "MagUninitialize");
    g_mag_setsrc = (PFN_MagSetWindowSource)       GetProcAddress(g_mag_lib, "MagSetWindowSource");
    g_mag_setcb  = (PFN_MagSetImageScalingCallback_Real)
                     GetProcAddress(g_mag_lib, "MagSetImageScalingCallback");
    if (!g_mag_init || !g_mag_uninit || !g_mag_setsrc || !g_mag_setcb) return -2;
    if (!g_mag_init()) return -3;
    return 0;
}

// mag_init creates the host + magnifier child windows and registers the
// scaling callback. Captures the entire virtual desktop.
// Returns 0 on success, negative error code on failure.
static int mag_init(void) {
    if (mag_load_library() != 0) return -10;

    // Process must be per-monitor DPI-aware for Magnification to work.
    // (SetProcessDpiAwarenessContext if available; fall through if not.)
    {
        HMODULE u32 = GetModuleHandleA("user32.dll");
        if (u32) {
            typedef BOOL (WINAPI *PFN_Aware)(void*);
            PFN_Aware fn = (PFN_Aware)GetProcAddress(u32, "SetProcessDpiAwarenessContext");
            // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
            if (fn) fn((void*)(INT_PTR)-4);
        }
    }

    int x = GetSystemMetrics(SM_XVIRTUALSCREEN);
    int y = GetSystemMetrics(SM_YVIRTUALSCREEN);
    int w = GetSystemMetrics(SM_CXVIRTUALSCREEN);
    int h = GetSystemMetrics(SM_CYVIRTUALSCREEN);
    if (w <= 0 || h <= 0) return -11;

    g_mag_rect.left = x;
    g_mag_rect.top = y;
    g_mag_rect.right = x + w;
    g_mag_rect.bottom = y + h;
    g_mag_w = w;
    g_mag_h = h;

    HINSTANCE hInst = NULL;
    GetModuleHandleExA(2 | 4, (LPCSTR)DefWindowProcA, &hInst); // GET_MODULE_HANDLE_EX_FLAG_FROM_ADDRESS|UNCHANGED_REFCOUNT

    WNDCLASSEXA wcex;
    ZeroMemory(&wcex, sizeof(wcex));
    wcex.cbSize = sizeof(wcex);
    wcex.lpfnWndProc = DefWindowProcA;
    wcex.hInstance = hInst;
    wcex.hCursor = LoadCursorA(NULL, IDC_ARROW);
    wcex.lpszClassName = "ObliReachMagHost";
    if (!RegisterClassExA(&wcex)) {
        DWORD e = GetLastError();
        if (e != ERROR_CLASS_ALREADY_EXISTS) return -12;
    }

    g_mag_host = CreateWindowExA(
        WS_EX_LAYERED, "ObliReachMagHost", "MagHost", WS_POPUP,
        0, 0, 0, 0, NULL, NULL, hInst, NULL);
    if (!g_mag_host) return -13;

    g_mag_magnifier = CreateWindowExA(
        0, "Magnifier", "MagCtl", WS_CHILD | WS_VISIBLE,
        0, 0, 0, 0, g_mag_host, NULL, hInst, NULL);
    if (!g_mag_magnifier) return -14;

    ShowWindow(g_mag_host, SW_HIDE);

    if (!g_mag_setcb(g_mag_magnifier, (MagImageScalingCallback)mag_callback)) {
        return -15;
    }
    return 0;
}

static int mag_width(void)  { return g_mag_w; }
static int mag_height(void) { return g_mag_h; }

// mag_capture_frame captures one frame into out_bgra (w*h*4 bytes).
// Magnification produces RGBA; we swap R/B bytes to BGRA for consistency
// with the rest of the pipeline.
// Returns 0 on success, negative on error.
static int mag_capture_frame(unsigned char *out_bgra) {
    if (!g_mag_magnifier) return -20;
    if (!SetWindowPos(g_mag_magnifier, HWND_TOP,
                       g_mag_rect.left, g_mag_rect.top,
                       g_mag_rect.right - g_mag_rect.left,
                       g_mag_rect.bottom - g_mag_rect.top, 0)) {
        return -21;
    }
    g_mag_buffer_valid = 0;
    if (!g_mag_setsrc(g_mag_magnifier, g_mag_rect)) {
        return -22;
    }
    if (!g_mag_buffer_valid || g_mag_buffer_size < (SIZE_T)(g_mag_w * g_mag_h * 4)) {
        return -23;
    }
    // Swap R<->B (RGBA → BGRA) because our encoder expects BGRA.
    SIZE_T px = g_mag_buffer_size / 4;
    unsigned char *src = g_mag_buffer;
    for (SIZE_T i = 0; i < px; i++) {
        out_bgra[i*4+0] = src[i*4+2]; // B
        out_bgra[i*4+1] = src[i*4+1]; // G
        out_bgra[i*4+2] = src[i*4+0]; // R
        out_bgra[i*4+3] = src[i*4+3]; // A
    }
    return 0;
}

static void mag_close(void) {
    if (g_mag_magnifier) { DestroyWindow(g_mag_magnifier); g_mag_magnifier = NULL; }
    if (g_mag_host)      { DestroyWindow(g_mag_host);      g_mag_host = NULL; }
    if (g_mag_uninit)    { g_mag_uninit(); }
    if (g_mag_lib)       { FreeLibrary(g_mag_lib); g_mag_lib = NULL; }
    if (g_mag_buffer)    { free(g_mag_buffer); g_mag_buffer = NULL; g_mag_buffer_cap = 0; }
    g_mag_w = g_mag_h = 0;
    g_mag_buffer_valid = 0;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

var magActive bool

func magCaptureInit() error {
	rc := int(C.mag_init())
	if rc < 0 {
		return fmt.Errorf("magnification init failed (code %d)", rc)
	}
	magActive = true
	return nil
}

func magCaptureWidth() int  { return int(C.mag_width()) }
func magCaptureHeight() int { return int(C.mag_height()) }

func magCaptureFrame(buf []byte) error {
	if !magActive {
		return fmt.Errorf("magnification not initialised")
	}
	w := magCaptureWidth()
	h := magCaptureHeight()
	if len(buf) < w*h*4 {
		return fmt.Errorf("magnification: buffer too small %d < %d", len(buf), w*h*4)
	}
	rc := int(C.mag_capture_frame((*C.uchar)(unsafe.Pointer(&buf[0]))))
	if rc < 0 {
		return fmt.Errorf("magnification capture failed (code %d)", rc)
	}
	return nil
}

func magCaptureClose() {
	if magActive {
		C.mag_close()
		magActive = false
	}
}
