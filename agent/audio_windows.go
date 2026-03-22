//go:build windows

package main

/*
#cgo LDFLAGS: -lole32 -loleaut32

#include <windows.h>
#include <mmdeviceapi.h>
#include <audioclient.h>
#include <stdlib.h>
#include <string.h>

// MinGW may not have these — define manually
static const GUID g_CLSID_MMDeviceEnumerator = {0xBCDE0395, 0xE52F, 0x467C, {0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}};
static const GUID g_IID_IMMDeviceEnumerator  = {0xA95664D2, 0x9614, 0x4F35, {0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}};
static const GUID g_IID_IAudioClient         = {0x1CB9AD4C, 0xDBFA, 0x4C32, {0xB1, 0x78, 0xC2, 0xF5, 0x68, 0xA7, 0x03, 0xB2}};
static const GUID g_IID_IAudioCaptureClient  = {0xC8ADBD64, 0xE71E, 0x48A0, {0xA4, 0xDE, 0x18, 0x5C, 0x39, 0x5C, 0xD3, 0x17}};

static IAudioClient *g_audioClient = NULL;
static IAudioCaptureClient *g_captureClient = NULL;
static WAVEFORMATEX *g_audioFmt = NULL;
static int g_audioInit = 0;

static int audio_init(void) {
    if (g_audioInit) return 0;
    HRESULT hr;

    CoInitializeEx(NULL, COINIT_MULTITHREADED);

    IMMDeviceEnumerator *enumerator = NULL;
    hr = CoCreateInstance(&g_CLSID_MMDeviceEnumerator, NULL, CLSCTX_ALL,
                          &g_IID_IMMDeviceEnumerator, (void**)&enumerator);
    if (FAILED(hr)) return -1;

    IMMDevice *device = NULL;
    hr = IMMDeviceEnumerator_GetDefaultAudioEndpoint(enumerator, eRender, eConsole, &device);
    IUnknown_Release(enumerator);
    if (FAILED(hr)) return -2;

    hr = IMMDevice_Activate(device, &g_IID_IAudioClient, CLSCTX_ALL, NULL, (void**)&g_audioClient);
    IUnknown_Release(device);
    if (FAILED(hr)) return -3;

    hr = IAudioClient_GetMixFormat(g_audioClient, &g_audioFmt);
    if (FAILED(hr)) return -4;

    // Initialize in loopback mode (capture system audio output)
    REFERENCE_TIME bufDuration = 200000; // 20ms buffer
    hr = IAudioClient_Initialize(g_audioClient, AUDCLNT_SHAREMODE_SHARED,
        AUDCLNT_STREAMFLAGS_LOOPBACK, bufDuration, 0, g_audioFmt, NULL);
    if (FAILED(hr)) return -5;

    hr = IAudioClient_GetService(g_audioClient, &g_IID_IAudioCaptureClient, (void**)&g_captureClient);
    if (FAILED(hr)) return -6;

    hr = IAudioClient_Start(g_audioClient);
    if (FAILED(hr)) return -7;

    g_audioInit = 1;
    return 0;
}

// audio_capture: reads available audio data into out_buf as 16-bit PCM mono.
// Returns number of bytes written, 0 = no data, negative = error.
static int audio_capture(unsigned char *out_buf, int out_cap) {
    if (!g_audioInit || !g_captureClient) return -1;

    UINT32 packetLen = 0;
    IAudioCaptureClient_GetNextPacketSize(g_captureClient, &packetLen);
    if (packetLen == 0) return 0;

    int total = 0;
    while (packetLen > 0) {
        BYTE *data = NULL;
        UINT32 frames = 0;
        DWORD flags = 0;
        HRESULT hr = IAudioCaptureClient_GetBuffer(g_captureClient, &data, &frames, &flags, NULL, NULL);
        if (FAILED(hr)) break;

        // Convert float32 stereo to int16 mono
        int channels = g_audioFmt->nChannels;
        UINT32 i;
        for (i = 0; i < frames && total + 2 <= out_cap; i++) {
            float sample = 0;
            if (flags & AUDCLNT_BUFFERFLAGS_SILENT) {
                sample = 0;
            } else {
                // Average all channels to mono
                float sum = 0;
                int ch;
                for (ch = 0; ch < channels; ch++) {
                    sum += ((float*)data)[i * channels + ch];
                }
                sample = sum / channels;
            }
            // Clamp and convert to int16
            if (sample > 1.0f) sample = 1.0f;
            if (sample < -1.0f) sample = -1.0f;
            short s16 = (short)(sample * 32767.0f);
            out_buf[total] = (unsigned char)(s16 & 0xFF);
            out_buf[total+1] = (unsigned char)((s16 >> 8) & 0xFF);
            total += 2;
        }

        IAudioCaptureClient_ReleaseBuffer(g_captureClient, frames);
        IAudioCaptureClient_GetNextPacketSize(g_captureClient, &packetLen);
    }
    return total;
}

static void audio_close(void) {
    if (g_audioClient) {
        IAudioClient_Stop(g_audioClient);
        IUnknown_Release(g_audioClient);
        g_audioClient = NULL;
    }
    if (g_captureClient) {
        IUnknown_Release(g_captureClient);
        g_captureClient = NULL;
    }
    if (g_audioFmt) {
        CoTaskMemFree(g_audioFmt);
        g_audioFmt = NULL;
    }
    g_audioInit = 0;
}

static int audio_sample_rate(void) {
    return g_audioFmt ? (int)g_audioFmt->nSamplesPerSec : 48000;
}
*/
import "C"
import (
	"log"
	"unsafe"
)

const audioMaxBuf = 64 * 1024 // 64KB audio buffer

var (
	audioInitDone bool
	audioBuf      = make([]byte, audioMaxBuf)
)

func audioInit() error {
	rc := int(C.audio_init())
	if rc != 0 {
		return nil // audio not available — not fatal
	}
	audioInitDone = true
	log.Printf("Audio capture initialized (sample rate: %d Hz)", int(C.audio_sample_rate()))
	return nil
}

func audioCapture() []byte {
	if !audioInitDone {
		return nil
	}
	n := int(C.audio_capture(
		(*C.uchar)(unsafe.Pointer(&audioBuf[0])),
		C.int(audioMaxBuf),
	))
	if n <= 0 {
		return nil
	}
	out := make([]byte, n)
	copy(out, audioBuf[:n])
	return out
}

func audioClose() {
	if audioInitDone {
		C.audio_close()
		audioInitDone = false
	}
}

func audioSampleRate() int {
	if !audioInitDone {
		return 48000
	}
	return int(C.audio_sample_rate())
}
