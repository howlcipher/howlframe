package vm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

func assertRuntimeResult(t *testing.T, source string, caps []capability.Capability, stdin, wantOut, wantErr string, wantExit int, useBytecode bool) string {
	t.Helper()
	node, prog := parseAndCompile(t, source)
	var out, errOut bytes.Buffer
	var exitCode int
	if useBytecode {
		exitCode = RunBytecode(prog, nil, caps, strings.NewReader(stdin), &out, &errOut)
	} else {
		exitCode = Interpret(node, nil, caps, strings.NewReader(stdin), &out, &errOut)
	}
	if exitCode != wantExit {
		t.Fatalf("exit = %d, want %d; stdout=%q stderr=%q", exitCode, wantExit, out.String(), errOut.String())
	}
	if out.String() != wantOut {
		t.Fatalf("stdout = %q, want %q; stderr=%q", out.String(), wantOut, errOut.String())
	}
	if wantErr != "" && !strings.Contains(errOut.String(), wantErr) {
		t.Fatalf("stderr = %q, want %q", errOut.String(), wantErr)
	} else if wantExit != 0 && wantErr == "" && errOut.String() != "" {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
	return out.String()
}

func assertInterpretStdout(t *testing.T, source, want string) string {
	t.Helper()
	return assertRuntimeResult(t, source, nil, "", want, "", 0, false)
}

func assertBytecodeStdout(t *testing.T, source, want string) string {
	t.Helper()
	return assertRuntimeResult(t, source, nil, "", want, "", 0, true)
}

func assertInterpretErrorContains(t *testing.T, source, wantCode, wantMsg string) string {
	t.Helper()
	node, _ := parseAndCompile(t, source)
	var out, errOut bytes.Buffer
	exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut)
	if exitCode == 0 || (wantCode != "" && !strings.Contains(errOut.String(), wantCode)) || !strings.Contains(errOut.String(), wantMsg) {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, out.String(), errOut.String())
	}
	return out.String()
}

type vmTypeErrCase struct {
	name, source, want string
}

func runVMTypeErrorCases(t *testing.T, cases []vmTypeErrCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertParityTypeErrors(t, tc.source, tc.want)
		})
	}
}

func assertParityTypeErrors(t *testing.T, source, want string) {
	t.Helper()
	t.Run("interpreter", func(t *testing.T) {
		if out := assertInterpretErrorContains(t, source, "TYPE_ERROR", want); strings.TrimSpace(out) != "" {
			t.Fatalf("stdout = %q, want empty", out)
		}
	})
	t.Run("bytecode", func(t *testing.T) {
		if out := assertBytecodeTypeErrorContains(t, source, "TYPE_ERROR", want); strings.TrimSpace(out) != "" {
			t.Fatalf("stdout = %q, want empty", out)
		}
	})
}

func evalBytecodeWithEnv(t *testing.T, source string) (bytecode.ExecutionEvidence, string) {
	t.Helper()
	_, prog := parseAndCompile(t, source)
	var out, errOut bytes.Buffer
	ev := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errOut, 0)
	return ev, out.String()
}

func assertBytecodeTypeErrorContains(t *testing.T, source, wantCode, wantMsg string) string {
	t.Helper()
	evidence, out := evalBytecodeWithEnv(t, source)
	if evidence.RuntimeFailure == nil || evidence.RuntimeFailure.Code != wantCode {
		t.Fatalf("failure = %#v, want %s", evidence.RuntimeFailure, wantCode)
	}
	if !strings.Contains(evidence.RuntimeFailure.Message, wantMsg) {
		t.Fatalf("message = %q, want %q", evidence.RuntimeFailure.Message, wantMsg)
	}
	return out
}

func assertBytecodeEvidenceStdout(t *testing.T, source, want string) {
	t.Helper()
	evidence, out := evalBytecodeWithEnv(t, source)
	if evidence.RuntimeFailure != nil || evidence.ExitCode != 0 {
		t.Fatalf("exit = %d, failure = %+v, stdout = %q", evidence.ExitCode, evidence.RuntimeFailure, out)
	}
	if out != want {
		t.Fatalf("stdout = %q, want %q", out, want)
	}
}

func assertInterpretWithCaps(t *testing.T, node *ast.Node, caps []capability.Capability) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	exitCode := Interpret(node, nil, caps, strings.NewReader(""), &stdout, &stderr)
	return exitCode, stdout.String(), stderr.String()
}

func runVMTestCase(t *testing.T, prog *bytecode.BCProgram, caps []capability.Capability) bytecode.ExecutionEvidence {
	t.Helper()
	var out, errOut bytes.Buffer
	return RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), caps, strings.NewReader(""), &out, &errOut, 0)
}

func newHTTPDispatchVM(prog *bytecode.BCProgram, errOut *bytes.Buffer) (*BCVM, int) {
	env := NewBcEnv(nil)
	vm := &BCVM{
		prog:        prog,
		env:         env,
		insts:       prog.Main,
		stores:      newBCStoreRegistry(),
		Limits:      DefaultLimits,
		AllowedCaps: []capability.Capability{capability.Network},
		Out:         &bytes.Buffer{},
		ErrOut:      errOut,
	}
	serve := len(prog.Main)
	for index, inst := range prog.Main {
		if inst.Op == bytecode.OpHttpServerServe {
			serve = index
			break
		}
	}
	return vm, serve
}
