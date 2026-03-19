//go:build darwin

package main

// TODO: Implement using VideoToolbox H.264 hardware encoder.

import "fmt"

func encoderInit(width, height, fps, bitrate int) (extradata []byte, err error) {
	return nil, fmt.Errorf("H.264 encoder not yet implemented on macOS (coming soon)")
}
func encodeFrame(bgra []byte, width, height int, pts int64) ([]byte, error) {
	return nil, fmt.Errorf("H.264 encoder not yet implemented on macOS")
}
func encoderClose() {}
