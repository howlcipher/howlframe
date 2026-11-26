//go:build linux

package main

import (
	"encoding/json"
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

// ptySession runs a terminal_app on a pseudo-terminal of 80 by 24, as an
// operator would.
type ptySession struct {
	t      *testing.T
	master *os.File
	slave  *os.File
	cmd    *exec.Cmd
	before syscall.Termios
	mu     sync.Mutex
	out    strings.Builder
	pos    int
}

func startPTY(t *testing.T, binary, app string, env []string, flags ...string) *ptySession {
	t.Helper()
	master, slave := openPTY(t)
	t.Cleanup(func() { master.Close(); slave.Close() })
	ws := struct{ Row, Col, X, Y uint16 }{24, 80, 0, 0}
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, slave.Fd(), syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&ws))); e != 0 {
		t.Fatalf("TIOCSWINSZ: %v", e)
	}
	src := filepath.Join(t.TempDir(), "app.howl")
	if err := os.WriteFile(src, []byte(app), 0o600); err != nil {
		t.Fatal(err)
	}
	s := &ptySession{t: t, master: master, slave: slave, before: termiosOf(t, slave)}
	s.cmd = exec.Command(binary, append(append([]string{"run"}, flags...), src)...)
	s.cmd.Env = env
	s.cmd.Stdin, s.cmd.Stdout, s.cmd.Stderr = slave, slave, slave
	s.cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := s.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if s.cmd.ProcessState == nil {
			s.cmd.Process.Kill()
		}
	})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			s.mu.Lock()
			s.out.Write(buf[:n])
			s.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return s
}

// ptyEnv is this process's environment with TERM set and NO_COLOR set to
// noColor, or left out when noColor is empty.
func ptyEnv(term, noColor string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TERM=") && !strings.HasPrefix(kv, "NO_COLOR=") {
			env = append(env, kv)
		}
	}
	env = append(env, "TERM="+term)
	if noColor != "" {
		env = append(env, "NO_COLOR="+noColor)
	}
	return env
}

func (s *ptySession) send(keys string) {
	s.t.Helper()
	if _, err := s.master.Write([]byte(keys)); err != nil {
		s.t.Fatalf("send %q: %v", keys, err)
	}
}

func (s *ptySession) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.String()
}

