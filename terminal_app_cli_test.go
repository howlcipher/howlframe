package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// terminal_app (improvements.md #117) in line mode, the mode used whenever
// stdin is not a terminal. The test handler echoes every non-line event as
// one JSON view line and treats a typed line as a JSON list of effects to
// request, so each test drives the closed effect table directly.
const terminalEchoApp = `(cli_app
  (defun on_event (state event)
    (type_hints (state dict) (event dict) (return dict))
    (let (kind (map_get event "type"))
      (let (effects (list))
        (let (seen (+ (map_get state "seen") 1))
          (do
            (when (= kind "line")
              (if (= (map_get event "text") "spin")
                (while true (set seen (+ seen 1)))
                (if (= (map_get event "text") "big")
                  (map_set state "blob" (str_join (list "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx") ""))
                  (set effects (parse_json Any (map_get event "text"))))))
            (map_set state "seen" seen)
            (return (dict ("state" state)
                          ("view" (list (if (= kind "line") (str_join (list "line " (to_string seen)) "") (encode_json event))))
                          ("prompt" "> ")
                          ("effects" effects))))))))
  (exit (terminal_app "on_event" (dict ("seen" 0)))))`

type termSession struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	mu     sync.Mutex
	output strings.Builder
	done   chan error
}

func startTerminalApp(t *testing.T, source string, flags ...string) *termSession {
	t.Helper()
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "app.howl")
	if err := os.WriteFile(src, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, append(append([]string{"run"}, flags...), src)...)
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	s := &termSession{t: t, cmd: cmd, stdin: stdin, done: make(chan error, 1)}
	cmd.Stderr = &lockedWriter{s: s}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		r := bufio.NewReader(stdout)
		buf := make([]byte, 4096)
		for {
			n, err := r.Read(buf)
			s.mu.Lock()
			s.output.Write(buf[:n])
			s.mu.Unlock()
			if err != nil {
				break
			}
		}
		s.done <- cmd.Wait()
	}()
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
		}
	})
	return s
}

type lockedWriter struct{ s *termSession }

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	return w.s.output.Write(p)
}

func (s *termSession) send(line string) {
	s.t.Helper()
	if _, err := io.WriteString(s.stdin, line+"\n"); err != nil {
		s.t.Fatalf("send %q: %v", line, err)
	}
}

func (s *termSession) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.output.String()
}

// waitFor returns the first view line containing every fragment.
func (s *termSession) waitFor(fragments ...string) string {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, line := range strings.Split(s.text(), "\n") {
			match := true
			for _, f := range fragments {
				match = match && strings.Contains(line, f)
			}
			if match {
				return line
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("no line with %q in:\n%s", fragments, s.text())
	return ""
}

func (s *termSession) exit() int {
	s.t.Helper()
	select {
	case err := <-s.done:
		if err == nil {
			return 0
		}
		if e, ok := err.(*exec.ExitError); ok {
			return e.ExitCode()
		}
		s.t.Fatal(err)
	case <-time.After(15 * time.Second):
		s.t.Fatalf("terminal_app did not exit:\n%s", s.text())
	}
	return -1
}

func effects(e ...string) string { return "[" + strings.Join(e, ",") + "]" }

func event(t *testing.T, line string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "> ")), &doc); err != nil {
		t.Fatalf("not an event: %q: %v", line, err)
	}
	return doc
}

func TestTerminalAppLineModeLoop(t *testing.T) {
	s := startTerminalApp(t, terminalEchoApp)
	start := event(t, s.waitFor(`"type":"start"`))
	if start["tty"] != false {
		t.Fatalf("piped stdin must be line mode: %v", start)
	}
	s.send("[]")
	s.waitFor("line 2")
	s.send(effects(`{"kind":"quit","code":7}`))
	if code := s.exit(); code != 7 {
		t.Fatalf("quit code = %d\n%s", code, s.text())
	}
}

func TestTerminalAppEveryEventHasAFreshBudget(t *testing.T) {
	// 300 events at well over 50 instructions each would exhaust any
	// cumulative 2000 instruction budget; per-event budgets never do.
	s := startTerminalApp(t, terminalEchoApp, "--max-instructions", "2000", "--max-event-instructions", "2000")
	s.waitFor(`"type":"start"`)
	for i := 0; i < 300; i++ {
		s.send("[]")
	}
	s.waitFor("line 301")
	// An endless handler fails alone: the previous state is kept.
	s.send("spin")
	failed := event(t, s.waitFor(`"type":"handler_failed"`))
	errObj, _ := failed["error"].(map[string]any)
	if errObj["code"] != "LIMIT_EXCEEDED" || errObj["schema"] != "howlframe.runtime_error/v1" {
		t.Fatalf("handler_failed = %v", failed)
	}
	s.send("[]")
	s.waitFor("line 303") // seen kept counting from the state before "spin"
	s.send(effects(`{"kind":"quit","code":0}`))
	if code := s.exit(); code != 0 {
		t.Fatalf("exit %d\n%s", code, s.text())
	}
}

