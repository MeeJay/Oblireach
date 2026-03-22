//go:build !windows

package main

var audioInitDone bool

func audioInit() error     { return nil }
func audioCapture() []byte { return nil }
func audioClose()          {}
func audioSampleRate() int { return 48000 }
