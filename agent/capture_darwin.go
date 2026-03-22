//go:build darwin

package main

// TODO: Implement using ScreenCaptureKit (macOS 12.3+) with VideoToolbox H.264.
// For now, return unsupported so the macOS agent registers but streaming is unavailable.

import "fmt"

type MonitorInfo struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func enumerateMonitors() []MonitorInfo                       { return nil }
func captureInitMonitor(idx int) error                       { return fmt.Errorf("not supported on macOS") }
func captureMonitorOffset() (x, y int)                       { return 0, 0 }
func captureInit() error                                     { return fmt.Errorf("screen capture not yet implemented on macOS") }
func captureClose()                                          {}
func captureWidth() int                                      { return 0 }
func captureHeight() int                                     { return 0 }
func captureFrame(buf []byte) (width, height int, err error) { return 0, 0, fmt.Errorf("not supported on macOS") }
