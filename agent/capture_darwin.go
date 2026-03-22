//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fmodules
#cgo LDFLAGS: -framework ScreenCaptureKit -framework CoreMedia -framework CoreVideo -framework CoreGraphics -framework CoreFoundation -framework Foundation

#include <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// ── Monitor enumeration (CoreGraphics — NOT deprecated) ─────────────────────

#define MAC_MAX_MONITORS 16

typedef struct {
    int index;
    char name[64];
    int x, y, w, h;
    uint32_t displayID;
} MacMonitorInfo;

static int      g_mac_width   = 0;
static int      g_mac_height  = 0;
static int      g_mac_mon_x   = 0;
static int      g_mac_mon_y   = 0;
static uint32_t g_mac_display = 0;

static int mac_enumerate_monitors(MacMonitorInfo *out, int maxCount) {
    uint32_t displayCount = 0;
    CGDirectDisplayID displays[MAC_MAX_MONITORS];
    CGGetActiveDisplayList(MAC_MAX_MONITORS, displays, &displayCount);

    int count = 0;
    uint32_t i;
    for (i = 0; i < displayCount && count < maxCount; i++) {
        CGRect bounds = CGDisplayBounds(displays[i]);
        out[count].index = count;
        out[count].displayID = displays[i];
        if (CGDisplayIsMain(displays[i])) {
            snprintf(out[count].name, 64, "Main Display");
        } else {
            snprintf(out[count].name, 64, "Display %d", i + 1);
        }
        out[count].x = (int)bounds.origin.x;
        out[count].y = (int)bounds.origin.y;
        out[count].w = (int)bounds.size.width;
        out[count].h = (int)bounds.size.height;
        count++;
    }
    return count;
}

// ── ScreenCaptureKit capture (Objective-C, macOS 12.3+) ─────────────────────
// We use the synchronous SCScreenshotManager API (macOS 14+) for simplicity.
// For older macOS, fall back to a CGDisplayStream approach.

#import <ScreenCaptureKit/ScreenCaptureKit.h>

// Global capture state
static unsigned char *g_mac_framebuf = NULL;
static int g_mac_framebuf_size = 0;
static dispatch_semaphore_t g_mac_sema = NULL;
static SCStream *g_mac_stream = NULL;
static SCContentFilter *g_mac_filter = NULL;
static SCStreamConfiguration *g_mac_config = NULL;
static int g_mac_capturing = 0;

// Stream delegate to receive frames
@interface ObliReachStreamDelegate : NSObject <SCStreamOutput>
@end

@implementation ObliReachStreamDelegate
- (void)stream:(SCStream *)stream didOutputSampleBuffer:(CMSampleBufferRef)sampleBuffer ofType:(SCStreamOutputType)type {
    if (type != SCStreamOutputTypeScreen) return;

    CVImageBufferRef imageBuffer = CMSampleBufferGetImageBuffer(sampleBuffer);
    if (!imageBuffer) return;

    CVPixelBufferLockBaseAddress(imageBuffer, kCVPixelBufferLock_ReadOnly);

    size_t width = CVPixelBufferGetWidth(imageBuffer);
    size_t height = CVPixelBufferGetHeight(imageBuffer);
    size_t bytesPerRow = CVPixelBufferGetBytesPerRow(imageBuffer);
    void *baseAddr = CVPixelBufferGetBaseAddress(imageBuffer);

    if (baseAddr && g_mac_framebuf) {
        int copyW = (int)(width < (size_t)g_mac_width ? width : (size_t)g_mac_width);
        int copyH = (int)(height < (size_t)g_mac_height ? height : (size_t)g_mac_height);
        int row;
        for (row = 0; row < copyH; row++) {
            memcpy(g_mac_framebuf + row * g_mac_width * 4,
                   (unsigned char*)baseAddr + row * bytesPerRow,
                   copyW * 4);
        }
    }

    CVPixelBufferUnlockBaseAddress(imageBuffer, kCVPixelBufferLock_ReadOnly);

    if (g_mac_sema) dispatch_semaphore_signal(g_mac_sema);
}
@end

