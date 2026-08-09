package vm

import (
	"bytes"
	"strings"
	"testing"

	"zero/internal/bytecode"
	"zero/internal/capability"
	"zero/internal/checker"
)

func runBytecodeForTryLetTest(
	prog *bytecode.BCProgram,
	allowedCaps []capability.Capability,
) (stdout string, panicValue any) {
	var out bytes.Buffer
	machine := &BCVM{
		prog:        prog,
		env:         NewBcEnv(nil),
		insts:       prog.Main,
		stores:      newBCStoreRegistry(),
		AllowedCaps: allowedCaps,
		Limits:      DefaultLimits,
		In:          strings.NewReader(""),
		Out:         &out,
	}

	defer func() {
		stdout = out.String()
		panicValue = recover()
	}()

	machine.run(machine.insts, machine.env)
	return out.String(), nil
}

func TestBytecodeTryLetResumesAfterEmbeddedInstructionsInFor(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		allowedCaps []capability.Capability
		wantStdout  string
	}{
		{
			name: "success branch",
			code: `(cli_app
  (let (items (list "one" "two"))
    (do
      (for item items
        (try_let (value item)
          (catch err (print "unexpected"))
          (print value)))
      (print "done"))))`,
			wantStdout: "one\ntwo\ndone\n",
		},
		{
			name: "catch branch",
			code: `(cli_app
  (let (items (list "one" "two"))
    (do
      (for item items
        (try_let (value (read_file ""))
          (catch err (print "caught" item))
          (print value)))
      (print "done"))))`,
			allowedCaps: []capability.Capability{capability.Filesystem},
			wantStdout:  "caught one\ncaught two\ndone\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, prog := parseAndCompile(t, tt.code)
			checker.Check(node)
			stdout, panicValue := runBytecodeForTryLetTest(prog, tt.allowedCaps)
			if panicValue != nil {
				t.Fatalf("bytecode execution panicked: %v; stdout = %q", panicValue, stdout)
			}
			if stdout != tt.wantStdout {
				t.Fatalf("stdout = %q, want %q", stdout, tt.wantStdout)
			}
		})
	}
}
