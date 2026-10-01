package vm

import (
	"bytes"
	"strings"
	"testing"
)

// The VM keeps for-loop state and call operands on one shared operand stack.
// A callee that returns from inside a loop, a discarded value, or a failed
// try_let expression must not leave operands behind for the caller or the
// enclosing loop to consume. Each case prints the interpreter's answer.
func TestOperandStackDiscipline(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "return inside for, called inside for",
			src: `(cli_app
  (defun contains ((xs list) (want string)) bool
    (do (for x xs (if (= x want) (return true))) (return false)))
  (for o (list "a" "b" "c") (print o (call contains (list "b") o))))`,
			want: "a false\nb true\nc false\n",
		},
		{
			name: "discarded call value in for body",
			src: `(cli_app
  (defun side ((s string)) string (return s))
  (for o (list "a" "b" "c") (do (call side o) (print o))))`,
			want: "a\nb\nc\n",
		},
		{
			name: "data-dependent discarded value in for body",
			src: `(cli_app
  (defun side ((s string)) string (return s))
  (for o (list "a" "b" "c" "d") (do (if (= o "b") (call side o)) (print o))))`,
			want: "a\nb\nc\nd\n",
		},
		{
			name: "callee discard does not corrupt caller arithmetic",
			src: `(cli_app
  (defun two () float (do 5.0 (return 2.0)))
  (print "sum" (+ 1.0 (call two))))`,
			want: "sum 3\n",
		},
		{
			name: "denylist membership after return inside loop",
			src: `(cli_app
  (defun contains ((xs list) (want string)) bool
    (do (for x xs (if (= x want) (return true))) (return false)))
  (if (!= true (call contains (list "mallory" "eve") "mallory"))
    (print "ALLOW")
    (print "DENY")))`,
			want: "DENY\n",
		},
		{
			name: "loop accumulation over leaking callee",
			src: `(cli_app
  (defun f ((x float)) float (if (> x 0.0) (do (+ x 1.0) (return x)) (return 0.0)))
  (let (total 0.0)
    (do (for v (list 1.0 2.0 3.0) (set total (+ total (call f v)))) (print "total" total))))`,
			want: "total 6\n",
		},
		{
			name: "nested loops",
			src: `(cli_app
  (for a (list "x" "y") (for b (list 1.0 2.0) (do "discard" (print a b)))))`,
			want: "x 1\nx 2\ny 1\ny 2\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, prog := parseAndCompile(t, c.src)
			var out, errOut bytes.Buffer
			evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errOut, 0)
			if evidence.RuntimeFailure != nil || evidence.ExitCode != 0 {
				t.Fatalf("exit = %d, failure = %+v, stdout = %q", evidence.ExitCode, evidence.RuntimeFailure, out.String())
			}
			if out.String() != c.want {
				t.Fatalf("stdout = %q, want %q", out.String(), c.want)
			}
		})
	}
}

func TestTryLetFailureDropsPartialOperands(t *testing.T) {
	const src = `(cli_app
  (defun risky ((p string)) string (return (str_join (list "x" (bytes_to_string (read_file p))) "")))
  (for o (list "a" "b" "c")
    (try_let (v (call risky o))
      (catch err (print "caught" o))
      (print "ok" o v))))`
	_, prog := parseAndCompile(t, src)
	var out, errOut bytes.Buffer
	evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errOut, 0)
	if evidence.RuntimeFailure != nil || evidence.ExitCode != 0 {
		t.Fatalf("exit = %d, failure = %+v, stdout = %q", evidence.ExitCode, evidence.RuntimeFailure, out.String())
	}
	if want := "caught a\ncaught b\ncaught c\n"; out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}
}
