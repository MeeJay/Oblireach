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

// g_dxgi_last_fail: exit code of the most recent dxgi_init failure, for diag.
// g_dxgi_last_hr: raw HRESULT from DuplicateOutput when it was the failing step.
static int g_dxgi_last_fail = 0;
static unsigned int g_dxgi_last_hr = 0;
static int dxgi_last_fail(void) { return g_dxgi_last_fail; }
static unsigned int dxgi_last_hr(void) { return g_dxgi_last_hr; }

// Returns 0 on success, negative on any failure (resources cleaned up).
// monitor_idx: which output to capture (0 = primary, enumerate order).
//
// IMPORTANT: DXGI requires the D3D11 device to be created on the SAME
// adapter that owns the target output. Using a device from a different
// adapter makes DuplicateOutput return E_INVALIDARG (0x80070057). That
// matters for systems with multiple adapters — e.g. a Hyper-V VM where
// the primary is "Microsoft Hyper-V Video" and the Virtual Display
// Driver is a separate IDD adapter: we must enumerate first, find the
// adapter that owns monitor_idx, then create D3D11 on it.
static int dxgi_init(int monitor_idx) {
    IDXGIFactory1 *factory = NULL;
    HRESULT hr = CreateDXGIFactory1(&IID_IDXGIFactory1, (void**)&factory);
    if (FAILED(hr)) { g_dxgi_last_fail = -1; return -1; }

    // First pass: locate the adapter that owns monitor_idx.
    IDXGIAdapter *targetAdapter = NULL;
    IDXGIOutput *targetOutput = NULL;
    int cur = 0;
    UINT ai, oi;
    IDXGIAdapter *adp = NULL;
    for (ai = 0; !targetOutput && IDXGIFactory1_EnumAdapters(factory, ai, &adp) == S_OK; ai++) {
        IDXGIOutput *out = NULL;
        for (oi = 0; IDXGIAdapter_EnumOutputs(adp, oi, &out) == S_OK; oi++) {
            if (cur == monitor_idx) {
                targetOutput = out;              // keep ref, released later
                targetAdapter = adp;             // keep ref (will be released after device creation)
                IDXGIAdapter_AddRef(targetAdapter);
                break;
            }
            cur++;
            IUnknown_Release(out);
        }
        IUnknown_Release(adp);
    }
    IUnknown_Release(factory);

    if (!targetOutput || !targetAdapter) {
        g_dxgi_last_fail = -4;
        if (targetOutput) IUnknown_Release(targetOutput);
        if (targetAdapter) IUnknown_Release(targetAdapter);
        return -4;
    }

    // Create D3D11 device on the adapter that owns this output. With an
    // explicit adapter the driver type MUST be D3D_DRIVER_TYPE_UNKNOWN.
    D3D_FEATURE_LEVEL fl;
    hr = D3D11CreateDevice(
        (IDXGIAdapter*)targetAdapter, D3D_DRIVER_TYPE_UNKNOWN, NULL,
        0, NULL, 0, D3D11_SDK_VERSION,
        &g_device, &fl, &g_ctx
    );
    IUnknown_Release(targetAdapter);
    if (FAILED(hr)) {
        IUnknown_Release(targetOutput);
        g_dxgi_last_fail = -1;
        g_dxgi_last_hr = (unsigned int)hr;
        return -1;
    }

    DXGI_OUTPUT_DESC desc;
    IDXGIOutput_GetDesc(targetOutput, &desc);
    g_width  = desc.DesktopCoordinates.right  - desc.DesktopCoordinates.left;
    g_height = desc.DesktopCoordinates.bottom - desc.DesktopCoordinates.top;
    g_mon_x  = desc.DesktopCoordinates.left;
    g_mon_y  = desc.DesktopCoordinates.top;

    IDXGIOutput1 *output1 = NULL;
    hr = IDXGIOutput_QueryInterface(targetOutput, &IID_IDXGIOutput1, (void**)&output1);
    IUnknown_Release(targetOutput);
    if (FAILED(hr)) { g_dxgi_last_fail = -5; dxgi_close(); return -5; }

    hr = IDXGIOutput1_DuplicateOutput(output1, (IUnknown*)g_device, &g_dup);
    IUnknown_Release(output1);
    if (FAILED(hr)) {
        // HRESULT encodes the reason: E_ACCESSDENIED = no rights (secure
        // desktop / cross-session), DXGI_ERROR_UNSUPPORTED = adapter does
        // not implement desktop duplication, E_INVALIDARG = device/output
        // adapter mismatch (should no longer happen after the fix above).
        g_dxgi_last_fail = -6;
        g_dxgi_last_hr = (unsigned int)hr;
        dxgi_close(); g_width = 0; g_height = 0; return -6;
    }

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

// capture_init_dxgi_only: attempts DXGI desktop duplication on monitor_idx
// without the GDI fallback. Used by capture_init_best to iterate monitors
// and pick the first one that actually supports duplication — the Virtual
// Display Driver does, while Hyper-V "basic" video and disconnected RDP
// outputs do not.
static int capture_init_dxgi_only(int monitor_idx) {
    g_target_monitor = monitor_idx;
    return dxgi_init(monitor_idx);
}

// capture_init_best: iterates every enumerated monitor and picks the first
// one on which DXGI Desktop Duplication succeeds. Falls back to GDI on
// monitor 0 if no DXGI output works at all (e.g. pure session-0 context).
// Returns 0 on success, negative on failure.
//
// g_diag_init records the per-output attempt results so the Go caller can
// surface the full picture to the log (adapter descriptions + HRESULTs).
#define OR_MAX_DIAG 8
typedef struct {
    wchar_t adapter[64];
    wchar_t device[32];
    int     w, h;
    int     step;       // where dxgi_init failed for this output (0 if ok)
    unsigned int hr;    // HRESULT of the last failing step
} DxgiDiagEntry;
static DxgiDiagEntry g_diag[OR_MAX_DIAG];
static int g_diag_count = 0;

// dxgi_diag_dump populates g_diag with one entry per output, trying each
// in isolation so we can see which adapter+output succeeded and why the
// others failed. Called by capture_init_best.
static void dxgi_diag_collect(void) {
    g_diag_count = 0;
    IDXGIFactory1 *factory = NULL;
    if (FAILED(CreateDXGIFactory1(&IID_IDXGIFactory1, (void**)&factory))) return;
    int idx = 0;
    IDXGIAdapter *adp = NULL;
    for (UINT ai = 0; IDXGIFactory1_EnumAdapters(factory, ai, &adp) == S_OK; ai++) {
        DXGI_ADAPTER_DESC adesc;
        IDXGIAdapter_GetDesc(adp, &adesc);
        IDXGIOutput *out = NULL;
        for (UINT oi = 0; IDXGIAdapter_EnumOutputs(adp, oi, &out) == S_OK; oi++) {
            if (idx < OR_MAX_DIAG) {
                DXGI_OUTPUT_DESC odesc;
                IDXGIOutput_GetDesc(out, &odesc);
                wcsncpy(g_diag[idx].adapter, adesc.Description, 63);
                wcsncpy(g_diag[idx].device, odesc.DeviceName, 31);
                g_diag[idx].w = odesc.DesktopCoordinates.right - odesc.DesktopCoordinates.left;
                g_diag[idx].h = odesc.DesktopCoordinates.bottom - odesc.DesktopCoordinates.top;
                g_diag[idx].step = 0;
                g_diag[idx].hr = 0;
                g_diag_count = idx + 1;
                // Try dxgi_init(idx) to capture failure code for this output.
                if (dxgi_init(idx) == 0) {
                    dxgi_close();
                } else {
                    g_diag[idx].step = g_dxgi_last_fail;
                    g_diag[idx].hr = g_dxgi_last_hr;
                }
            }
            idx++;
            IUnknown_Release(out);
        }
        IUnknown_Release(adp);
    }
    IUnknown_Release(factory);
}

static int diag_count(void) { return g_diag_count; }
static DxgiDiagEntry *diag_entry(int i) { return &g_diag[i]; }

static int capture_init_best(void) {
    MonitorInfoC mons[OR_MAX_MONITORS];
    int n = enumerate_monitors(mons, OR_MAX_MONITORS);
    for (int i = 0; i < n; i++) {
        if (dxgi_init(i) == 0) {
            g_target_monitor = i;
            return 0;
        }
    }
    // No DXGI output usable — last-resort GDI on the virtual screen.
    return gdi_init(0, 0, 0, 0);
}

// capture_reinit_current: re-initialises capture on the currently tracked
// monitor (g_target_monitor). Used by the recovery path after DXGI_ERROR_
// ACCESS_LOST (UAC prompt / workstation lock / login screen transition).
// Caller is expected to re-attach the thread to the active input desktop
// first (via inputSwitchActiveDesktop on the Go side) so that DXGI can
// duplicate whichever desktop is currently visible. If the previously
// chosen monitor no longer accepts duplication (the VDD disconnected,
// topology changed), we iterate all outputs again via capture_init_best.
static int capture_reinit_current(void) {
    int rc = capture_init_monitor(g_target_monitor);
    if (rc == 0) return 0;
    return capture_init_best();
}

static int capture_get_target_monitor(void) { return g_target_monitor; }

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
	// Up-front diagnostic dump: try each DXGI output independently and log
	// adapter + HRESULT for each. Makes it obvious which adapter succeeds
	// and which Windows is refusing (E_ACCESSDENIED, UNSUPPORTED, etc.).
	C.dxgi_diag_collect()
	n := int(C.diag_count())
	log.Printf("helper: DXGI dump — %d outputs visible to this process", n)
	for i := 0; i < n; i++ {
		e := C.diag_entry(C.int(i))
		adapter := syscall.UTF16ToString((*[64]uint16)(unsafe.Pointer(&e.adapter[0]))[:])
		device := syscall.UTF16ToString((*[32]uint16)(unsafe.Pointer(&e.device[0]))[:])
		if e.step == 0 {
			log.Printf("  [%d] %s / %s  %dx%d  DXGI ok", i, adapter, device, int(e.w), int(e.h))
		} else {
			log.Printf("  [%d] %s / %s  %dx%d  DXGI fail step=%d hr=0x%08x",
				i, adapter, device, int(e.w), int(e.h), int(e.step), uint32(e.hr))
		}
	}

	// Iterate every enumerated DXGI output and bind to the first one that
	// supports desktop duplication. On Hyper-V basic VMs the primary output
	// (Microsoft Basic Display / Hyper-V Video) does NOT support duplication
	// and returns E_ACCESSDENIED, while the bundled Virtual Display Driver
	// monitor does. Falling through the list surfaces whichever is usable.
	ret := int(C.capture_init_best())
	if ret < 0 {
		return fmt.Errorf("screen capture init failed (code %d)", ret)
	}
	captureActive = true
	mons := enumerateMonitors()
	picked := -1
	if ret == 0 {
		picked = int(C.capture_get_target_monitor())
	}
	if C.capture_is_gdi() != 0 {
		failStep := int(C.dxgi_last_fail())
		hr := uint32(C.dxgi_last_hr())
		log.Printf("helper: capture path = GDI (no DXGI-capable output among %d — last step=%d hr=0x%08x)",
			len(mons), failStep, hr)
	} else {
		name := ""
		if picked >= 0 && picked < len(mons) {
			name = fmt.Sprintf(" [%s %dx%d@%d,%d]",
				mons[picked].Name, mons[picked].Width, mons[picked].Height,
				mons[picked].X, mons[picked].Y)
		}
		log.Printf("helper: capture path = DXGI Desktop Duplication (monitor %d of %d)%s",
			picked, len(mons), name)
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
		// DXGI_ERROR_ACCESS_LOST happens when the input desktop changes
		// (UAC Secure Desktop, workstation lock, login screen). Re-attach
		// this thread to the active desktop before recreating DXGI so the
		// duplication targets the currently visible desktop.
		C.capture_close()
		inputSwitchActiveDesktop()
		if rc := int(C.capture_reinit_current()); rc < 0 {
			return 0, 0, fmt.Errorf("capture reinit failed (code %d)", rc)
		}
		return width, height, fmt.Errorf("no new frame")
	}
	return width, height, nil
}
