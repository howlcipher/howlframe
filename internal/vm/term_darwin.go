//go:build darwin

package vm

import "syscall"

const (
	termGet = syscall.TIOCGETA
	termSet = syscall.TIOCSETA
)
