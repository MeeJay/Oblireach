//go:build windows

package main

/*
#cgo CFLAGS: -DCOBJMACROS -DINITGUID
#cgo LDFLAGS: -lmfplat -lmf -lmfuuid -luuid -lole32 -lstrmiids

#include <windows.h>
#include <mfapi.h>
#include <mfidl.h>
#include <mftransform.h>
#include <mferror.h>
#include <codecapi.h>
#include <stdlib.h>
#include <string.h>

// Software H.264 encoder MFT CLSID
// {a7e2c842-2f83-4d5a-adbd-ca3855e2f824}
static const GUID CLSID_H264Encoder = {
    0xa7e2c842, 0x2f83, 0x4d5a,
    {0xad, 0xbd, 0xca, 0x38, 0x55, 0xe2, 0xf8, 0x24}
};

static IMFTransform *g_encoder   = NULL;
static int           g_enc_w     = 0;
static int           g_enc_h     = 0;
static int           g_fps       = 15;
static int           g_bitrate   = 3000000;

// BGRA to NV12 (BT.601 limited range, CPU software conversion)
static void bgra_to_nv12(
    const unsigned char *bgra, int w, int h,
    unsigned char *y_plane, unsigned char *uv_plane)
{
    int stride = w * 4;
    // Y plane
    for (int row = 0; row < h; row++) {
        const unsigned char *src = bgra + row * stride;
        unsigned char *ydst = y_plane + row * w;
        for (int col = 0; col < w; col++, src += 4) {
            int b = src[0], g = src[1], r = src[2];
            ydst[col] = (unsigned char)(((77*r + 150*g + 29*b + 128) >> 8) + 16);
        }
    }
    // UV plane (interleaved Cb,Cr — 2x downsampled)
    for (int row = 0; row < h/2; row++) {
        const unsigned char *s0 = bgra + (row*2)   * stride;
        const unsigned char *s1 = bgra + (row*2+1) * stride;
        unsigned char *uvdst = uv_plane + row * w;
        for (int col = 0; col < w/2; col++) {
            int b = ((int)s0[col*2*4+0] + s0[(col*2+1)*4+0] +
                          s1[col*2*4+0] + s1[(col*2+1)*4+0]) >> 2;
            int g = ((int)s0[col*2*4+1] + s0[(col*2+1)*4+1] +
                          s1[col*2*4+1] + s1[(col*2+1)*4+1]) >> 2;
            int r = ((int)s0[col*2*4+2] + s0[(col*2+1)*4+2] +
                          s1[col*2*4+2] + s1[(col*2+1)*4+2]) >> 2;
            uvdst[col*2]   = (unsigned char)(((-43*r -  85*g + 128*b + 128) >> 8) + 128);
            uvdst[col*2+1] = (unsigned char)(((128*r - 107*g -  21*b + 128) >> 8) + 128);
        }
    }
}

// encoder_init: initialises WMF H.264 MFT.
// Returns 0 on success, negative on failure.
// extradata_out/extradata_size: AVCC SPS+PPS bytes if available (may be 0).
static int encoder_init(
    int w, int h, int fps, int bitrate,
    unsigned char *extradata_out, int *extradata_size)
{
    *extradata_size = 0;
    HRESULT hr;

    hr = MFStartup(MF_VERSION, MFSTARTUP_NOSOCKET);
    if (FAILED(hr)) return -1;

    hr = CoInitializeEx(NULL, COINIT_MULTITHREADED);
    // E_ALREADY_INIT is fine

    hr = CoCreateInstance(
        &CLSID_H264Encoder, NULL, CLSCTX_INPROC_SERVER,
        &IID_IMFTransform, (void**)&g_encoder);
    if (FAILED(hr)) return -2;

    // ── Output type: H.264 ────────────────────────────────────────────────────
    IMFMediaType *outType = NULL;
    MFCreateMediaType(&outType);
    IMFMediaType_SetGUID(outType, &MF_MT_MAJOR_TYPE, &MFMediaType_Video);
    IMFMediaType_SetGUID(outType, &MF_MT_SUBTYPE, &MFVideoFormat_H264);
    IMFMediaType_SetUINT32(outType, &MF_MT_AVG_BITRATE, (UINT32)bitrate);
    IMFMediaType_SetUINT32(outType, &MF_MT_INTERLACE_MODE, MFVideoInterlace_Progressive);
    // MFSetAttributeSize/Ratio are C++ inline helpers — use SetUINT64 directly (same packing)
    IMFMediaType_SetUINT64(outType, &MF_MT_FRAME_SIZE, ((UINT64)(UINT32)w << 32) | (UINT64)(UINT32)h);
    IMFMediaType_SetUINT64(outType, &MF_MT_FRAME_RATE, ((UINT64)(UINT32)fps << 32) | 1ULL);
    IMFMediaType_SetUINT64(outType, &MF_MT_PIXEL_ASPECT_RATIO, (1ULL << 32) | 1ULL);

    hr = IMFTransform_SetOutputType(g_encoder, 0, outType, 0);
    IMFMediaType_Release(outType);
    if (FAILED(hr)) { IUnknown_Release(g_encoder); g_encoder = NULL; return -3; }

    // ── Input type: NV12 ──────────────────────────────────────────────────────
    IMFMediaType *inType = NULL;
    MFCreateMediaType(&inType);
    IMFMediaType_SetGUID(inType, &MF_MT_MAJOR_TYPE, &MFMediaType_Video);
    IMFMediaType_SetGUID(inType, &MF_MT_SUBTYPE, &MFVideoFormat_NV12);
    IMFMediaType_SetUINT32(inType, &MF_MT_INTERLACE_MODE, MFVideoInterlace_Progressive);
    IMFMediaType_SetUINT64(inType, &MF_MT_FRAME_SIZE, ((UINT64)(UINT32)w << 32) | (UINT64)(UINT32)h);
    IMFMediaType_SetUINT64(inType, &MF_MT_FRAME_RATE, ((UINT64)(UINT32)fps << 32) | 1ULL);
    IMFMediaType_SetUINT64(inType, &MF_MT_PIXEL_ASPECT_RATIO, (1ULL << 32) | 1ULL);

    hr = IMFTransform_SetInputType(g_encoder, 0, inType, 0);
    IMFMediaType_Release(inType);
    if (FAILED(hr)) { IUnknown_Release(g_encoder); g_encoder = NULL; return -4; }

    // ── Start streaming ───────────────────────────────────────────────────────
    IMFTransform_ProcessMessage(g_encoder, MFT_MESSAGE_COMMAND_FLUSH, 0);
    IMFTransform_ProcessMessage(g_encoder, MFT_MESSAGE_NOTIFY_BEGIN_STREAMING, 0);
    IMFTransform_ProcessMessage(g_encoder, MFT_MESSAGE_NOTIFY_START_OF_STREAM, 0);

    // ── Extract SPS/PPS (codec private data) ─────────────────────────────────
    IMFMediaType *curOutType = NULL;
    if (SUCCEEDED(IMFTransform_GetOutputCurrentType(g_encoder, 0, &curOutType))) {
        UINT8 *seqHdr = NULL;
        UINT32 seqLen = 0;
        if (SUCCEEDED(IMFMediaType_GetAllocatedBlob(curOutType,
                &MF_MT_MPEG_SEQUENCE_HEADER, &seqHdr, &seqLen))
            && seqLen > 0 && seqLen < 256)
        {
            memcpy(extradata_out, seqHdr, seqLen);
            *extradata_size = (int)seqLen;
            CoTaskMemFree(seqHdr);
        }
        IMFMediaType_Release(curOutType);
    }

    g_enc_w   = w;
    g_enc_h   = h;
    g_fps     = fps;
    g_bitrate = bitrate;
    return 0;
}

// encode_frame: submits one BGRA frame, drains all available output.
// Returns bytes written to out_buf (0 = encoder still buffering), -1 = error.
// pts_100ns: presentation timestamp in 100-nanosecond units.
static int encode_frame(
    const unsigned char *bgra, int w, int h,
    long long pts_100ns,
    unsigned char *out_buf, int out_cap)
{
    if (!g_encoder) return -1;

    // ── Convert BGRA → NV12 ──────────────────────────────────────────────────
    int nv12_size = w * h + (w * h / 2);
    unsigned char *nv12 = (unsigned char*)malloc(nv12_size);
    if (!nv12) return -1;
    bgra_to_nv12(bgra, w, h, nv12, nv12 + w * h);

    // ── Wrap in MF sample ─────────────────────────────────────────────────────
    IMFMediaBuffer *inBuf = NULL;
    HRESULT hr = MFCreateMemoryBuffer((DWORD)nv12_size, &inBuf);
    if (FAILED(hr)) { free(nv12); return -1; }

    BYTE *dst = NULL;
    IMFMediaBuffer_Lock(inBuf, &dst, NULL, NULL);
    memcpy(dst, nv12, nv12_size);
    IMFMediaBuffer_Unlock(inBuf);
    IMFMediaBuffer_SetCurrentLength(inBuf, (DWORD)nv12_size);
    free(nv12);

    IMFSample *inSample = NULL;
    MFCreateSample(&inSample);
    IMFSample_AddBuffer(inSample, inBuf);
    IMFSample_SetSampleTime(inSample, (LONGLONG)pts_100ns);
    IMFSample_SetSampleDuration(inSample,
        (LONGLONG)(10000000 / g_fps)); // 100ns units
    IMFMediaBuffer_Release(inBuf);

    hr = IMFTransform_ProcessInput(g_encoder, 0, inSample, 0);
    IMFSample_Release(inSample);
    if (FAILED(hr) && hr != MF_E_NOTACCEPTING) return -1;

    // ── Drain output ──────────────────────────────────────────────────────────
    int total = 0;
    for (;;) {
        IMFSample *outSample = NULL;
        MFCreateSample(&outSample);
        IMFMediaBuffer *outBuf = NULL;
        MFCreateMemoryBuffer(2*1024*1024, &outBuf);
        IMFSample_AddBuffer(outSample, outBuf);

        MFT_OUTPUT_DATA_BUFFER outData = {0};
        outData.pSample = outSample;
        DWORD status = 0;

        hr = IMFTransform_ProcessOutput(g_encoder, 0, 1, &outData, &status);

        if (hr == MF_E_TRANSFORM_NEED_MORE_INPUT) {
            IMFMediaBuffer_Release(outBuf);
            IMFSample_Release(outSample);
            break;
        }
        if (FAILED(hr)) {
            IMFMediaBuffer_Release(outBuf);
            IMFSample_Release(outSample);
            break;
        }

        // Collect all buffers from the output sample
        DWORD bufCount = 0;
        IMFSample_GetBufferCount(outData.pSample, &bufCount);
        for (DWORD i = 0; i < bufCount; i++) {
            IMFMediaBuffer *b = NULL;
            IMFSample_GetBufferByIndex(outData.pSample, i, &b);
            BYTE *data = NULL; DWORD len = 0;
            IMFMediaBuffer_Lock(b, &data, NULL, &len);
            if (total + (int)len <= out_cap) {
                memcpy(out_buf + total, data, len);
                total += (int)len;
            }
            IMFMediaBuffer_Unlock(b);
            IMFMediaBuffer_Release(b);
        }

        IMFMediaBuffer_Release(outBuf);
        IMFSample_Release(outData.pSample);
    }

    return total;
}

static void encoder_close(void) {
    if (g_encoder) {
        IMFTransform_ProcessMessage(g_encoder, MFT_MESSAGE_NOTIFY_END_OF_STREAM, 0);
        IMFTransform_ProcessMessage(g_encoder, MFT_MESSAGE_COMMAND_DRAIN, 0);
        IUnknown_Release(g_encoder);
        g_encoder = NULL;
    }
    MFShutdown();
}
*/
import "C"
import (
	"encoding/base64"
	"fmt"
	"unsafe"
)