static ObliReachStreamDelegate *g_mac_delegate = nil;

static int mac_capture_init(int monitor_idx) {
    MacMonitorInfo mons[MAC_MAX_MONITORS];
    int n = mac_enumerate_monitors(mons, MAC_MAX_MONITORS);

    if (monitor_idx >= 0 && monitor_idx < n) {
        g_mac_display = mons[monitor_idx].displayID;
        g_mac_width   = mons[monitor_idx].w;
        g_mac_height  = mons[monitor_idx].h;
        g_mac_mon_x   = mons[monitor_idx].x;
        g_mac_mon_y   = mons[monitor_idx].y;
    } else if (n > 0) {
        g_mac_display = mons[0].displayID;
        g_mac_width   = mons[0].w;
        g_mac_height  = mons[0].h;
        g_mac_mon_x   = 0;
        g_mac_mon_y   = 0;
    } else {
        g_mac_display = CGMainDisplayID();
        g_mac_width   = (int)CGDisplayPixelsWide(g_mac_display);
        g_mac_height  = (int)CGDisplayPixelsHigh(g_mac_display);
        g_mac_mon_x   = 0;
        g_mac_mon_y   = 0;
    }

    if (g_mac_width <= 0 || g_mac_height <= 0) return -1;

    // Allocate frame buffer
    g_mac_framebuf_size = g_mac_width * g_mac_height * 4;
    g_mac_framebuf = (unsigned char*)calloc(1, g_mac_framebuf_size);
    if (!g_mac_framebuf) return -2;

    g_mac_sema = dispatch_semaphore_create(0);

    // Set up ScreenCaptureKit stream
    __block int initResult = 0;
    __block BOOL initDone = NO;

    dispatch_semaphore_t setupSema = dispatch_semaphore_create(0);

    [SCShareableContent getShareableContentWithCompletionHandler:^(SCShareableContent *content, NSError *error) {
        if (error || !content) {
            initResult = -3;
            initDone = YES;
            dispatch_semaphore_signal(setupSema);
            return;
        }

        // Find the target display
        SCDisplay *targetDisplay = nil;
        for (SCDisplay *d in content.displays) {
            if (d.displayID == g_mac_display) {
                targetDisplay = d;
                break;
            }
        }
        if (!targetDisplay && content.displays.count > 0) {
            targetDisplay = content.displays[0];
        }
        if (!targetDisplay) {
            initResult = -4;
            initDone = YES;
            dispatch_semaphore_signal(setupSema);
            return;
        }

        // Create filter — capture the entire display
        g_mac_filter = [[SCContentFilter alloc] initWithDisplay:targetDisplay excludingWindows:@[]];

        // Configure stream
        g_mac_config = [[SCStreamConfiguration alloc] init];
        g_mac_config.width = g_mac_width;
        g_mac_config.height = g_mac_height;
        g_mac_config.minimumFrameInterval = CMTimeMake(1, 30); // 30 fps
        g_mac_config.pixelFormat = kCVPixelFormatType_32BGRA;
        g_mac_config.showsCursor = YES;

        // Create and start stream
        g_mac_delegate = [[ObliReachStreamDelegate alloc] init];
        g_mac_stream = [[SCStream alloc] initWithFilter:g_mac_filter configuration:g_mac_config delegate:nil];

        NSError *addErr = nil;
        [g_mac_stream addStreamOutput:g_mac_delegate type:SCStreamOutputTypeScreen sampleHandlerQueue:dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_HIGH, 0) error:&addErr];
        if (addErr) {
            initResult = -5;
            initDone = YES;
            dispatch_semaphore_signal(setupSema);
            return;
        }

        [g_mac_stream startCaptureWithCompletionHandler:^(NSError *startErr) {
            if (startErr) {
                initResult = -6;
            } else {
                g_mac_capturing = 1;
            }
            initDone = YES;
            dispatch_semaphore_signal(setupSema);
        }];
    }];

    // Wait for async setup (max 10 seconds)
    dispatch_semaphore_wait(setupSema, dispatch_time(DISPATCH_TIME_NOW, 10 * NSEC_PER_SEC));

    return initResult;
}

