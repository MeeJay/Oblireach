//go:build darwin

package main

// TODO: Implement using CGEventPost (keyboard) and CGEventCreateMouseEvent (mouse).

func inputMouseMove(x, y int)                          {}
func inputMouseButton(button int, down bool, x, y int) {}
func inputMouseScroll(delta int)                       {}
func inputKey(vk int, down bool)                       {}
