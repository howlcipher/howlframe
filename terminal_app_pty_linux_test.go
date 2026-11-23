//go:build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// openPTY returns the master and slave ends of a new pseudo-terminal.
func openPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pty: %v", err)
	}
	var unlock int32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), syscall.TIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); e != 0 {
		t.Skipf("unlockpt: %v", e)
	}
	var n uint32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), syscall.TIOCGPTN, uintptr(unsafe.Pointer(&n))); e != 0 {
		t.Skipf("ptsname: %v", e)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("open slave: %v", err)
	}
	return master, slave
}

func termiosOf(t *testing.T, f *os.File) syscall.Termios {
	t.Helper()
	var tio syscall.Termios
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), syscall.TCGETS, uintptr(unsafe.Pointer(&tio))); e != 0 {
		t.Fatalf("TCGETS: %v", e)
	}
	return tio
}

// In a terminal the runtime takes raw mode, edits the line itself, turns
// Ctrl-C into an interrupt event and restores the terminal on exit.
func TestTerminalAppRawModeInAPTY(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	master, slave := openPTY(t)
	defer master.Close()
	defer slave.Close()
	before := termiosOf(t, slave)

	app := `(cli_app
  (defun on_event (state event)
    (type_hints (state dict) (event dict) (return dict))
    (let (effects (list))
      (do
        (when (and (= (map_get event "type") "line") (= (map_get event "text") "bye"))
          (append effects (dict ("kind" "quit") ("code" 5))))
        (return (dict ("state" state) ("view" (list (str_join (list "EV " (encode_json event)) ""))) ("prompt" "raw> ") ("effects" effects))))))
  (exit (terminal_app "on_event" (dict))))`
	src := filepath.Join(t.TempDir(), "raw.howl")
	if err := os.WriteFile(src, []byte(app), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "run", src)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var out strings.Builder
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			mu.Lock()
			out.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	wait := func(fragment string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			mu.Lock()
			got := out.String()
			mu.Unlock()
			if strings.Contains(got, fragment) {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("no %q in %q", fragment, out.String())
	}
	wait(`"tty":true`)
	master.Write([]byte("ab\x7fc\r"))
	wait(`"text":"ac"`)
	master.Write([]byte("x\x15y\x1b[Az\r")) // Ctrl-U clears, an arrow key is dropped
	wait(`"text":"yz"`)
	master.Write([]byte{0x03})
	wait(`"type":"interrupt"`)
	master.Write([]byte("bye\r"))
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 5 {
			t.Fatalf("exit = %v", err)
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatal("raw terminal_app did not exit")
	}
	after := termiosOf(t, slave)
	if before.Lflag != after.Lflag || before.Iflag != after.Iflag || before.Cflag != after.Cflag {
		t.Fatalf("terminal not restored: before %+v after %+v", before, after)
	}
}
