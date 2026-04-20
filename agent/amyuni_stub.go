//go:build !windows

package main

func amyuniEnsureInstalled(configDir string) error { return nil }
func amyuniEnableMonitor() error                   { return nil }
func amyuniDisableMonitor() error                  { return nil }
func amyuniDevicePresent() bool                    { return false }
