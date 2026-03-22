//go:build !windows

package main

func audioInit() error    { return nil }
func audioCapture() []byte { return nil }
func audioClose()         {}
func audioSampleRate() int { return 48000 }