func TestTerminalAppStateIsBounded(t *testing.T) {
	s := startTerminalApp(t, terminalEchoApp, "--max-state-bytes", "64")
	s.waitFor(`"type":"start"`)
	s.send("big")
	failed := event(t, s.waitFor(`"type":"handler_failed"`))
	if errObj, _ := failed["error"].(map[string]any); errObj["code"] != "LIMIT_EXCEEDED" || !strings.Contains(fmt.Sprint(errObj["message"]), "--max-state-bytes") {
		t.Fatalf("state cap: %v", failed)
	}
	s.send(effects(`{"kind":"quit","code":0}`))
	s.exit()
}

func TestTerminalAppProcessEffects(t *testing.T) {
	sh := canonicalProgram(t, "sh")
	sleep := canonicalProgram(t, "sleep")
	grants := fmt.Sprintf("process:exec=%s -c **,process:exec=%s 30", sh, sleep)
	s := startTerminalApp(t, terminalEchoApp, "--allow-caps", grants)
	s.waitFor(`"type":"start"`)

	s.send(effects(fmt.Sprintf(`{"kind":"exec_start","id":"p1","argv":[%q,"-c","echo out; echo err >&2; exit 4"]}`, sh)))
	exit := event(t, s.waitFor(`"type":"process_exit"`, `"id":"p1"`))
	if exit["exit_code"] != float64(4) || exit["stdout"] != "out\n" || exit["stderr"] != "err\n" {
		t.Fatalf("process_exit = %v", exit)
	}

	// exec_start is authorized like exec: no grant, no child.
	s.send(effects(`{"kind":"exec_start","id":"p2","argv":["/bin/echo","nope"]}`))
	denied := event(t, s.waitFor(`"type":"effect_failed"`, `"id":"p2"`))
	if errObj, _ := denied["error"].(map[string]any); errObj["code"] != "CAPABILITY_DENIED" {
		t.Fatalf("denied exec_start = %v", denied)
	}

	// interrupt reaches the child; the event reports it ended by a signal.
	s.send(effects(fmt.Sprintf(`{"kind":"exec_start","id":"p3","argv":[%q,"30"]}`, sleep)))
	time.Sleep(200 * time.Millisecond)
	started := time.Now()
	s.send(effects(`{"kind":"exec_signal","id":"p3","signal":"interrupt"}`))
	stopped := event(t, s.waitFor(`"type":"process_exit"`, `"id":"p3"`))
	if time.Since(started) > 5*time.Second || stopped["exit_code"] != float64(-1) {
		t.Fatalf("interrupted child = %v after %s", stopped, time.Since(started))
	}

	// Quitting with a live child ends it promptly.
	s.send(effects(fmt.Sprintf(`{"kind":"exec_start","id":"p4","argv":[%q,"30"]}`, sleep)))
	time.Sleep(200 * time.Millisecond)
	started = time.Now()
	s.send(effects(`{"kind":"quit","code":0}`))
	if code := s.exit(); code != 0 || time.Since(started) > 8*time.Second {
		t.Fatalf("quit with a live child: exit %d after %s", code, time.Since(started))
	}
}

