//go:build windows

package main

/*
#cgo CFLAGS: -DCOBJMACROS -DINITGUID
#cgo LDFLAGS: -ld3d11 -ldxgi -lgdi32

#include <windows.h>
#include <d3d11.h>
#include <dxgi1_2.h>
#include <stdlib.h>
#include <string.h>

// ── DXGI globals ──────────────────────────────────────────────────────────────
static ID3D11Device           *g_device  = NULL;
static ID3D11DeviceContext    *g_ctx     = NULL;
static IDXGIOutputDuplication *g_dup     = NULL;
static ID3D11Texture2D        *g_staging = NULL;

// ── GDI globals (fallback for RDP / remote sessions) ─────────────────────────
static int     g_use_gdi   = 0;
static HDC     g_hdcScreen = NULL;
static HDC     g_hdcMem    = NULL;
static HBITMAP g_hBitmap   = NULL;
static HBITMAP g_hOldBmp   = NULL;

// ── Common ────────────────────────────────────────────────────────────────────
static int g_width  = 0;
static int g_height = 0;
static int g_mon_x  = 0; // monitor origin (for multi-monitor coordinate mapping)
static int g_mon_y  = 0;
static int g_target_monitor = 0; // which monitor to capture

// ── Monitor enumeration ──────────────────────────────────────────────────────

#define OR_MAX_MONITORS 16

typedef struct {
    int index;
    wchar_t name[32];
    int x, y, w, h;
} MonitorInfoC;

static int enumerate_monitors(MonitorInfoC *out, int maxCount) {
    IDXGIFactory1 *factory = NULL;
    HRESULT hr;
    int count = 0;
    UINT ai, oi;
    IDXGIAdapter *adapter = NULL;
    IDXGIOutput *output = NULL;

    // CreateDXGIFactory1 is in dxgi.dll (already linked via -ldxgi)
    hr = CreateDXGIFactory1(&IID_IDXGIFactory1, (void**)&factory);
    if (FAILED(hr)) return 0;

    for (ai = 0; IDXGIFactory1_EnumAdapters(factory, ai, &adapter) == S_OK; ai++) {
        for (oi = 0; IDXGIAdapter_EnumOutputs(adapter, oi, &output) == S_OK; oi++) {
            if (count >= maxCount) { IUnknown_Release(output); break; }
            DXGI_OUTPUT_DESC desc;
            IDXGIOutput_GetDesc(output, &desc);
            out[count].index = count;
            memcpy(out[count].name, desc.DeviceName, sizeof(desc.DeviceName));
            out[count].x = desc.DesktopCoordinates.left;
            out[count].y = desc.DesktopCoordinates.top;
            out[count].w = desc.DesktopCoordinates.right - desc.DesktopCoordinates.left;
            out[count].h = desc.DesktopCoordinates.bottom - desc.DesktopCoordinates.top;
            count++;
            IUnknown_Release(output);
        }
        IUnknown_Release(adapter);
    }
    IUnknown_Release(factory);
    return count;
}

static void capture_get_monitor_offset(int *ox, int *oy) {
    *ox = g_mon_x;
    *oy = g_mon_y;
}

// ── Cursor overlay ───────────────────────────────────────────────────────────

static void overlay_cursor_gdi(unsigned char *bgra, int w, int h, int mon_x, int mon_y) {
    CURSORINFO ci;
    ci.cbSize = sizeof(ci);
    if (!GetCursorInfo(&ci)) return;
    if (!(ci.flags & CURSOR_SHOWING)) return;

    int cx = ci.ptScreenPos.x - mon_x;
    int cy = ci.ptScreenPos.y - mon_y;
    if (cx < -32 || cx > w + 32 || cy < -32 || cy > h + 32) return;

    ICONINFO ii;
    if (!GetIconInfo(ci.hCursor, &ii)) return;

    int hotX = (int)ii.xHotspot;
    int hotY = (int)ii.yHotspot;
    int drawX = cx - hotX;
    int drawY = cy - hotY;

    // Draw cursor onto a temporary DC, then alpha-blend into the BGRA buffer
    HDC hdcScreen = GetDC(NULL);
    HDC hdcMem = CreateCompatibleDC(hdcScreen);
    HBITMAP hbm = CreateCompatibleBitmap(hdcScreen, w, h);
    HBITMAP old = (HBITMAP)SelectObject(hdcMem, hbm);

    // Copy existing frame into the DC
    BITMAPINFO bmi;
    ZeroMemory(&bmi, sizeof(bmi));
    bmi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    bmi.bmiHeader.biWidth = w;
    bmi.bmiHeader.biHeight = -h;
    bmi.bmiHeader.biPlanes = 1;
    bmi.bmiHeader.biBitCount = 32;
    bmi.bmiHeader.biCompression = BI_RGB;
    SetDIBitsToDevice(hdcMem, 0, 0, w, h, 0, 0, 0, h, bgra, &bmi, DIB_RGB_COLORS);

    // Draw the cursor
    DrawIconEx(hdcMem, drawX, drawY, ci.hCursor, 0, 0, 0, NULL, DI_NORMAL);

    // Read back
    GetDIBits(hdcMem, hbm, 0, h, bgra, &bmi, DIB_RGB_COLORS);

    SelectObject(hdcMem, old);
    DeleteObject(hbm);
    DeleteDC(hdcMem);
    ReleaseDC(NULL, hdcScreen);

    if (ii.hbmMask) DeleteObject(ii.hbmMask);
    if (ii.hbmColor) DeleteObject(ii.hbmColor);
}

// ── DXGI helpers ──────────────────────────────────────────────────────────────

static void dxgi_close(void) {
    if (g_staging) { IUnknown_Release(g_staging); g_staging = NULL; }
    if (g_dup)     { IUnknown_Release(g_dup);     g_dup     = NULL; }
    if (g_ctx)     { IUnknown_Release(g_ctx);     g_ctx     = NULL; }
    if (g_device)  { IUnknown_Release(g_device);  g_device  = NULL; }
}

// Returns 0 on success, negative on any failure (resources cleaned up).
// monitor_idx: which output to capture (0 = primary, enumerate order).
static int dxgi_init(int monitor_idx) {
    D3D_FEATURE_LEVEL fl;
    HRESULT hr = D3D11CreateDevice(
        NULL, D3D_DRIVER_TYPE_HARDWARE, NULL,
        0, NULL, 0, D3D11_SDK_VERSION,
        &g_device, &fl, &g_ctx
    );
    if (FAILED(hr)) {
        hr = D3D11CreateDevice(
            NULL, D3D_DRIVER_TYPE_WARP, NULL,
            0, NULL, 0, D3D11_SDK_VERSION,
            &g_device, &fl, &g_ctx
        );
        if (FAILED(hr)) return -1;
    }

    // Find the target output by walking all adapters/outputs
    IDXGIDevice *dxgiDev = NULL;
    hr = ID3D11Device_QueryInterface(g_device, &IID_IDXGIDevice, (void**)&dxgiDev);
    if (FAILED(hr)) { dxgi_close(); return -2; }

    IDXGIAdapter *adapter = NULL;
    hr = IDXGIDevice_GetAdapter(dxgiDev, &adapter);
    IUnknown_Release(dxgiDev);
    if (FAILED(hr)) { dxgi_close(); return -3; }

    // Walk outputs to find monitor_idx
    IDXGIOutput *output = NULL;
    IDXGIFactory1 *factory = NULL;
    IDXGIAdapter_GetParent(adapter, &IID_IDXGIFactory1, (void**)&factory);
    IUnknown_Release(adapter);
    if (!factory) { dxgi_close(); return -3; }

    int cur = 0;
    int found = 0;
    UINT ai, oi;
    IDXGIAdapter *adp = NULL;
    for (ai = 0; !found && IDXGIFactory1_EnumAdapters(factory, ai, &adp) == S_OK; ai++) {
        IDXGIOutput *out = NULL;
        for (oi = 0; IDXGIAdapter_EnumOutputs(adp, oi, &out) == S_OK; oi++) {
            if (cur == monitor_idx) {
                output = out;
                found = 1;
                break;
            }
            cur++;
            IUnknown_Release(out);
        }
        IUnknown_Release(adp);
    }
    IUnknown_Release(factory);

    if (!output) { dxgi_close(); return -4; }

    DXGI_OUTPUT_DESC desc;
    IDXGIOutput_GetDesc(output, &desc);
    g_width  = desc.DesktopCoordinates.right  - desc.DesktopCoordinates.left;
    g_height = desc.DesktopCoordinates.bottom - desc.DesktopCoordinates.top;
    g_mon_x  = desc.DesktopCoordinates.left;
    g_mon_y  = desc.DesktopCoordinates.top;

    IDXGIOutput1 *output1 = NULL;
    hr = IDXGIOutput_QueryInterface(output, &IID_IDXGIOutput1, (void**)&output1);
    IUnknown_Release(output);
    if (FAILED(hr)) { dxgi_close(); return -5; }

    hr = IDXGIOutput1_DuplicateOutput(output1, (IUnknown*)g_device, &g_dup);
    IUnknown_Release(output1);
    if (FAILED(hr)) { dxgi_close(); g_width = 0; g_height = 0; return -6; }

    return 0;
}

// ── GDI helpers ───────────────────────────────────────────────────────────────

static void gdi_close(void) {
    if (g_hOldBmp && g_hdcMem) SelectObject(g_hdcMem, g_hOldBmp);
    if (g_hBitmap)   { DeleteObject(g_hBitmap);        g_hBitmap   = NULL; }
    if (g_hdcMem)    { DeleteDC(g_hdcMem);             g_hdcMem    = NULL; }
    if (g_hdcScreen) { ReleaseDC(NULL, g_hdcScreen);   g_hdcScreen = NULL; }
    g_hOldBmp = NULL;
    g_use_gdi = 0;
}

// gdi_init: rx/ry/rw/rh = region to capture. If rw <= 0, captures virtual screen.
static int gdi_init(int rx, int ry, int rw, int rh) {
    g_hdcScreen = GetDC(NULL);
    if (!g_hdcScreen) return -10;

    if (rw > 0 && rh > 0) {
        g_width  = rw;
        g_height = rh;
        g_mon_x  = rx;
        g_mon_y  = ry;
    } else {
        g_width  = GetSystemMetrics(SM_CXVIRTUALSCREEN);
        g_height = GetSystemMetrics(SM_CYVIRTUALSCREEN);
        g_mon_x  = GetSystemMetrics(SM_XVIRTUALSCREEN);
        g_mon_y  = GetSystemMetrics(SM_YVIRTUALSCREEN);
    }
    if (g_width <= 0 || g_height <= 0) {
        ReleaseDC(NULL, g_hdcScreen);
        g_hdcScreen = NULL;
        return -11;
    }

    g_hdcMem = CreateCompatibleDC(g_hdcScreen);
    if (!g_hdcMem) { gdi_close(); return -12; }

    g_hBitmap = CreateCompatibleBitmap(g_hdcScreen, g_width, g_height);
    if (!g_hBitmap) { gdi_close(); return -13; }

    g_hOldBmp = (HBITMAP)SelectObject(g_hdcMem, g_hBitmap);
    g_use_gdi = 1;
    return 0;
}

// ── Public capture API ────────────────────────────────────────────────────────

// capture_init_monitor: tries DXGI first; falls back to GDI.
static int capture_init_monitor(int monitor_idx) {
    g_target_monitor = monitor_idx;

    if (dxgi_init(monitor_idx) == 0) return 0;

    // DXGI unavailable — use GDI. Find monitor region via enumeration.
    MonitorInfoC mons[OR_MAX_MONITORS];
    int n = enumerate_monitors(mons, OR_MAX_MONITORS);
    if (monitor_idx >= 0 && monitor_idx < n) {
        return gdi_init(mons[monitor_idx].x, mons[monitor_idx].y,
                        mons[monitor_idx].w, mons[monitor_idx].h);
    }
    return gdi_init(0, 0, 0, 0); // fallback: entire virtual screen
}

// capture_init: backward compat — captures primary monitor.
static int capture_init(void) {
    return capture_init_monitor(0);
}

static void capture_get_size(int *w, int *h) {
    *w = g_width;
    *h = g_height;
}

// capture_frame: fills out_bgra (pre-allocated w*h*4 bytes) with BGRA pixel data.
// Returns:  0 = frame captured,  1 = no new frame (DXGI timeout),  -1 = fatal error.
// GDI path always returns 0 (no "wait for new frame" concept).
static int capture_frame(unsigned char *out_bgra) {
    if (g_use_gdi) {
        // GDI BitBlt — works in RDP sessions (CPU capture, no hardware acceleration)
        if (!BitBlt(g_hdcMem, 0, 0, g_width, g_height, g_hdcScreen, g_mon_x, g_mon_y, SRCCOPY | CAPTUREBLT))
            return -1;

        BITMAPINFO bmi;
        ZeroMemory(&bmi, sizeof(bmi));
        bmi.bmiHeader.biSize        = sizeof(BITMAPINFOHEADER);
        bmi.bmiHeader.biWidth       = g_width;
        bmi.bmiHeader.biHeight      = -g_height; // negative = top-down (matches DXGI convention)
        bmi.bmiHeader.biPlanes      = 1;
        bmi.bmiHeader.biBitCount    = 32;        // BGRX — alpha ignored by encoder
        bmi.bmiHeader.biCompression = BI_RGB;

        int lines = GetDIBits(g_hdcMem, g_hBitmap, 0, (UINT)g_height, out_bgra, &bmi, DIB_RGB_COLORS);
        if (lines <= 0) return -1;
        overlay_cursor_gdi(out_bgra, g_width, g_height, g_mon_x, g_mon_y);
        return 0;
    }

    // ── DXGI path (original) ─────────────────────────────────────────────────
    DXGI_OUTDUPL_FRAME_INFO info;
    IDXGIResource *res = NULL;

    HRESULT hr = IDXGIOutputDuplication_AcquireNextFrame(g_dup, 33, &info, &res); // 33ms timeout
    if (hr == DXGI_ERROR_WAIT_TIMEOUT) return 1;
    if (hr == DXGI_ERROR_ACCESS_LOST) return -1;
    if (FAILED(hr)) return -1;

    ID3D11Texture2D *tex = NULL;
    hr = IDXGIResource_QueryInterface(res, &IID_ID3D11Texture2D, (void**)&tex);
    IUnknown_Release(res);
    if (FAILED(hr)) {
        IDXGIOutputDuplication_ReleaseFrame(g_dup);
        return -1;
    }

    if (!g_staging) {
        D3D11_TEXTURE2D_DESC td;
        ID3D11Texture2D_GetDesc(tex, &td);
        td.Usage          = D3D11_USAGE_STAGING;
        td.BindFlags      = 0;
        td.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
        td.MiscFlags      = 0;
        ID3D11Device_CreateTexture2D(g_device, &td, NULL, &g_staging);
    }

    ID3D11DeviceContext_CopyResource(g_ctx, (ID3D11Resource*)g_staging, (ID3D11Resource*)tex);
    IUnknown_Release(tex);
    IDXGIOutputDuplication_ReleaseFrame(g_dup);

    D3D11_MAPPED_SUBRESOURCE mapped;
    hr = ID3D11DeviceContext_Map(g_ctx, (ID3D11Resource*)g_staging, 0, D3D11_MAP_READ, 0, &mapped);
    if (FAILED(hr)) return -1;

    int row_bytes = g_width * 4;
    unsigned char *src = (unsigned char*)mapped.pData;
    for (int y = 0; y < g_height; y++) {
        memcpy(out_bgra + y * row_bytes, src + y * mapped.RowPitch, row_bytes);
    }

    ID3D11DeviceContext_Unmap(g_ctx, (ID3D11Resource*)g_staging, 0);

    // Overlay the mouse cursor onto the captured frame
    overlay_cursor_gdi(out_bgra, g_width, g_height, g_mon_x, g_mon_y);

    return 0;
}

// capture_close: releases all resources (DXGI or GDI).
static void capture_close(void) {
    if (g_use_gdi) {
        gdi_close();
    } else {
        dxgi_close();
    }
    g_width = 0;
    g_height = 0;
}
static int capture_is_gdi(void) { return g_use_gdi; }
*/
import "C"
import (
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

// MonitorInfo describes a connected display for the viewer's monitor selector.
type MonitorInfo struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func enumerateMonitors() []MonitorInfo {
	var cmons [16]C.MonitorInfoC
	n := int(C.enumerate_monitors(&cmons[0], C.int(16)))
	out := make([]MonitorInfo, n)
	for i := 0; i < n; i++ {
		out[i] = MonitorInfo{
			Index:  int(cmons[i].index),
			Name:   syscall.UTF16ToString((*[32]uint16)(unsafe.Pointer(&cmons[i].name[0]))[:]),
			X:      int(cmons[i].x),
			Y:      int(cmons[i].y),
			Width:  int(cmons[i].w),
			Height: int(cmons[i].h),
		}
	}
	return out
}

func captureInitMonitor(idx int) error {
	ret := int(C.capture_init_monitor(C.int(idx)))
	if ret < 0 {
		return fmt.Errorf("capture init monitor %d failed (code %d)", idx, ret)
	}
	captureActive = true
	if C.capture_is_gdi() != 0 {
		log.Printf("capture: monitor %d via GDI", idx)
	} else {
		log.Printf("capture: monitor %d via DXGI", idx)
	}
	return nil
}

func captureMonitorOffset() (x, y int) {
	var cx, cy C.int
	C.capture_get_monitor_offset(&cx, &cy)
	return int(cx), int(cy)
}

var captureActive bool

func captureInit() error {
	ret := int(C.capture_init())
	if ret < 0 {
		return fmt.Errorf("screen capture init failed (code %d)", ret)
	}
	captureActive = true
	if C.capture_is_gdi() != 0 {
		log.Printf("helper: capture path = GDI (DXGI unavailable in this session)")
	} else {
		log.Printf("helper: capture path = DXGI Desktop Duplication")
	}
	return nil
}

func captureClose() {
	if captureActive {
		C.capture_close()
		captureActive = false
	}
}

func captureWidth() int {
	var w, h C.int
	C.capture_get_size(&w, &h)
	return int(w)
}

func captureHeight() int {
	var w, h C.int
	C.capture_get_size(&w, &h)
	return int(h)
}

// captureFrame fills buf (must be width*height*4 bytes) with BGRA pixel data.
// Returns actual (width, height) and any error.
// err == nil and width>0 means a new frame was captured.
func captureFrame(buf []byte) (width, height int, err error) {
	var w, h C.int
	C.capture_get_size(&w, &h)
	width = int(w)
	height = int(h)

	expected := width * height * 4
	if len(buf) < expected {
		return 0, 0, fmt.Errorf("captureFrame: buffer too small (%d < %d)", len(buf), expected)
	}

	ret := int(C.capture_frame((*C.uchar)(unsafe.Pointer(&buf[0]))))
	if ret == 1 {
		// DXGI timeout — no new frame since last call
		return width, height, fmt.Errorf("no new frame")
	}
	if ret < 0 {
		// Fatal — attempt reinitialise (DXGI access lost, or GDI error)
		C.capture_close()
		if rc := int(C.capture_init()); rc < 0 {
			return 0, 0, fmt.Errorf("capture reinit failed (code %d)", rc)
		}
		return width, height, fmt.Errorf("no new frame")
	}
	return width, height, nil
}
