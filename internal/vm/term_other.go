//go:build !linux && !darwin

package vm

import (
	"errors"
	"os"
)

var terminalSignals = []os.Signal{os.Interrupt}

func isResizeSignal(os.Signal) bool { return false }

func processSignal(string) os.Signal { return os.Kill }

// Without a terminal driver for this platform terminal_app runs in line mode.
func isTerminal(fd uintptr) bool               { return false }
func makeRaw(fd uintptr) (func(), error)       { return nil, errors.New("raw mode unsupported") }
func terminalSize(fd uintptr) (cols, rows int) { return 80, 24 }
