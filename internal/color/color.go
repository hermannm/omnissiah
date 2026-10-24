// Package color provides utilities for colored terminal output.
package color

import (
	"os"

	"hermannm.dev/devlog"
)

type Color string

// ANSI color codes (https://en.wikipedia.org/wiki/ANSI_escape_code#Colors).
//
//goland:noinspection GoUnusedGlobalVariable
var (
	Red     = Color("\x1b[31m")
	Green   = Color("\x1b[32m")
	Yellow  = Color("\x1b[33m")
	Blue    = Color("\x1b[34m")
	Magenta = Color("\x1b[35m")
	Cyan    = Color("\x1b[36m")
	Gray    = Color("\x1b[37m")
	Reset   = Color("\x1b[0m")
)

// On applies the given color on the given string, if stdout supports ANSI colors.
func (c Color) On(s string) string {
	if !supportsColor() {
		return s
	}
	return string(c) + s + string(Reset)
}

// Caches result of [devlog.IsColorTerminal].
var stdoutSupportsColor *bool = nil

func supportsColor() bool {
	if stdoutSupportsColor == nil {
		stdoutSupportsColor = new(devlog.IsColorTerminal(os.Stdout))
	}
	return *stdoutSupportsColor
}
