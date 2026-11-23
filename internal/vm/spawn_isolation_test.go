package vm

import (
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
)

func runSpawnProgram(t *testing.T, source string, maxInstructions int) (string, string, int) {
	t.Helper()
	_, prog := parseAndCompile(t, source)
	// Parent and spawn bodies print concurrently; os.Stdout tolerates that,
	// a bytes.Buffer does not.
	var out, errOut safeBuffer
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = maxInstructions
	policy.Limits.MaxMemoryBytes = 1 << 30
	ev := RunBytecodeWithEvidence(prog, nil, policy, []capability.Capability{capability.Process}, strings.NewReader(""), &out, &errOut, 0)
	code := ev.ExitCode
	if ev.RuntimeFailure != nil {
		errOut.Write([]byte(ev.RuntimeFailure.Message))
		code = 1
	}
	return out.String(), errOut.String(), code
}

// HF-D1: each spawn body started with a fresh instruction count, so five
// bodies could do five budgets of work. The same work done sequentially
// fails, and spread across spawns it must not finish either.
func TestSpawnBodiesShareTheInstructionLimit(t *testing.T) {
	spawned := `(cli_app (do
	  (for i (range 0 5)
	    (spawn (lambda () (let (n 0) (do (while (< n 100) (set n (+ n 1))) (print "child done"))))))
	  (sleep 300)
	  (print "main done")))`
	out, _, _ := runSpawnProgram(t, spawned, 1500)
	if got := strings.Count(out, "child done"); got == 5 {
		t.Fatalf("all five spawn bodies finished under a 1500 instruction limit:\n%s", out)
	}
}

// One spawn body well inside the limit still runs to completion.
func TestSpawnBodyWithinLimitStillRuns(t *testing.T) {
	out, errOut, code := runSpawnProgram(t, `(cli_app (do
	  (spawn (lambda () (print "child done")))
	  (sleep 200)
	  (print "main done")))`, 100000)
	if code != 0 || !strings.Contains(out, "child done") || !strings.Contains(out, "main done") {
		t.Fatalf("exit %d stdout %q stderr %q", code, out, errOut)
	}
}

// HF-D2: a spawn body and its parent writing one captured dict crashed the Go
// runtime with "concurrent map writes". Each now owns a copy.
func TestSpawnBodyGetsItsOwnDictCopy(t *testing.T) {
	out, errOut, code := runSpawnProgram(t, `(cli_app
	  (let (shared (dict ("k" 0)))
	    (do
	      (spawn (lambda ()
	        (let (i 0) (while (< i 20000) (do (map_set shared (to_string i) i) (set i (+ i 1)))))))
	      (let (j 0) (while (< j 20000) (do (map_set shared (str_join (list "m" (to_string j)) "") j) (set j (+ j 1)))))
	      (print "parent keys" (list_len (map_keys shared))))))`, 10000000)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errOut)
	}
	if out != "parent keys 20001\n" {
		t.Fatalf("parent saw child writes or lost its own: %q", out)
	}
}

// Nested dicts inside lists are copied too, and the child still reads the
// values that existed when it was spawned.
func TestSpawnCopyIsDeepAndReadable(t *testing.T) {
	out, errOut, code := runSpawnProgram(t, `(cli_app
	  (let (rows (list (dict ("n" 1))))
	    (do
	      (spawn (lambda ()
	        (let (row (list_get rows 0)) (do (map_set row "n" 99) (print "child sees" (map_get (list_get rows 0) "n"))))))
	      (sleep 200)
	      (print "parent sees" (map_get (list_get rows 0) "n")))))`, 100000)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errOut)
	}
	if !strings.Contains(out, "parent sees 1") {
		t.Fatalf("child mutation reached the parent: %q", out)
	}
}
