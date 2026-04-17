//go:build !windows && !darwin && !linux

package main

func setInputMonitorOffset(x, y int)                  {}
func inputMouseMove(x, y int)                          {}
func inputMouseButton(button int, down bool, x, y int) {}
func inputMouseScroll(delta int)                        {}
func inputKey(vk int, down bool)                        {}
func inputSAS()                                          {}
func inputVKFromKey(key string) (int, int)               { return 0, 0 }
func clipboardGet() string                               { return "" }
func clipboardSet(text string)                           {}
func inputBlock(block bool)                             {}
func inputUnblock()                                     {}
func inputSwitchActiveDesktop()                         {}