func TestTerminalAppTimerTailAndLog(t *testing.T) {
	dir := t.TempDir()
	progress := filepath.Join(dir, "progress.jsonl")
	logFile := filepath.Join(dir, "logs", "events.jsonl")
	if err := os.MkdirAll(filepath.Dir(logFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(progress, []byte("{\"seq\":1}\n{\"seq\":2}\n{\"seq\":3"), 0o600); err != nil {
		t.Fatal(err)
	}
	grants := "filesystem:read=" + dir + ",filesystem:write=" + filepath.Dir(logFile)
	s := startTerminalApp(t, terminalEchoApp, "--allow-caps", grants)
	s.waitFor(`"type":"start"`)

	s.send(effects(`{"kind":"timer","id":"tick","ms":50}`))
	s.waitFor(`"type":"timer"`, `"id":"tick"`)

	s.send(effects(fmt.Sprintf(`{"kind":"tail","id":"t1","path":%q,"offset":0}`, progress)))
	tail := event(t, s.waitFor(`"type":"tail_lines"`, `"id":"t1"`))
	lines, _ := tail["lines"].([]any)
	if len(lines) != 2 || lines[1] != `{"seq":2}` || tail["offset"] != float64(20) {
		t.Fatalf("tail stops at the last whole line: %v", tail)
	}
	s.send(effects(fmt.Sprintf(`{"kind":"tail","id":"t2","path":%q,"offset":0}`, filepath.Join(dir, "absent.jsonl"))))
	if missing := event(t, s.waitFor(`"type":"tail_lines"`, `"id":"t2"`)); missing["missing"] != true {
		t.Fatalf("missing file: %v", missing)
	}
	s.send(effects(`{"kind":"tail","id":"t3","path":"/etc/hostname","offset":0}`))
	if denied := event(t, s.waitFor(`"type":"effect_failed"`, `"id":"t3"`)); fmt.Sprint(denied["error"]) == "" || !strings.Contains(fmt.Sprint(denied["error"]), "CAPABILITY_DENIED") {
		t.Fatalf("ungranted tail: %v", denied)
	}

	s.send(effects(fmt.Sprintf(`{"kind":"append_log","path":%q,"line":"one"}`, logFile), fmt.Sprintf(`{"kind":"append_log","path":%q,"line":"two\nlines"}`, logFile)))
	s.send(effects(`{"kind":"nope"}`))
	s.waitFor(`"type":"effect_failed"`, `UNKNOWN_EFFECT`)
	s.send(effects(`{"kind":"quit","code":0}`))
	s.exit()
	data, err := os.ReadFile(logFile)
	if err != nil || string(data) != "one\ntwo lines\n" {
		t.Fatalf("log = %q, %v", data, err)
	}
	if info, _ := os.Stat(logFile); info.Mode().Perm() != 0o600 {
		t.Fatalf("log mode = %v", info.Mode().Perm())
	}
}

func TestTerminalAppViewCannotInjectTerminalSequences(t *testing.T) {
	app := `(cli_app
  (defun on_event (state event)
    (type_hints (state dict) (event dict) (return dict))
    (return (dict ("state" state) ("view" (list (str_join (list "a" (map_get state "esc") "[2Jb") ""))) ("prompt" "> ")
                  ("effects" (list (dict ("kind" "quit") ("code" 0)))))))
  (let (raw "{\"esc\":\"\\u001b\"}") (exit (terminal_app "on_event" (parse_json Any raw)))))`
	s := startTerminalApp(t, app)
	s.exit()
	if out := s.text(); strings.Contains(out, "\x1b") || !strings.Contains(out, "a?[2Jb") {
		t.Fatalf("view not sanitized: %q", out)
	}
}

func TestTerminalAppHandlerMustBeATwoParameterDefun(t *testing.T) {
	app := `(cli_app (defun on_event (state) (return state)) (exit (terminal_app "on_event" (dict))))`
	s := startTerminalApp(t, app)
	if code := s.exit(); code == 0 || !strings.Contains(s.text(), "two parameters") {
		t.Fatalf("exit %d: %s", code, s.text())
	}
}

func TestTerminalAppRunsOncePerProgram(t *testing.T) {
	app := `(cli_app
  (defun on_event (state event)
    (type_hints (state dict) (event dict) (return dict))
    (return (dict ("state" state) ("view" (list "v")) ("prompt" "> ") ("effects" (list (dict ("kind" "quit") ("code" 0)))))))
  (terminal_app "on_event" (dict))
  (try_let (again (terminal_app "on_event" (dict))) (catch e (print e)) (print "second ran")))`
	s := startTerminalApp(t, app)
	if code := s.exit(); code != 0 || !strings.Contains(s.text(), "only once per program") || strings.Contains(s.text(), "second ran") {
		t.Fatalf("exit %d: %s", code, s.text())
	}
}

func TestTerminalAppEventFlagsRefuseUnlimited(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	for _, flag := range []string{"--max-event-instructions", "--max-event-memory-bytes", "--max-state-bytes"} {
		code, _, errOut := runSource(t, binary, `(cli_app (print "ran"))`, flag, "0")
		if code != 2 || !strings.Contains(errOut, strings.TrimPrefix(flag, "-")) {
			t.Fatalf("%s 0: code %d stderr %q", flag, code, errOut)
		}
	}
}
