//go:build !windows && !darwin

package main

import "fmt"

func captureInit() error  { return fmt.Errorf("screen capture not supported on this platform") }
func captureClose()        {}
func captureWidth() int    { return 0 }
func captureHeight() int   { return 0 }
func captureFrame(buf []byte) (width, height int, err error) {
	return 0, 0, fmt.Errorf("screen capture not supported on this platform")
}