const maxNALBuf = 4 * 1024 * 1024 // 4 MB output buffer

var (
	encoderInitDone bool
	nalBuf          = make([]byte, maxNALBuf)
)

func encoderInit(width, height, fps, bitrate int) (extradata []byte, err error) {
	extBuf := make([]byte, 256)
	var extSize C.int

	ret := int(C.encoder_init(
		C.int(width), C.int(height), C.int(fps), C.int(bitrate),
		(*C.uchar)(unsafe.Pointer(&extBuf[0])),
		&extSize,
	))
	if ret < 0 {
		return nil, fmt.Errorf("WMF encoder init failed (code %d)", ret)
	}
	encoderInitDone = true

	if int(extSize) > 0 {
		// Return as base64 string (JSON-friendly for the init message)
		raw := extBuf[:int(extSize)]
		b64 := base64.StdEncoding.EncodeToString(raw)
		return []byte(b64), nil
	}
	return nil, nil
}

func encodeFrame(bgra []byte, width, height int, pts int64) ([]byte, error) {
	if !encoderInitDone {
		return nil, fmt.Errorf("encoder not initialised")
	}

	n := int(C.encode_frame(
		(*C.uchar)(unsafe.Pointer(&bgra[0])),
		C.int(width), C.int(height),
		C.longlong(pts),
		(*C.uchar)(unsafe.Pointer(&nalBuf[0])),
		C.int(maxNALBuf),
	))
	if n < 0 {
		return nil, fmt.Errorf("encode_frame failed")
	}
	if n == 0 {
		return nil, nil
	}

	out := make([]byte, n)
	copy(out, nalBuf[:n])
	return out, nil
}

func encoderClose() {
	if encoderInitDone {
		C.encoder_close()
		encoderInitDone = false
	}
}
