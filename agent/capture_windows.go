//go:build windows

package main

/*
#cgo CFLAGS: -DCOBJMACROS -DINITGUID
#cgo LDFLAGS: -ld3d11 -ldxgi

#include <windows.h>
#include <d3d11.h>
#include <dxgi1_2.h>
#include <stdlib.h>
#include <string.h>

static ID3D11Device          *g_device   = NULL;
static ID3D11DeviceContext   *g_ctx      = NULL;
static IDXGIOutputDuplication *g_dup     = NULL;
static ID3D11Texture2D       *g_staging  = NULL;
static int                    g_width    = 0;
static int                    g_height   = 0;

// capture_init: creates D3D11 device and acquires desktop duplication.
// Returns 0 on success, negative error code on failure.
static int capture_init(void) {
    D3D_FEATURE_LEVEL fl;
    HRESULT hr = D3D11CreateDevice(
        NULL, D3D_DRIVER_TYPE_HARDWARE, NULL,
        0, NULL, 0, D3D11_SDK_VERSION,
        &g_device, &fl, &g_ctx
    );
    if (FAILED(hr)) {
        // Fallback to WARP (software) renderer if no hardware
        hr = D3D11CreateDevice(
            NULL, D3D_DRIVER_TYPE_WARP, NULL,
            0, NULL, 0, D3D11_SDK_VERSION,
            &g_device, &fl, &g_ctx
        );
        if (FAILED(hr)) return -1;
    }

    IDXGIDevice *dxgiDev = NULL;
    hr = ID3D11Device_QueryInterface(g_device, &IID_IDXGIDevice, (void**)&dxgiDev);
    if (FAILED(hr)) return -2;

    IDXGIAdapter *adapter = NULL;
    hr = IDXGIDevice_GetAdapter(dxgiDev, &adapter);
    IUnknown_Release(dxgiDev);
    if (FAILED(hr)) return -3;

    IDXGIOutput *output = NULL;
    hr = IDXGIAdapter_EnumOutputs(adapter, 0, &output);
    IUnknown_Release(adapter);
    if (FAILED(hr)) return -4;

    // Get desktop bounds from output
    DXGI_OUTPUT_DESC desc;
    IDXGIOutput_GetDesc(output, &desc);
    g_width  = desc.DesktopCoordinates.right  - desc.DesktopCoordinates.left;
    g_height = desc.DesktopCoordinates.bottom - desc.DesktopCoordinates.top;

    IDXGIOutput1 *output1 = NULL;
    hr = IDXGIOutput_QueryInterface(output, &IID_IDXGIOutput1, (void**)&output1);
    IUnknown_Release(output);
    if (FAILED(hr)) return -5;

    hr = IDXGIOutput1_DuplicateOutput(output1, (IUnknown*)g_device, &g_dup);
    IUnknown_Release(output1);
    if (FAILED(hr)) return -6;

    return 0;
}

// capture_get_size: fills *w and *h with the capture resolution.
static void capture_get_size(int *w, int *h) {
    *w = g_width;
    *h = g_height;
}

// capture_frame: acquires the next desktop frame into out_bgra (pre-allocated: w*h*4 bytes).
// Returns:
//   0  = new frame captured
//   1  = timeout (no new frame since last call)
//  -1  = fatal error (caller should reinit)
static int capture_frame(unsigned char *out_bgra) {
    DXGI_OUTDUPL_FRAME_INFO info;
    IDXGIResource *res = NULL;

    HRESULT hr = IDXGIOutputDuplication_AcquireNextFrame(g_dup, 33, &info, &res); // 33ms timeout
    if (hr == DXGI_ERROR_WAIT_TIMEOUT) return 1;
    if (hr == DXGI_ERROR_ACCESS_LOST) {
        // Monitor config changed — reinitialise required
        return -1;
    }
    if (FAILED(hr)) return -1;

    ID3D11Texture2D *tex = NULL;
    hr = IDXGIResource_QueryInterface(res, &IID_ID3D11Texture2D, (void**)&tex);
    IUnknown_Release(res);
    if (FAILED(hr)) {
        IDXGIOutputDuplication_ReleaseFrame(g_dup);
        return -1;
    }

    // (Re)create the CPU-readable staging texture on first call or after size change
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

    // Copy rows — RowPitch may be padded beyond width*4
    int row_bytes = g_width * 4;
    unsigned char *src = (unsigned char*)mapped.pData;
    for (int y = 0; y < g_height; y++) {
        memcpy(out_bgra + y * row_bytes, src + y * mapped.RowPitch, row_bytes);
    }

    ID3D11DeviceContext_Unmap(g_ctx, (ID3D11Resource*)g_staging, 0);
    return 0;
}

// capture_close: releases all DXGI/D3D11 resources.
static void capture_close(void) {
    if (g_staging) { IUnknown_Release(g_staging); g_staging = NULL; }
    if (g_dup)     { IUnknown_Release(g_dup);     g_dup     = NULL; }
    if (g_ctx)     { IUnknown_Release(g_ctx);     g_ctx     = NULL; }
    if (g_device)  { IUnknown_Release(g_device);  g_device  = NULL; }
    g_width = 0; g_height = 0;
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

var captureActive bool

func captureInit() error {
	ret := int(C.capture_init())
	if ret < 0 {
		return fmt.Errorf("DXGI capture init failed (code %d)", ret)
	}
	captureActive = true
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
// err == nil and width>0 means new frame captured.
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
		// timeout — no new frame
		return width, height, fmt.Errorf("no new frame")
	}
	if ret < 0 {
		// Fatal — reinitialise DXGI
		C.capture_close()
		if rc := int(C.capture_init()); rc < 0 {
			return 0, 0, fmt.Errorf("DXGI reinit failed (code %d)", rc)
		}
		return width, height, fmt.Errorf("no new frame")
	}
	return width, height, nil
}
