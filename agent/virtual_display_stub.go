//go:build !windows

package main

// vddEnsureInstalled is a no-op on non-Windows platforms. Virtual Display
// Driver integration is Windows-only; other agents capture via their native
// platform APIs (ScreenCaptureKit on macOS, X11 on Linux).
func vddEnsureInstalled(configDir string) error { return nil }
func vddDevicePresent() bool                    { return false }
func vddEnsureDevicePresent(infPath string) error { return nil }
