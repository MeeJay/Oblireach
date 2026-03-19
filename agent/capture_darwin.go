//go:build darwin

package main

// TODO: Implement using ScreenCaptureKit (macOS 12.3+) with VideoToolbox H.264.
// For now, return unsupported so the macOS agent registers but streaming is unavailable.

import "fmt"

func captureInit() error  { return fmt.Errorf("screen capture not yet implemented on macOS (coming soon)") }
func captureClose()        {}
func captureWidth() int    { return 0 }
func captureHeight() int   { return 0 }
func captureFrame(buf []byte) (width, height int, err error) {
	return 0, 0, fmt.Errorf("screen capture not yet implemented on macOS")
}
