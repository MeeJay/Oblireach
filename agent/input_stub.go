//go:build !windows && !darwin

package main

func inputMouseMove(x, y int)              {}
func inputMouseButton(button int, down bool, x, y int) {}
func inputMouseScroll(delta int)            {}
func inputKey(vk int, down bool)            {}