// expect waits for fragment after the previous match and returns the output
// from there up to and including fragment, so each step sees only its own
// frames.
func (s *ptySession) expect(fragment string) string {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		got := s.output()
		if i := strings.Index(got[s.pos:], fragment); i >= 0 {
			seen := got[s.pos : s.pos+i+len(fragment)]
			s.pos += i + len(fragment)
			return seen
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("no %q after offset %d in %q", fragment, s.pos, s.output())
	return ""
}

// finish waits for the exit code and checks the terminal mode was restored.
func (s *ptySession) finish(code int) {
	s.t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()
	select {
	case err := <-done:
		got := 0
		if e, ok := err.(*exec.ExitError); ok {
			got = e.ExitCode()
		} else if err != nil {
			s.t.Fatal(err)
		}
		if got != code {
			s.t.Fatalf("exit %d, want %d; output %q", got, code, s.output())
		}
	case <-time.After(10 * time.Second):
		s.cmd.Process.Kill()
		s.t.Fatalf("terminal_app did not exit; output %q", s.output())
	}
	after := termiosOf(s.t, s.slave)
	if s.before.Lflag != after.Lflag || s.before.Iflag != after.Iflag || s.before.Cflag != after.Cflag {
		s.t.Fatalf("terminal not restored: before %+v after %+v", s.before, after)
	}
}

// ptyEchoApp shows each event as JSON and quits with code 5 on "bye".
const ptyEchoApp = `(cli_app
  (defun on_event (state event)
    (type_hints (state dict) (event dict) (return dict))
    (let (effects (list))
      (do
        (when (and (= (map_get event "type") "line") (= (map_get event "text") "bye"))
          (append effects (dict ("kind" "quit") ("code" 5))))
        (return (dict ("state" state) ("view" (list (str_join (list "EV " (encode_json event)) ""))) ("prompt" "raw> ") ("effects" effects))))))
  (exit (terminal_app "on_event" (dict))))`

// In a terminal the runtime takes raw mode, edits the line itself, turns
// Ctrl-C into an interrupt event and restores the terminal on exit.
func TestTerminalAppRawModeInAPTY(t *testing.T) {
	s := startPTY(t, buildHowlFrameBinaryForTest(t), ptyEchoApp, ptyEnv("xterm", "1"))
	s.expect(`"tty":true`)
	s.send("ab\x7fc\r")
	s.expect(`"text":"ac"`)
	s.send("x\x15y\x1b[2~z\r") // Ctrl-U clears, an unbound key (Insert) is dropped
	s.expect(`"text":"yz"`)
	s.send("\x03")
	s.expect(`"type":"interrupt"`)
	s.send("bye\r")
	s.finish(5)
}

func TestTerminalAppLineEditingInAPTY(t *testing.T) {
	s := startPTY(t, buildHowlFrameBinaryForTest(t), ptyEchoApp, ptyEnv("xterm", "1"))
	s.expect(`"tty":true`)
	s.send("abc\x1b[D\x1b[D")
	s.expect("raw> abc\r\x1b[6C") // the cursor is drawn where the editor has it
	steps := []struct{ keys, text string }{
		{"X\x1b[FY\r", "aXbcY"},                             // insert mid-line, End
		{"world\x01hello \x05!\r", "hello world!"},          // Ctrl-A, Ctrl-E
		{"fix the bug\x17test\r", "fix the test"},           // Ctrl-W
		{"abcdef\x02\x02\x02\x0b\x02\x1b[3~\x06Z\r", "abZ"}, // Ctrl-B, Ctrl-K, Delete, Ctrl-F
		{"xy\x01\x04\r", "y"},                               // Ctrl-D deletes on a non-empty line
		{"\x1b[A\x1b[A\x1b[A\x1b[B\r", "abZ"},               // Up and Down walk the history
		{"draft\x10\x0e\r", "draft"},                        // Ctrl-P, Ctrl-N: the draft comes back
	}
	for _, step := range steps {
		s.send(step.keys)
		s.expect(`"pasted":false,"text":"` + step.text + `","type":"line"`)
	}
	s.send("bye\r")
	s.finish(5)
}

func TestTerminalAppBracketedPasteInAPTY(t *testing.T) {
	s := startPTY(t, buildHowlFrameBinaryForTest(t), ptyEchoApp, ptyEnv("xterm", "1"))
	if seen := s.expect(`"tty":true`); !strings.Contains(seen, "\x1b[?2004h") {
		t.Fatalf("bracketed paste not enabled: %q", seen)
	}
	// A paste never submits: its line breaks become spaces, and only Enter
	// sends the line.
	s.send("run \x1b[200~one\rtwo\nthree\x1b[201~")
	s.expect("raw> run one two three")
	s.send("\r")
	s.expect(`"pasted":true,"text":"run one two three"`)
	if n := strings.Count(s.output(), `"type":"line"`); n != 1 {
		t.Fatalf("the paste submitted on its own: %d line events in %q", n, s.output())
	}
	s.send("y\r")
	s.expect(`"pasted":false,"text":"y"`)
	s.send("\x1b[A\x1b[A\r") // a pasted line recalled from history keeps its mark
	s.expect(`"pasted":true,"text":"run one two three"`)
	s.send("bye\r")
	s.expect("\x1b[?2004l")
	s.finish(5)
}

func TestTerminalAppColourInAPTY(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	cases := []struct {
		name   string
		env    []string
		flags  []string
		colour bool
	}{
		{"auto on a terminal", ptyEnv("xterm", ""), nil, true},
		{"NO_COLOR", ptyEnv("xterm", "1"), nil, false},
		{"a dumb terminal", ptyEnv("dumb", ""), nil, false},
		{"--color never", ptyEnv("xterm", ""), []string{"--color", "never"}, false},
		{"--color always over NO_COLOR", ptyEnv("xterm", "1"), []string{"--color", "always"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := startPTY(t, binary, terminalStyledApp, c.env, c.flags...)
			s.expect("\x1b[?1049l")
			s.finish(0)
			out := s.output()
			red, green := strings.Contains(out, "\x1b[31mFAILED\x1b[0m"), strings.Contains(out, "\x1b[32mfine\x1b[0m")
			if red != c.colour || green != c.colour || !strings.Contains(out, "FAILED") {
				t.Fatalf("want colour %v: %q", c.colour, out)
			}
			// Styled text is still sanitized, and a style outside the palette
			// is plain.
			if strings.Contains(out, "\x1b[5m") || strings.Contains(out, "\x1b[7m") || !strings.Contains(out, "a?[5mb") {
				t.Fatalf("styled text not sanitized: %q", out)
			}
		})
	}
}

// A view taller than the terminal shows its newest lines, PageUp and
// PageDown move through it with indicators, a new frame keeps a scrolled
// view in place, and Enter returns to the bottom.
func TestTerminalAppScrollsTallViewsInAPTY(t *testing.T) {
	rows := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("row %02d", i+1)
		}
		return out
	}
	raw, _ := json.Marshal(map[string]any{"view": rows(60), "longer": rows(65)})
	app := `(cli_app
  (defun on_event (state event)
    (type_hints (state dict) (event dict) (return dict))
    (let (kind (map_get event "type"))
      (let (effects (list))
        (do
          (when (= kind "interrupt") (map_set state "view" (map_get state "longer")))
          (when (and (= kind "line") (= (map_get event "text") "bye"))
            (append effects (dict ("kind" "quit") ("code" 0))))
          (return (dict ("state" state) ("view" (map_get state "view")) ("prompt" "> ") ("effects" effects)))))))
  (let (raw ` + howlString(string(raw)) + `) (exit (terminal_app "on_event" (parse_json Any raw)))))`
	s := startPTY(t, buildHowlFrameBinaryForTest(t), app, ptyEnv("xterm", "1"))
	frame := func(seen string) string { return seen[strings.LastIndex(seen, "\x1b[2J"):] }
	check := func(seen string, want []string, absent []string) {
		t.Helper()
		f := frame(seen)
		for _, w := range want {
			if !strings.Contains(f, w) {
				t.Fatalf("no %q in frame %q", w, f)
			}
		}
		for _, a := range absent {
			if strings.Contains(f, a) {
				t.Fatalf("%q in frame %q", a, f)
			}
		}
	}

	check(s.expect("row 60"), []string{"-- 38 more lines above: PageUp --", "row 39"}, []string{"row 38", "more lines below"})
	s.send("\x1b[5~")
	check(s.expect("-- 21 more lines below: PageDown --"), []string{"-- 18 more lines above: PageUp --", "row 19", "row 39"}, []string{"row 18", "row 40"})
	s.send("\x03") // the view grows by five lines while scrolled
	check(s.expect("-- 26 more lines below: PageDown --"), []string{"-- 18 more lines above: PageUp --", "row 19", "row 39"}, []string{"row 18", "row 40"})
	s.send("\x1b[6~")
	check(s.expect("-- 5 more lines below: PageDown --"), []string{"-- 39 more lines above: PageUp --", "row 40", "row 60"}, []string{"row 39", "row 61"})
	s.send("\x1b[6~")
	check(s.expect("row 65"), []string{"-- 43 more lines above: PageUp --", "row 44"}, []string{"row 43", "more lines below"})
	s.send("\x1b[5~\x1b[5~\x1b[5~\x1b[5~")
	check(s.expect("-- 43 more lines below: PageDown --"), []string{"row 01", "row 22"}, []string{"row 23", "more lines above"})
	s.send("\r")
	check(s.expect("row 65"), []string{"-- 43 more lines above: PageUp --"}, []string{"more lines below"})
	s.send("bye\r")
	s.finish(0)
}
