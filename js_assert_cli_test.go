package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A web_app test block must be able to fail. assert lowers to stderr and exit,
// which the JavaScript backend did not emit, so a test could only print.
func TestJSTestBlockAssertFailsTheTest(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	src := mustWriteFile(t, dir, "a.howl", `(web_app
  (defun two ()
    (type_hint return "int")
    (return 2))
  (test "passes" (assert (= (call two) 2)))
  (test "fails" (assert (= (call two) 3)))
  (test "runs after a failure" (assert (= (call two) 2))))`)
	out := filepath.Join(dir, "js")
	if b, err := exec.Command(bin, "build", src, "--target", "js", "-o", out).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	test, err := os.ReadFile(filepath.Join(out, "app.test.js"))
	if err != nil {
		t.Fatal(err)
	}
	// One declaration only: a redeclared function is a SyntaxError wherever
	// the file is evaluated as a module or in strict mode.
	if n := strings.Count(string(test), "function howlFrameExit("); n != 1 {
		t.Fatalf("app.test.js declares howlFrameExit %d times", n)
	}
	cmd := exec.Command("node", "--test", "--test-reporter=tap", "app.test.js")
	cmd.Dir = out
	b, err := cmd.CombinedOutput()
	report := string(b)
	if err == nil {
		t.Fatalf("a failing assert must fail node --test:\n%s", report)
	}
	for _, want := range []string{"ok 1 - passes", "not ok 2 - fails", "ok 3 - runs after a failure", "assertion failed"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report lacks %q:\n%s", want, report)
		}
	}
}

// A try_let does not catch exit, so an assert that fails inside a try_let
// binding still fails its test (bugs.md #67): on the VM exit ends the
// program whatever handler encloses it, and app.test.js throws it instead.
func TestJSTestExitPassesTryLet(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	src := mustWriteFile(t, dir, "a.howl", `(web_app
  (defun checked ((x int))
    (type_hint return "int")
    (do (assert (= x 1)) (return x)))
  (test "assert in a binding" (try_let (v (call checked 2)) (catch e (print "handler ran")) (print "body ran" v)))
  (test "assert in a value" (print (try_let (v (call checked 2)) (catch e "handler value") v)))
  (test "errors are still caught" (try_let (v (to_int "x")) (catch e (print "caught conversion")) (print v))))`)
	out := filepath.Join(dir, "js")
	if b, err := exec.Command(bin, "build", src, "--target", "js", "-o", out).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	cmd := exec.Command("node", "--test", "--test-reporter=tap", "app.test.js")
	cmd.Dir = out
	b, err := cmd.CombinedOutput()
	report := string(b)
	if err == nil {
		t.Fatalf("an assert failing inside try_let must fail node --test:\n%s", report)
	}
	for _, want := range []string{"not ok 1 - assert in a binding", "not ok 2 - assert in a value", "ok 3 - errors are still caught", "caught conversion"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report lacks %q:\n%s", want, report)
		}
	}
	for _, unwanted := range []string{"handler ran", "body ran", "handler value"} {
		if strings.Contains(report, unwanted) {
			t.Fatalf("a try_let handled an exit (%q):\n%s", unwanted, report)
		}
	}
}

func TestJSStderrAndExitInNode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	src := mustWriteFile(t, dir, "e.howl", `(web_app (do (print "before") (stderr "bad input") (exit 3) (print "after")))`)
	out := filepath.Join(dir, "js")
	if b, err := exec.Command(bin, "build", src, "--target", "js", "-o", out).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	cmd := exec.Command("node", filepath.Join(out, "app.js"))
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 3 {
		t.Fatalf("want exit status 3, got %v", err)
	}
	if stdout.String() != "before\n" || stderr.String() != "bad input" {
		t.Fatalf("stdout %q stderr %q", stdout.String(), stderr.String())
	}
}
