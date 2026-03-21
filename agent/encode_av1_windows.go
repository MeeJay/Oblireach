//go:build windows

package main

/*
#include <windows.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdio.h>

// ── Minimal SVT-AV1 type definitions for dynamic loading ────────────────────

typedef int32_t EbErrorType;
#define EB_ErrorNone 0

typedef struct { uint32_t size; void *p_component_private; void *p_application_private; } EbComponentType;

typedef struct {
    uint32_t size;
    uint8_t *p_buffer;
    uint32_t n_filled_len;
    uint32_t n_alloc_len;
    void *p_app_private;
    void *wrapper_ptr;
    uint32_t n_tick_count;
    int64_t dts;
    int64_t pts;
    uint32_t qp;
    uint32_t pic_type;
    uint64_t luma_sse, cr_sse, cb_sse;
    uint32_t flags;
    double luma_ssim, cr_ssim, cb_ssim;
    void *metadata;
} EbBufferHeaderType;

typedef struct {
    uint8_t *luma;
    uint8_t *cb;
    uint8_t *cr;
    uint32_t y_stride;
    uint32_t cr_stride;
    uint32_t cb_stride;
    uint32_t width;
    uint32_t height;
    uint32_t org_x;
    uint32_t org_y;
    uint32_t color_fmt;
    uint32_t bit_depth;
} EbSvtIOFormat;

// EbSvtAv1EncConfiguration is huge (~4KB). We use parse_parameter instead.
typedef char EbSvtAv1EncConfiguration[4096];

// ── Function pointers ───────────────────────────────────────────────────────

typedef EbErrorType (*PFN_svt_av1_enc_init_handle)(EbComponentType**, void*, EbSvtAv1EncConfiguration*);
typedef EbErrorType (*PFN_svt_av1_enc_set_parameter)(EbComponentType*, EbSvtAv1EncConfiguration*);
typedef EbErrorType (*PFN_svt_av1_enc_parse_parameter)(EbSvtAv1EncConfiguration*, const char*, const char*);
typedef EbErrorType (*PFN_svt_av1_enc_init)(EbComponentType*);
typedef EbErrorType (*PFN_svt_av1_enc_send_picture)(EbComponentType*, EbBufferHeaderType*);
typedef EbErrorType (*PFN_svt_av1_enc_get_packet)(EbComponentType*, EbBufferHeaderType**, unsigned char);
typedef void        (*PFN_svt_av1_enc_release_out_buffer)(EbBufferHeaderType**);
typedef EbErrorType (*PFN_svt_av1_enc_deinit)(EbComponentType*);
typedef EbErrorType (*PFN_svt_av1_enc_deinit_handle)(EbComponentType*);

// ── Global state ────────────────────────────────────────────────────────────

static HMODULE g_av1_dll = NULL;
static EbComponentType *g_av1_enc = NULL;
static EbSvtAv1EncConfiguration g_av1_cfg;
static EbBufferHeaderType g_av1_in_buf;
static EbSvtIOFormat g_av1_io;
static int g_av1_init = 0;
static int g_av1_w = 0, g_av1_h = 0;
static int64_t g_av1_pts = 0;

static PFN_svt_av1_enc_init_handle      pfn_av1_init_handle = NULL;
static PFN_svt_av1_enc_set_parameter    pfn_av1_set_param = NULL;
static PFN_svt_av1_enc_parse_parameter  pfn_av1_parse_param = NULL;
static PFN_svt_av1_enc_init             pfn_av1_init = NULL;
static PFN_svt_av1_enc_send_picture     pfn_av1_send = NULL;
static PFN_svt_av1_enc_get_packet       pfn_av1_get_pkt = NULL;
static PFN_svt_av1_enc_release_out_buffer pfn_av1_release = NULL;
static PFN_svt_av1_enc_deinit           pfn_av1_deinit = NULL;
static PFN_svt_av1_enc_deinit_handle    pfn_av1_deinit_handle = NULL;

static int av1_load_dll(void) {
    if (g_av1_dll) return 0;
    const char *names[] = {"libSvtAv1Enc-2.dll", "SvtAv1Enc.dll", "libSvtAv1Enc.dll", NULL};
    int i;
    for (i = 0; names[i]; i++) {
        g_av1_dll = LoadLibraryA(names[i]);
        if (g_av1_dll) break;
    }
    if (!g_av1_dll) {
        char p[MAX_PATH]; GetModuleFileNameA(NULL, p, MAX_PATH);
        char *s = strrchr(p, '\\');
        if (s) for (i = 0; names[i]; i++) {
            strcpy(s+1, names[i]);
            g_av1_dll = LoadLibraryA(p);
            if (g_av1_dll) break;
        }
    }
    if (!g_av1_dll) return -1;

    pfn_av1_init_handle  = (PFN_svt_av1_enc_init_handle)GetProcAddress(g_av1_dll, "svt_av1_enc_init_handle");
    pfn_av1_set_param    = (PFN_svt_av1_enc_set_parameter)GetProcAddress(g_av1_dll, "svt_av1_enc_set_parameter");
    pfn_av1_parse_param  = (PFN_svt_av1_enc_parse_parameter)GetProcAddress(g_av1_dll, "svt_av1_enc_parse_parameter");
    pfn_av1_init         = (PFN_svt_av1_enc_init)GetProcAddress(g_av1_dll, "svt_av1_enc_init");
    pfn_av1_send         = (PFN_svt_av1_enc_send_picture)GetProcAddress(g_av1_dll, "svt_av1_enc_send_picture");
    pfn_av1_get_pkt      = (PFN_svt_av1_enc_get_packet)GetProcAddress(g_av1_dll, "svt_av1_enc_get_packet");
    pfn_av1_release      = (PFN_svt_av1_enc_release_out_buffer)GetProcAddress(g_av1_dll, "svt_av1_enc_release_out_buffer");
    pfn_av1_deinit       = (PFN_svt_av1_enc_deinit)GetProcAddress(g_av1_dll, "svt_av1_enc_deinit");
    pfn_av1_deinit_handle= (PFN_svt_av1_enc_deinit_handle)GetProcAddress(g_av1_dll, "svt_av1_enc_deinit_handle");

    if (!pfn_av1_init_handle || !pfn_av1_set_param || !pfn_av1_init ||
        !pfn_av1_send || !pfn_av1_get_pkt || !pfn_av1_release ||
        !pfn_av1_deinit || !pfn_av1_deinit_handle) {
        FreeLibrary(g_av1_dll); g_av1_dll = NULL; return -2;
    }
    return 0;
}

static int av1_available(void) { return av1_load_dll() == 0 ? 1 : 0; }

static int av1_encoder_init(int w, int h, int fps, int bitrate_kbps) {
    if (g_av1_init) return 0;
    if (av1_load_dll() != 0) return -1;

    memset(&g_av1_cfg, 0, sizeof(g_av1_cfg));
    EbErrorType err = pfn_av1_init_handle(&g_av1_enc, NULL, (EbSvtAv1EncConfiguration*)&g_av1_cfg);
    if (err != EB_ErrorNone) return -2;

    // Configure via parse_parameter for safety
    if (pfn_av1_parse_param) {
        char buf[64];
        sprintf(buf, "%d", w); pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "source-width", buf);
        sprintf(buf, "%d", h); pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "source-height", buf);
        sprintf(buf, "%d", fps); pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "fps-num", buf);
        pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "fps-den", "1");
        sprintf(buf, "%d", bitrate_kbps * 1000); pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "tbr", buf);
        pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "preset", "12");  // fastest
        pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "rc", "1");       // VBR
        pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "lp", "2");       // 2 threads
        sprintf(buf, "%d", fps); pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "keyint", buf);
        pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "lookahead", "0");
        pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "enable-overlays", "0");
        pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "scd", "0");
        pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "tile-rows", "0");
        pfn_av1_parse_param((EbSvtAv1EncConfiguration*)&g_av1_cfg, "tile-columns", "1");
    }

    err = pfn_av1_set_param(g_av1_enc, (EbSvtAv1EncConfiguration*)&g_av1_cfg);
    if (err != EB_ErrorNone) { pfn_av1_deinit_handle(g_av1_enc); g_av1_enc = NULL; return -3; }

    err = pfn_av1_init(g_av1_enc);
    if (err != EB_ErrorNone) { pfn_av1_deinit_handle(g_av1_enc); g_av1_enc = NULL; return -4; }

    g_av1_w = w; g_av1_h = h; g_av1_pts = 0; g_av1_init = 1;
    return 0;
}

static int av1_encode_frame(const unsigned char *i420, int w, int h,
                             unsigned char *out, int out_cap) {
    if (!g_av1_init || !g_av1_enc) return -1;

    int y_sz = w * h, uv_sz = (w/2) * (h/2);

    memset(&g_av1_io, 0, sizeof(g_av1_io));
    g_av1_io.luma   = (uint8_t*)i420;
    g_av1_io.cb     = (uint8_t*)(i420 + y_sz);
    g_av1_io.cr     = (uint8_t*)(i420 + y_sz + uv_sz);
    g_av1_io.y_stride  = w;
    g_av1_io.cb_stride = w / 2;
    g_av1_io.cr_stride = w / 2;
    g_av1_io.width  = w;
    g_av1_io.height = h;
    g_av1_io.color_fmt = 1; // EB_YUV420
    g_av1_io.bit_depth = 0; // EB_EIGHT_BIT

    memset(&g_av1_in_buf, 0, sizeof(g_av1_in_buf));
    g_av1_in_buf.size = sizeof(EbBufferHeaderType);
    g_av1_in_buf.p_buffer = (uint8_t*)&g_av1_io;
    g_av1_in_buf.n_filled_len = y_sz + 2 * uv_sz;
    g_av1_in_buf.pts = g_av1_pts++;
    g_av1_in_buf.pic_type = 0; // EB_AV1_INVALID_PICTURE (auto)

    EbErrorType err = pfn_av1_send(g_av1_enc, &g_av1_in_buf);
    if (err != EB_ErrorNone) return -2;

    // Get output (non-blocking)
    EbBufferHeaderType *out_pkt = NULL;
    err = pfn_av1_get_pkt(g_av1_enc, &out_pkt, 0);
    if (err != EB_ErrorNone || !out_pkt || out_pkt->n_filled_len == 0) return 0;

    int sz = (int)out_pkt->n_filled_len;
    if (sz > out_cap) sz = out_cap;
    memcpy(out, out_pkt->p_buffer, sz);
    pfn_av1_release(&out_pkt);
    return sz;
}

static void av1_encoder_close(void) {
    if (g_av1_init && g_av1_enc) {
        // Send EOS
        EbBufferHeaderType eos;
        memset(&eos, 0, sizeof(eos));
        eos.size = sizeof(EbBufferHeaderType);
        eos.flags = 0x00000001; // EB_BUFFERFLAG_EOS
        pfn_av1_send(g_av1_enc, &eos);

        pfn_av1_deinit(g_av1_enc);
        pfn_av1_deinit_handle(g_av1_enc);
        g_av1_enc = NULL;
        g_av1_init = 0;
    }
}
*/
import "C"
import (
	"fmt"
	"log"
	"unsafe"
)