static void mac_capture_close(void) {
    if (g_mac_stream && g_mac_capturing) {
        dispatch_semaphore_t stopSema = dispatch_semaphore_create(0);
        [g_mac_stream stopCaptureWithCompletionHandler:^(NSError *err) {
            dispatch_semaphore_signal(stopSema);
        }];
        dispatch_semaphore_wait(stopSema, dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC));
    }
    g_mac_stream = nil;
    g_mac_filter = nil;
    g_mac_config = nil;
    g_mac_delegate = nil;
    g_mac_capturing = 0;
    if (g_mac_framebuf) { free(g_mac_framebuf); g_mac_framebuf = NULL; }
    g_mac_width = 0;
    g_mac_height = 0;
}

static void mac_get_size(int *w, int *h) {
    *w = g_mac_width;
    *h = g_mac_height;
}

static void mac_get_offset(int *ox, int *oy) {
    *ox = g_mac_mon_x;
    *oy = g_mac_mon_y;
}

// mac_capture_frame: copies the latest captured frame into the output buffer.
// The SCStream delegate continuously updates g_mac_framebuf in the background.
static int mac_capture_frame(unsigned char *out_bgra) {
    if (!g_mac_capturing || !g_mac_framebuf) return -1;

    // Wait for next frame (max 100ms)
    long result = dispatch_semaphore_wait(g_mac_sema, dispatch_time(DISPATCH_TIME_NOW, 100 * NSEC_PER_MSEC));
    if (result != 0) return 1; // timeout, no new frame

    memcpy(out_bgra, g_mac_framebuf, g_mac_width * g_mac_height * 4);
    return 0;
}
*/
import "C"
import (
	"fmt"
	"log"
	"unsafe"
)

type MonitorInfo struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func enumerateMonitors() []MonitorInfo {
	var cmons [16]C.MacMonitorInfo
	n := int(C.mac_enumerate_monitors(&cmons[0], C.int(16)))
	out := make([]MonitorInfo, n)
	for i := 0; i < n; i++ {
		out[i] = MonitorInfo{
			Index:  int(cmons[i].index),
			Name:   C.GoString(&cmons[i].name[0]),
			X:      int(cmons[i].x),
			Y:      int(cmons[i].y),
			Width:  int(cmons[i].w),
			Height: int(cmons[i].h),
		}
	}
	return out
}

var captureActive bool

func captureInitMonitor(idx int) error {
	ret := int(C.mac_capture_init(C.int(idx)))
	if ret < 0 {
		return fmt.Errorf("macOS capture init failed (code %d)", ret)
	}
	captureActive = true
	log.Printf("capture: monitor %d via ScreenCaptureKit", idx)
	return nil
}

func captureMonitorOffset() (x, y int) {
	var cx, cy C.int
	C.mac_get_offset(&cx, &cy)
	return int(cx), int(cy)
}

func captureInit() error {
	return captureInitMonitor(0)
}

func captureClose() {
	if captureActive {
		C.mac_capture_close()
		captureActive = false
	}
}

func captureWidth() int {
	var w, h C.int
	C.mac_get_size(&w, &h)
	return int(w)
}

func captureHeight() int {
	var w, h C.int
	C.mac_get_size(&w, &h)
	return int(h)
}

func captureFrame(buf []byte) (width, height int, err error) {
	var w, h C.int
	C.mac_get_size(&w, &h)
	width = int(w)
	height = int(h)

	expected := width * height * 4
	if len(buf) < expected {
		return 0, 0, fmt.Errorf("buffer too small (%d < %d)", len(buf), expected)
	}

	ret := int(C.mac_capture_frame((*C.uchar)(unsafe.Pointer(&buf[0]))))
	if ret == 1 {
		return width, height, fmt.Errorf("no new frame")
	}
	if ret != 0 {
		return width, height, fmt.Errorf("macOS capture failed (code %d)", ret)
	}
	return width, height, nil
}
