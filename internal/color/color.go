// Package color provides utilities for colored terminal output.
package color

import (
	"io"
	"os"
	"strings"

	"hermannm.dev/devlog"
)

type Color struct {
	string string
}

// ANSI color codes (https://en.wikipedia.org/wiki/ANSI_escape_code#Colors).
//
//goland:noinspection GoUnusedGlobalVariable
var (
	Red     = Color{"\x1b[31m"}
	Green   = Color{"\x1b[32m"}
	Yellow  = Color{"\x1b[33m"}
	Blue    = Color{"\x1b[34m"}
	Magenta = Color{"\x1b[35m"}
	Cyan    = Color{"\x1b[36m"}
	Gray    = Color{"\x1b[37m"}
	reset   = Color{"\x1b[0m"}
)

// On applies the given color on the given string, if stdout supports ANSI colors.
func (c Color) On(s string) string {
	if !supportsColor() {
		return s
	}
	return c.string + s + reset.string
}

func (c Color) Write(writer io.Writer) error {
	if !supportsColor() {
		return nil
	}
	_, err := writer.Write([]byte(c.string))
	return err
}

func ResetWriter(writer io.Writer) error {
	return reset.Write(writer)
}

func (c Color) Set(builder *strings.Builder) {
	if !supportsColor() {
		return
	}
	builder.WriteString(c.string)
}

func Reset(builder *strings.Builder) {
	reset.Set(builder)
}

// Caches result of [devlog.IsColorTerminal].
var stdoutSupportsColor *bool = nil

func supportsColor() bool {
	if stdoutSupportsColor == nil {
		stdoutSupportsColor = new(devlog.IsColorTerminal(os.Stdout))
	}
	return *stdoutSupportsColor
}
