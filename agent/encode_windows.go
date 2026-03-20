//go:build windows

package main

/*
#cgo CFLAGS: -DCOBJMACROS -DINITGUID
#cgo LDFLAGS: -lmfplat -lmf -lmfuuid -luuid -lole32 -loleaut32 -lstrmiids

#include <windows.h>
#include <stdio.h>
#include <mfapi.h>
#include <mfidl.h>
#include <mftransform.h>
#include <mferror.h>
#include <codecapi.h>
#include <stdlib.h>
#include <string.h>

static IMFTransform *g_encoder   = NULL;
static int           g_enc_w     = 0;
static int           g_enc_h     = 0;
static int           g_fps       = 15;
static int           g_bitrate   = 3000000;
static int           g_frame_count = 0;
static int           g_mft_provides_samples = 0;

// BGRA to NV12 (BT.601 limited range, CPU software conversion)
static void bgra_to_nv12(
    const unsigned char *bgra, int w, int h,
    unsigned char *y_plane, unsigned char *uv_plane)
{
    int stride = w * 4;
    int row, col;
    for (row = 0; row < h; row++) {
        const unsigned char *src = bgra + row * stride;
        unsigned char *ydst = y_plane + row * w;
        for (col = 0; col < w; col++, src += 4) {
            int b = src[0], g = src[1], r = src[2];
            ydst[col] = (unsigned char)(((77*r + 150*g + 29*b + 128) >> 8) + 16);
        }
    }
    for (row = 0; row < h/2; row++) {
        const unsigned char *s0 = bgra + (row*2)   * stride;
        const unsigned char *s1 = bgra + (row*2+1) * stride;
        unsigned char *uvdst = uv_plane + row * w;
        for (col = 0; col < w/2; col++) {
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
    // H.264 encoder CLSIDs tried in order:
    //   [0] CLSID_MSH264EncoderMFT {6ca50344-...} - pure software, works in all sessions.
    //   [1] {a7e2c842-...}                         - hardware-assisted fallback.
    static const GUID h264Clsids[2] = {
        { 0x6ca50344, 0x051a, 0x4ded, {0x97, 0x79, 0xa4, 0x33, 0x05, 0x16, 0x5e, 0x35} },
        { 0xa7e2c842, 0x2f83, 0x4d5a, {0xad, 0xbd, 0xca, 0x38, 0x55, 0xe2, 0xf8, 0x24} }
    };

    HRESULT hr;
    IMFMediaType *outType = NULL;
    IMFMediaType *inType = NULL;
    int n;

    *extradata_size = 0;

    // COM must be initialised before MFStartup.
    hr = CoInitializeEx(NULL, COINIT_MULTITHREADED);
    // S_FALSE = already initialised — fine to continue.

    hr = MFStartup(MF_VERSION, MFSTARTUP_NOSOCKET);
    if (FAILED(hr)) return -1;

    // Try each CLSID; use the first one that creates successfully.
    hr = E_FAIL;
    for (n = 0; n < 2; n++) {
        hr = CoCreateInstance(
            &h264Clsids[n], NULL, CLSCTX_INPROC_SERVER,
            &IID_IMFTransform, (void**)&g_encoder);
        if (SUCCEEDED(hr)) break;
    }
    if (FAILED(hr)) return -2;

    // Enable low-latency mode via ICodecAPI BEFORE setting media types
    // (Microsoft docs: "Set encoding properties before calling SetOutputType").
    // GUIDs defined manually — MinGW headers do not include these.
    {
        static const GUID AVLowLatencyMode =
            { 0x9c27891a, 0xed7a, 0x40e1, {0x88, 0xe8, 0xb2, 0x27, 0x27, 0xa0, 0x24, 0xee} };
        static const GUID AVEncMPVGOPSize =
            { 0x95f31b26, 0x95a4, 0x41aa, {0x93, 0x03, 0x24, 0x6a, 0x7f, 0xc6, 0xee, 0xf1} };

        ICodecAPI *codecApi = NULL;
        hr = IUnknown_QueryInterface(g_encoder, &IID_ICodecAPI, (void**)&codecApi);
        if (SUCCEEDED(hr) && codecApi) {
            VARIANT v;
            VariantInit(&v);

            // AVLowLatencyMode = TRUE — minimal buffering, produce output ASAP
            v.vt = VT_BOOL;
            v.boolVal = VARIANT_TRUE;
            ICodecAPI_SetValue(codecApi, &AVLowLatencyMode, &v);

            // Force IDR every N frames (short GOP for keyframe detection)
            VariantInit(&v);
            v.vt = VT_UI4;
            v.ulVal = (ULONG)fps;  // IDR every ~1 second
            ICodecAPI_SetValue(codecApi, &AVEncMPVGOPSize, &v);

            IUnknown_Release(codecApi);
        }
    }

    // Output type: H.264
    MFCreateMediaType(&outType);
    IMFMediaType_SetGUID(outType, &MF_MT_MAJOR_TYPE, &MFMediaType_Video);
    IMFMediaType_SetGUID(outType, &MF_MT_SUBTYPE, &MFVideoFormat_H264);
    IMFMediaType_SetUINT32(outType, &MF_MT_AVG_BITRATE, (UINT32)bitrate);
    IMFMediaType_SetUINT32(outType, &MF_MT_INTERLACE_MODE, MFVideoInterlace_Progressive);
    IMFMediaType_SetUINT64(outType, &MF_MT_FRAME_SIZE,
        ((UINT64)(UINT32)w << 32) | (UINT64)(UINT32)h);
    IMFMediaType_SetUINT64(outType, &MF_MT_FRAME_RATE,
        ((UINT64)(UINT32)fps << 32) | 1ULL);
    IMFMediaType_SetUINT64(outType, &MF_MT_PIXEL_ASPECT_RATIO, (1ULL << 32) | 1ULL);

    hr = IMFTransform_SetOutputType(g_encoder, 0, outType, 0);
    IMFMediaType_Release(outType);
    if (FAILED(hr)) { IUnknown_Release(g_encoder); g_encoder = NULL; return -3; }

    // Input type: NV12
    MFCreateMediaType(&inType);
    IMFMediaType_SetGUID(inType, &MF_MT_MAJOR_TYPE, &MFMediaType_Video);
    IMFMediaType_SetGUID(inType, &MF_MT_SUBTYPE, &MFVideoFormat_NV12);
    IMFMediaType_SetUINT32(inType, &MF_MT_INTERLACE_MODE, MFVideoInterlace_Progressive);
    IMFMediaType_SetUINT64(inType, &MF_MT_FRAME_SIZE,
        ((UINT64)(UINT32)w << 32) | (UINT64)(UINT32)h);
    IMFMediaType_SetUINT64(inType, &MF_MT_FRAME_RATE,
        ((UINT64)(UINT32)fps << 32) | 1ULL);
    IMFMediaType_SetUINT64(inType, &MF_MT_PIXEL_ASPECT_RATIO, (1ULL << 32) | 1ULL);

    // Set the stride so the MFT knows the memory layout of our NV12 data.
    // For NV12, default stride = width (bytes per row of Y plane).
    IMFMediaType_SetUINT32(inType, &MF_MT_DEFAULT_STRIDE, (UINT32)w);

    hr = IMFTransform_SetInputType(g_encoder, 0, inType, 0);
    IMFMediaType_Release(inType);
    if (FAILED(hr)) { IUnknown_Release(g_encoder); g_encoder = NULL; return -4; }

    // Check if the MFT provides its own output samples.
    {
        MFT_OUTPUT_STREAM_INFO osi;
        memset(&osi, 0, sizeof(osi));
        hr = IMFTransform_GetOutputStreamInfo(g_encoder, 0, &osi);
        if (SUCCEEDED(hr) && (osi.dwFlags & MFT_OUTPUT_STREAM_PROVIDES_SAMPLES)) {
            g_mft_provides_samples = 1;
        }
    }

    // Write diagnostic to a dedicated file (bypasses Go log buffering issues)
    {
        const char *diagpath = "C:\\Users\\Public\\oblireach-encoder-diag.log";
        FILE *diag = fopen(diagpath, "a");
        if (diag) {
            fprintf(diag, "encoder_init: %dx%d@%dfps br=%d mft_provides=%d\n",
                    w, h, fps, bitrate, g_mft_provides_samples);

            // Enumerate supported input types
            int idx;
            for (idx = 0; idx < 10; idx++) {
                IMFMediaType *avail = NULL;
                GUID subtype;
                hr = IMFTransform_GetInputAvailableType(g_encoder, 0, idx, &avail);
                if (FAILED(hr)) break;
                IMFMediaType_GetGUID(avail, &MF_MT_SUBTYPE, &subtype);
                fprintf(diag, "  input type[%d]: subtype={%08x-%04x-%04x-...}\n",
                        idx, (unsigned)subtype.Data1,
                        (unsigned)subtype.Data2, (unsigned)subtype.Data3);
                IMFMediaType_Release(avail);
            }
            fclose(diag);
        }
    }

    IMFTransform_ProcessMessage(g_encoder, MFT_MESSAGE_NOTIFY_BEGIN_STREAMING, 0);
    IMFTransform_ProcessMessage(g_encoder, MFT_MESSAGE_NOTIFY_START_OF_STREAM, 0);

    // SPS/PPS are embedded inline in the first IDR output sample (Annex B format).
    // Do not use MF_MT_MPEG_SEQUENCE_HEADER as extradata: it is Annex B, not AVCC,
    // and would corrupt the WebCodecs VideoDecoder description field.
    // extradata_size remains 0; the browser decoder handles in-band SPS/PPS.

    g_enc_w   = w;
    g_enc_h   = h;
    g_fps     = fps;
    g_bitrate = bitrate;
    g_frame_count = 0;
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
    int nv12_size;
    unsigned char *nv12;
    IMFMediaBuffer *inBuf;
    BYTE *dst;
    IMFSample *inSample;
    HRESULT hr;
    int total;

    if (!g_encoder) return -1;

    if (g_frame_count < 5 || g_frame_count == 15 || g_frame_count == 30) {
        FILE *df = fopen("C:\\Users\\Public\\oblireach-encoder-diag.log", "a");
        if (df) {
            fprintf(df, "encode_frame #%d: %dx%d provides=%d\n",
                    g_frame_count, w, h, g_mft_provides_samples);
            fclose(df);
        }
    }

    nv12_size = w * h + (w * h / 2);
    nv12 = (unsigned char*)malloc(nv12_size);
    if (!nv12) return -1;
    bgra_to_nv12(bgra, w, h, nv12, nv12 + w * h);

    inBuf = NULL;
    hr = MFCreateMemoryBuffer((DWORD)nv12_size, &inBuf);
    if (FAILED(hr)) { free(nv12); return -1; }

    dst = NULL;
    IMFMediaBuffer_Lock(inBuf, &dst, NULL, NULL);
    memcpy(dst, nv12, nv12_size);
    IMFMediaBuffer_Unlock(inBuf);
    IMFMediaBuffer_SetCurrentLength(inBuf, (DWORD)nv12_size);
    free(nv12);

    inSample = NULL;
    MFCreateSample(&inSample);
    IMFSample_AddBuffer(inSample, inBuf);
    IMFSample_SetSampleTime(inSample, (LONGLONG)pts_100ns);
    IMFSample_SetSampleDuration(inSample, (LONGLONG)(10000000 / g_fps));
    IMFMediaBuffer_Release(inBuf);

    hr = IMFTransform_ProcessInput(g_encoder, 0, inSample, 0);
    IMFSample_Release(inSample);
    if (FAILED(hr) && hr != MF_E_NOTACCEPTING) return -(int)(hr & 0xFFFF);

    // Force the encoder to produce output after each input.  Without this,
    // the software H.264 MFT on Windows Server buffers indefinitely.
    // DRAIN tells the encoder "no more input is coming, flush everything".
    // After draining, we restart streaming so the next ProcessInput works.
    IMFTransform_ProcessMessage(g_encoder, MFT_MESSAGE_COMMAND_DRAIN, 0);

    g_frame_count++;
    total = 0;
    for (;;) {
        IMFSample *outSample = NULL;
        IMFMediaBuffer *outBuf = NULL;
        MFT_OUTPUT_DATA_BUFFER outData;
        DWORD status = 0;
        DWORD bufCount = 0;
        DWORD i;
        int we_own_sample = 0;

        memset(&outData, 0, sizeof(outData));

        if (g_mft_provides_samples) {
            // MFT provides its own sample — do NOT supply one
            outData.pSample = NULL;
        } else {
            // We must provide the output sample
            MFCreateSample(&outSample);
            MFCreateMemoryBuffer(2*1024*1024, &outBuf);
            IMFSample_AddBuffer(outSample, outBuf);
            outData.pSample = outSample;
            we_own_sample = 1;
        }

        hr = IMFTransform_ProcessOutput(g_encoder, 0, 1, &outData, &status);

        if (hr == MF_E_TRANSFORM_NEED_MORE_INPUT) {
            if (g_frame_count <= 5) {
                FILE *df2 = fopen("C:\\Users\\Public\\oblireach-encoder-diag.log", "a");
                if (df2) {
                    fprintf(df2, "  ProcessOutput #%d: NEED_MORE_INPUT (total=%d)\n",
                            g_frame_count, total);
                    fclose(df2);
                }
            }
            if (we_own_sample) {
                IMFMediaBuffer_Release(outBuf);
                IMFSample_Release(outSample);
            }
            break;
        }
        if (FAILED(hr)) {
            {
                FILE *df3 = fopen("C:\\Users\\Public\\oblireach-encoder-diag.log", "a");
                if (df3) {
                    fprintf(df3, "  ProcessOutput #%d: FAILED hr=0x%08x\n",
                            g_frame_count, (unsigned)hr);
                    fclose(df3);
                }
            }
            if (we_own_sample) {
                IMFMediaBuffer_Release(outBuf);
                IMFSample_Release(outSample);
            }
            if (outData.pSample && !we_own_sample) {
                IMFSample_Release(outData.pSample);
            }
            break;
        }

        // Successfully got output — extract bytes
        if (outData.pSample) {
            IMFSample_GetBufferCount(outData.pSample, &bufCount);
            for (i = 0; i < bufCount; i++) {
                IMFMediaBuffer *b = NULL;
                BYTE *data = NULL;
                DWORD len = 0;
                IMFSample_GetBufferByIndex(outData.pSample, i, &b);
                IMFMediaBuffer_Lock(b, &data, NULL, &len);
                if (total + (int)len <= out_cap) {
                    memcpy(out_buf + total, data, len);
                    total += (int)len;
                }
                IMFMediaBuffer_Unlock(b);
                IMFMediaBuffer_Release(b);
            }
        }

        if (we_own_sample) {
            IMFMediaBuffer_Release(outBuf);
            IMFSample_Release(outData.pSample);
        } else if (outData.pSample) {
            // MFT-provided sample — we must release it
            IMFSample_Release(outData.pSample);
        }
    }

    // Restart streaming after drain so the next ProcessInput is accepted.
    if (total > 0 || g_frame_count > 1) {
        IMFTransform_ProcessMessage(g_encoder, MFT_MESSAGE_NOTIFY_START_OF_STREAM, 0);
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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"unsafe"
)

const maxNALBuf = 4 * 1024 * 1024 // 4 MB output buffer

var (
	encoderInitDone   bool
	nalBuf            = make([]byte, maxNALBuf)
	encodeInputCount  int
	encodeOutputCount int
	encodeDiagFile    *os.File
)

// openEncoderDiag opens a diagnostic file next to the helper log.
func openEncoderDiag() {
	// Try multiple paths — the helper's environment may be unexpected
	paths := []string{
		filepath.Join(os.TempDir(), "oblireach-encoder-diag.log"),
		`C:\Users\Public\oblireach-encoder-diag.log`,
		`C:\ProgramData\ObliReachAgent\encoder-diag.log`,
	}
	for _, p := range paths {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err == nil {
			encodeDiagFile = f
			return
		}
	}
}

// SetEncoderDiagFromLog allows the helper to redirect encoder diagnostics
// to the same writer as the helper log (which is known to work).
func SetEncoderDiagFromLog(f *os.File) {
	encodeDiagFile = f
}

func diagf(format string, args ...interface{}) {
	if encodeDiagFile != nil {
		fmt.Fprintf(encodeDiagFile, format+"\n", args...)
		encodeDiagFile.Sync()
	}
}

func encoderInit(width, height, fps, bitrate int) (extradata []byte, err error) {
	openEncoderDiag()
	diagf("encoderInit: %dx%d@%dfps bitrate=%d", width, height, fps, bitrate)

	extBuf := make([]byte, 256)
	var extSize C.int

	ret := int(C.encoder_init(
		C.int(width), C.int(height), C.int(fps), C.int(bitrate),
		(*C.uchar)(unsafe.Pointer(&extBuf[0])),
		&extSize,
	))
	if ret < 0 {
		diagf("encoderInit: FAILED code=%d", ret)
		return nil, fmt.Errorf("WMF encoder init failed (code %d)", ret)
	}
	encoderInitDone = true
	diagf("encoderInit: OK")
	// extSize is always 0: SPS/PPS are embedded inline in the H.264 bitstream.
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
	encodeInputCount++
	if n < 0 {
		diagf("encodeFrame #%d: FAILED code=%d", encodeInputCount, n)
		log.Printf("encode: frame %d failed (code %d)", encodeInputCount, n)
		return nil, fmt.Errorf("encode_frame failed (code %d)", n)
	}
	if n == 0 {
		if encodeInputCount <= 10 || encodeInputCount%100 == 0 {
			diagf("encodeFrame #%d: 0 bytes (buffering, %d produced)", encodeInputCount, encodeOutputCount)
		}
		return nil, nil
	}

	encodeOutputCount++
	diagf("encodeFrame #%d: %d bytes (output #%d)", encodeInputCount, n, encodeOutputCount)
	log.Printf("encode: frame %d → %d bytes (output #%d)", encodeInputCount, n, encodeOutputCount)

	out := make([]byte, n)
	copy(out, nalBuf[:n])
	return ensureAnnexB(out), nil
}

// ensureAnnexB detects whether the H.264 data is Annex B (start codes) or
// AVCC (4-byte length prefixes) and converts AVCC→Annex B if needed.
// The WMF H.264 encoder *should* output Annex B, but some configurations
// or hardware-assisted MFTs produce AVCC instead.
func ensureAnnexB(data []byte) []byte {
	if len(data) < 5 {
		return data
	}
	// Check for Annex B start codes: 00 00 00 01 or 00 00 01
	if (data[0] == 0 && data[1] == 0 && data[2] == 0 && data[3] == 1) ||
		(data[0] == 0 && data[1] == 0 && data[2] == 1) {
		return data // already Annex B
	}

	// Assume AVCC: each NAL unit prefixed by 4-byte big-endian length.
	// Convert to Annex B by replacing length prefixes with 00 00 00 01.
	startCode := []byte{0x00, 0x00, 0x00, 0x01}
	var out []byte
	i := 0
	for i+4 <= len(data) {
		nalLen := int(data[i])<<24 | int(data[i+1])<<16 | int(data[i+2])<<8 | int(data[i+3])
		i += 4
		if nalLen <= 0 || i+nalLen > len(data) {
			// Invalid length — return original data unchanged
			return data
		}
		out = append(out, startCode...)
		out = append(out, data[i:i+nalLen]...)
		i += nalLen
	}
	if len(out) == 0 {
		return data
	}
	return out
}

func encoderClose() {
	if encoderInitDone {
		C.encoder_close()
		encoderInitDone = false
	}
}
