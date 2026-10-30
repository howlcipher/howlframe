package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The Go backend's test blocks run as go test tests (bugs.md #68): a
// failing assert, an exit or a run-time error fails its own test and the
// rest still run, a test that uses a helper (map_get, /) compiles beside
// server.go, and tests whose names collide still get a function each.
func TestGoTestBlocksRunAsTests(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	repo, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	src := mustWriteFile(t, dir, "g.howl", `(cli_app
  (defun checked ((x int)) (type_hint return "int") (do (assert (= x 3)) (return x)))
  (print "top level")
  (test "passes" (assert (= (call checked 3) 3)))
  (test "assert fails" (assert (= 1 2)))
  (test "assert in a function" (call checked 4))
  (test "map get" (let (d (dict ("a" "b"))) (assert (= (map_get d "a") "b"))))
  (test "divides by zero" (print (/ 1 0)))
  (test "exit 3" (exit 3))
  (test "twin" (assert true))
  (test "twin" (assert true))
  (test "runs last" (assert true)))`)
	out := filepath.Join(dir, "go")
	if b, err := exec.Command(bin, "build", src, "--target", "go", "-o", out).CombinedOutput(); err != nil {
		t.Fatalf("go build: %v %s", err, b)
	}
	mod := "module probe\n\ngo 1.25\n\nrequire github.com/howlcipher/howlframe v0.0.0\n\nreplace github.com/howlcipher/howlframe => " + repo + "\n"
	os.WriteFile(filepath.Join(out, "go.mod"), []byte(mod), 0o600)
	cmd := exec.Command("go", "test", "-v", ".")
	cmd.Dir = out
	cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
	b, err := cmd.CombinedOutput()
	report := string(b)
	if err == nil {
		t.Fatalf("failing tests must fail go test:\n%s", report)
	}
	for _, want := range []string{
		"--- PASS: Test_passes",
		"HowlFrame exit 1 at g.howl:5", "--- FAIL: Test_assert_fails",
		"HowlFrame exit 1 at g.howl:2", "--- FAIL: Test_assert_in_a_function",
		"--- PASS: Test_map_get",
		"RUNTIME_ERROR: division by zero", "--- FAIL: Test_divides_by_zero",
		"HowlFrame exit 3 at g.howl:9", "--- FAIL: Test_exit_3",
		"--- PASS: Test_twin ", "--- PASS: Test_twin_2",
		"--- PASS: Test_runs_last",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report lacks %q:\n%s", want, report)
		}
	}
}
