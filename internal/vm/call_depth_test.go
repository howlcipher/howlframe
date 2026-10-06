package vm

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
)

const countdownFunction = `(defun down (n) (if (> n 0) (return (call down (- n 1))) (return 42)))`

func TestDefaultCallDepth(t *testing.T) {
	if DefaultLimits.MaxCallDepth != 1000 || DefaultExecutionPolicy().Limits.MaxCallDepth != 1000 {
		t.Fatalf("default call depth: limits %d policy %d; want 1000", DefaultLimits.MaxCallDepth, DefaultExecutionPolicy().Limits.MaxCallDepth)
	}
}

func TestCallDepthBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		limit        int
		limited      bool
		output       string
	}{
		{"unbounded default", `(cli_app (defun f () (return (call f))) (call f))`, 1000, true, ""},
		{"exact five frames", fmt.Sprintf(`(cli_app %s (print (call down 4)))`, countdownFunction), 5, false, "42\n"},
		{"six frames", fmt.Sprintf(`(cli_app %s (print (call down 5)))`, countdownFunction), 5, true, ""},
		{"999 frames", fmt.Sprintf(`(cli_app %s (print (call down 998)))`, countdownFunction), 1000, false, "42\n"},
		{"sequential loop calls", fmt.Sprintf(`(cli_app %s (let (i 0) (do (while (< i 100) (do (call down 1) (set i (+ i 1)))) (print i))))`, countdownFunction), 3, false, "100\n"},
		{"zero with call", fmt.Sprintf(`(cli_app %s (call down 0))`, countdownFunction), 0, true, ""},
		{"negative with call", fmt.Sprintf(`(cli_app %s (call down 0))`, countdownFunction), -1, true, ""},
		{"zero without call", `(cli_app (print 42))`, 0, false, "42\n"},
		{"spawn inherits active call", fmt.Sprintf(`(cli_app %s (defun parent () (spawn_agent "child" (task "work" (call down 2)))) (call parent))`, countdownFunction), 3, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, prog := parseAndCompile(t, tc.source)
			policy := DefaultExecutionPolicy()
			policy.Limits.MaxCallDepth = tc.limit
			policy.Limits.MaxInstructions = 10_000_000
			var out, stderr bytes.Buffer
			ev := RunBytecodeWithEvidence(prog, nil, policy, []capability.Capability{capability.Process}, nil, &out, &stderr, 0)
			if tc.limited {
				e := ev.RuntimeFailure
				if e == nil || e.Code != "LIMIT_EXCEEDED" || e.Opcode != "CALL" || !strings.Contains(e.Message, fmt.Sprintf("call depth limit exceeded (max %d)", tc.limit)) {
					t.Fatalf("exit %d failure %#v stderr %s", ev.ExitCode, e, &stderr)
				}
			} else if ev.RuntimeFailure != nil || ev.ExitCode != 0 || out.String() != tc.output {
				t.Fatalf("exit %d failure %#v stdout %q stderr %s; want %q", ev.ExitCode, ev.RuntimeFailure, out.String(), &stderr, tc.output)
			}
		})
	}
}

func TestCallDepthRestoredOnAllExits(t *testing.T) {
	for _, body := range []string{`42`, `(return 42)`, `(call missing)`} {
		t.Run(body, func(t *testing.T) {
			_, prog := parseAndCompile(t, fmt.Sprintf(`(cli_app (defun f () %s) (call f))`, body))
			machine := &BCVM{prog: prog, env: NewBcEnv(nil), Limits: DefaultLimits}
			machine.Limits.MaxCallDepth = 1
			func() {
				defer func() {
					r := recover()
					if body == `(call missing)` {
						if e, ok := r.(*VMError); !ok || e.Code != "RUNTIME_ERROR" {
							t.Fatalf("panic = %#v", r)
						}
					} else if r != nil {
						t.Fatalf("unexpected panic: %v", r)
					}
				}()
				machine.run(prog.Main, machine.env)
			}()
			if machine.callDepth != 0 {
				t.Fatalf("call depth = %d after exit", machine.callDepth)
			}
		})
	}
}
