package vm

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
)

// A child that runs and exits non-zero must not lose its exit status or its
// output: a catch binding receives them (Milestone 1, taskflow R6/R7), while
// the plain error text, the receipt and an uncaught failure stay unchanged.
func TestExecNonZeroExitExposesStatusAndOutputToCatch(t *testing.T) {
	sh, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		t.Skip("no /bin/sh")
	}
	caps := []capability.Capability{capability.Capability("process:exec=" + sh + " -c *")}
	run := func(script string) (string, string, int) {
		src := fmt.Sprintf(`(cli_app (try_let (r (exec %q "-c" %q)) (catch e (print e)) (print "ok")))`, sh, script)
		_, prog := parseAndCompile(t, src)
		var out, errOut bytes.Buffer
		code := RunBytecode(prog, nil, caps, nil, &out, &errOut)
		return out.String(), errOut.String(), code
	}

	out, _, code := run("printf 'partial\\nline'; printf ' err' >&2; exit 3")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{`"exit_code":3`, `"output":"partial\nline err"`, `"code":"IO_ERROR"`, `exec failed: exit status 3`} {
		if !strings.Contains(out, want) {
			t.Fatalf("catch value %q lacks %s", out, want)
		}
	}

	// Success is unchanged and a signal/other failure has no exit_code field.
	out, _, _ = run("kill -9 $$")
	if strings.Contains(out, "exit_code") {
		t.Fatalf("signal death must not report an exit code: %q", out)
	}
	out, _, _ = run("exit 0")
	if strings.TrimSpace(out) != "ok" {
		t.Fatalf("success path changed: %q", out)
	}

	// Uncaught: the recorded failure carries the message only, never the child output.
	_, prog := parseAndCompile(t, fmt.Sprintf(`(cli_app (exec %q "-c" "printf SECRET-OUT; exit 4"))`, sh))
	var out2, err2 bytes.Buffer
	ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), caps, nil, &out2, &err2, 0)
	if ev.RuntimeFailure == nil || !strings.Contains(ev.RuntimeFailure.Message, "exit status 4") {
		t.Fatalf("uncaught exec failure not recorded: %#v", ev.RuntimeFailure)
	}
	if strings.Contains(ev.RuntimeFailure.Message, "SECRET-OUT") || strings.Contains(err2.String(), "SECRET-OUT") {
		t.Fatalf("uncaught failure leaked child output: %q %q", ev.RuntimeFailure.Message, err2.String())
	}
}

// exec with exactly one trailing list argument uses the list as the argument
// vector; the allow-list is evaluated against the final vector.
func TestExecListIsArgumentVector(t *testing.T) {
	sh, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		t.Skip("no /bin/sh")
	}
	src := fmt.Sprintf(`(cli_app (let (argv (list "-c" "printf '%%s|%%s' \"$0\" \"$1\"" "a b" "")) (print (bytes_to_string (exec %q argv)))))`, sh)
	run := func(caps ...capability.Capability) (string, int) {
		node, prog := parseAndCompile(t, src)
		var out, errOut bytes.Buffer
		ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), caps, nil, &out, &errOut, 0)
		var out2, err2 bytes.Buffer
		if c := Interpret(node, nil, caps, nil, &out2, &err2); c != 0 {
			t.Fatalf("interpreter exit %d: %s", c, err2.String())
		} else if out2.String() != out.String() {
			t.Fatalf("interpreter %q != bytecode %q", out2.String(), out.String())
		}
		return out.String(), ev.ExitCode
	}
	if out, code := run(capability.Capability("process:exec=" + sh + " **")); code != 0 || out != "a b|\n" {
		t.Fatalf("list argv: exit %d out %q", code, out)
	}
	// Denial applies to the spread vector: this grant allows exactly 3 arguments.
	_, prog := parseAndCompile(t, src)
	var out, errOut bytes.Buffer
	ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), []capability.Capability{capability.Capability("process:exec=" + sh + " -c *")}, nil, &out, &errOut, 0)
	if ev.RuntimeFailure == nil || ev.RuntimeFailure.Code != "CAPABILITY_DENIED" {
		t.Fatalf("a 4-element vector must not match a 2-element pattern: %#v", ev.RuntimeFailure)
	}
}

// (exit n) inside a try_let value must end the program, not be caught and
// turned into an error branch (found by the Milestone 2 review).
func TestExitIsNotCatchableByTryLet(t *testing.T) {
	_, prog := parseAndCompile(t, `(cli_app (try_let (x (do (print "before") (exit 7) 1)) (catch e (print "caught")) (print "after")))`)
	var out, errOut bytes.Buffer
	code := RunBytecode(prog, nil, nil, nil, &out, &errOut)
	if code != 7 || out.String() != "before\n" {
		t.Fatalf("exit 7 inside try_let: code %d stdout %q (a catch must not see it)", code, out.String())
	}
}
