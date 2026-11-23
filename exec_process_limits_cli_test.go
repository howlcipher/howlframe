package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Exec process bounds (improvements.md #114, #115): a per-call wall clock
// (--max-exec-millis) and a ceiling on exec children alive at once across
// the run (--max-processes). Both are runner flags a program cannot read or
// widen; both fail with a catchable LIMIT_EXCEEDED at EXEC.

func runSource(t *testing.T, binary, source string, flags ...string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.howl")
	if err := os.WriteFile(src, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, append(append([]string{"run"}, flags...), src)...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		e, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = e.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

func TestExecPerCallTimeout(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	sleep := canonicalProgram(t, "sleep")
	grant := fmt.Sprintf("process:exec=%s 5", sleep)
	caught := fmt.Sprintf(`(cli_app (try_let (r (exec %q "5")) (catch e (print e)) (print "finished")) (print "after"))`, sleep)
	start := time.Now()
	code, out, errOut := runSource(t, binary, caught, "--allow-caps", grant, "--max-exec-millis", "200")
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("child was not stopped at the per-call bound: %s", elapsed)
	}
	if code != 0 || !strings.Contains(out, `"code":"LIMIT_EXCEEDED"`) || !strings.Contains(out, "--max-exec-millis of 200 ms") || !strings.HasSuffix(out, "after\n") || strings.Contains(out, "finished") {
		t.Fatalf("caught timeout: code %d stdout %q stderr %q", code, out, errOut)
	}
	uncaught := fmt.Sprintf(`(cli_app (exec %q "5") (print "after"))`, sleep)
	code, out, errOut = runSource(t, binary, uncaught, "--allow-caps", grant, "--max-exec-millis", "200")
	if code == 0 || strings.Contains(out, "after") || !strings.Contains(errOut, `"opcode":"EXEC"`) || !strings.Contains(errOut, "LIMIT_EXCEEDED") {
		t.Fatalf("uncaught timeout: code %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestExecWithoutPerCallBoundIsUnchanged(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	sh := canonicalProgram(t, "sh")
	// One pipe for both streams: the interleaving is exactly what the child wrote.
	script := "echo a; echo b >&2; echo c"
	source := fmt.Sprintf(`(cli_app (print (bytes_to_string (exec %q "-c" %q))))`, sh, script)
	code, out, errOut := runSource(t, binary, source, "--allow-caps", fmt.Sprintf("process:exec=%s -c **", sh))
	if code != 0 || out != "a\nb\nc\n\n" {
		t.Fatalf("combined output: code %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestExecLiveProcessCeiling(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	sleep := canonicalProgram(t, "sleep")
	echo := canonicalProgram(t, "echo")
	grants := fmt.Sprintf("process,process:exec=%s 1,process:exec=%s hi", sleep, echo)
	// A spawned body holds the only slot while main tries a second child.
	source := fmt.Sprintf(`(cli_app
  (spawn (grants "process:exec=%s 1") (lambda () (exec %q "1")))
  (sleep 300)
  (try_let (r (exec %q "hi")) (catch e (print e)) (print (bytes_to_string r))))`, sleep, sleep, echo)
	code, out, errOut := runSource(t, binary, source, "--allow-caps", grants, "--max-processes", "1")
	if code != 0 || !strings.Contains(out, "LIMIT_EXCEEDED") || !strings.Contains(out, "--max-processes of 1 live children") {
		t.Fatalf("ceiling 1: code %d stdout %q stderr %q", code, out, errOut)
	}
	code, out, errOut = runSource(t, binary, source, "--allow-caps", grants, "--max-processes", "2")
	if code != 0 || strings.TrimSpace(out) != "hi" {
		t.Fatalf("ceiling 2: code %d stdout %q stderr %q", code, out, errOut)
	}
	// Slots are released: sequential children never accumulate.
	sequential := fmt.Sprintf(`(cli_app (exec %q "hi") (exec %q "hi") (print "done"))`, echo, echo)
	if code, out, errOut := runSource(t, binary, sequential, "--allow-caps", grants, "--max-processes", "1"); code != 0 || out != "done\n" {
		t.Fatalf("sequential: code %d stdout %q stderr %q", code, out, errOut)
	}
}

func TestExecProcessFlagsRefuseUnlimited(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	for _, flags := range [][]string{{"--max-processes", "0"}, {"--max-processes", "-3"}, {"--max-exec-millis", "-1"}} {
		code, _, errOut := runSource(t, binary, `(cli_app (print "ran"))`, flags...)
		if code != 2 || !strings.Contains(errOut, strings.TrimPrefix(flags[0], "-")) {
			t.Fatalf("%v: code %d stderr %q", flags, code, errOut)
		}
	}
}