const av1MaxBuf = 4 * 1024 * 1024

var (
	av1InitDone bool
	av1NalBuf   = make([]byte, av1MaxBuf)
	av1I420Buf  []byte
)

func av1Available() bool  { return int(C.av1_available()) == 1 }

func av1EncoderInit(width, height, fps, bitrateKbps int) error {
	rc := int(C.av1_encoder_init(C.int(width), C.int(height), C.int(fps), C.int(bitrateKbps)))
	if rc != 0 {
		return fmt.Errorf("SVT-AV1 init failed (code %d)", rc)
	}
	av1InitDone = true
	av1I420Buf = make([]byte, width*height+2*(width/2)*(height/2))
	log.Printf("AV1 encoder initialized: %dx%d@%dfps %dkbps", width, height, fps, bitrateKbps)
	return nil
}

func av1EncodeFrame(bgra []byte, width, height int) ([]byte, error) {
	if !av1InitDone { return nil, fmt.Errorf("AV1 not initialized") }
	bgraToI420(bgra, width, height, av1I420Buf)
	n := int(C.av1_encode_frame(
		(*C.uchar)(unsafe.Pointer(&av1I420Buf[0])),
		C.int(width), C.int(height),
		(*C.uchar)(unsafe.Pointer(&av1NalBuf[0])), C.int(av1MaxBuf)))
	if n < 0 { return nil, fmt.Errorf("AV1 encode failed (code %d)", n) }
	if n == 0 { return nil, nil }
	out := make([]byte, n)
	copy(out, av1NalBuf[:n])
	return out, nil
}

func av1EncoderClose() {
	if av1InitDone { C.av1_encoder_close(); av1InitDone = false }
}
