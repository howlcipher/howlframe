//go:build linux || darwin

package vm

import (
	"os"
	"syscall"
	"unsafe"
)

var terminalSignals = []os.Signal{os.Interrupt, syscall.SIGWINCH, syscall.SIGTERM, syscall.SIGHUP}

func isResizeSignal(sig os.Signal) bool { return sig == syscall.SIGWINCH }

func processSignal(name string) os.Signal {
	if name == "terminate" {
		return syscall.SIGTERM
	}
	return syscall.SIGINT
}

// The terminal driver for terminal_app: raw mode, restore and size. Only the
// runtime touches the terminal; a program never receives a descriptor.

func ioctlTermios(fd uintptr, req uintptr, t *syscall.Termios) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(unsafe.Pointer(t))); errno != 0 {
		return errno
	}
	return nil
}

func isTerminal(fd uintptr) bool {
	var t syscall.Termios
	return ioctlTermios(fd, termGet, &t) == nil
}

// makeRaw puts fd into raw mode (no echo, no line buffering, no signal keys,
// output processing kept so "\n" still returns the carriage) and returns a
// function that restores the previous mode.
func makeRaw(fd uintptr) (func(), error) {
	var old syscall.Termios
	if err := ioctlTermios(fd, termGet, &old); err != nil {
		return nil, err
	}
	raw := old
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctlTermios(fd, termSet, &raw); err != nil {
		return nil, err
	}
	return func() { ioctlTermios(fd, termSet, &old) }, nil
}

type winsize struct{ Row, Col, X, Y uint16 }

func terminalSize(fd uintptr) (cols, rows int) {
	var ws winsize
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&ws))); errno != 0 || ws.Col == 0 {
		return 80, 24
	}
	return int(ws.Col), int(ws.Row)
}
