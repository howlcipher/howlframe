//go:build linux

package vm

import "syscall"

const (
	termGet = syscall.TCGETS
	termSet = syscall.TCSETS
)
