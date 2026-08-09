package vm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
)

func runInstructionBudget(limit int) (runtimeErr *VMError, output string) {
	limits := DefaultLimits
	limits.MaxInstructions = limit

	var stdout bytes.Buffer
	machine := &BCVM{
		env:    NewBcEnv(nil),
		stores: newBCStoreRegistry(),
		Limits: limits,
		Out:    &stdout,
	}
	instructions := []bytecode.BCInstruction{
		{OpString: "LOAD_CONST", Op: bytecode.OpLoadConst, ValueOperand: "ok"},
		{OpString: "PRINT", Op: bytecode.OpPrint, IntOperand: 1},
	}

	defer func() {
		output = stdout.String()
		if recovered := recover(); recovered != nil {
			if vmErr, ok := recovered.(*VMError); ok {
				runtimeErr = vmErr
				return
			}
			panic(recovered)
		}
	}()
	machine.run(instructions, machine.env)
	return nil, stdout.String()
}

func TestInstructionLimitBoundary(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		wantLimited bool
		wantOutput  string
	}{
		{name: "insufficient", limit: 1, wantLimited: true},
		{name: "exact", limit: 2, wantOutput: "ok\n"},
		{name: "greater", limit: 3, wantOutput: "ok\n"},
		{name: "zero permits no instructions", limit: 0, wantLimited: true},
		{name: "negative permits no instructions", limit: -1, wantLimited: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runtimeErr, output := runInstructionBudget(test.limit)
			if test.wantLimited {
				if runtimeErr == nil {
					t.Fatalf("limit %d executed successfully, output %q", test.limit, output)
				}
				if runtimeErr.Code != "LIMIT_EXCEEDED" {
					t.Fatalf("error code = %q, want LIMIT_EXCEEDED", runtimeErr.Code)
				}
				if !strings.Contains(runtimeErr.Error(), `"phase":"runtime"`) ||
					!strings.Contains(runtimeErr.Error(), `"code":"LIMIT_EXCEEDED"`) {
					t.Fatalf("limit error is not structured: %s", runtimeErr)
				}
			} else if runtimeErr != nil {
				t.Fatalf("limit %d failed: %v", test.limit, runtimeErr)
			}
			if output != test.wantOutput {
				t.Fatalf("output = %q, want %q", output, test.wantOutput)
			}
		})
	}
}

func TestDefaultExecutionPolicyPreservesVMLimits(t *testing.T) {
	policy := DefaultExecutionPolicy()
	if policy.Limits != DefaultLimits {
		t.Fatalf("default policy limits = %+v, want %+v", policy.Limits, DefaultLimits)
	}
	if policy.Limits.MaxInstructions != 100000 {
		t.Fatalf("default instruction limit = %d, want 100000", policy.Limits.MaxInstructions)
	}

	var zeroPolicy ExecutionPolicy
	if zeroPolicy.Limits.MaxInstructions != 0 {
		t.Fatalf("zero policy instruction limit = %d, want fail-closed zero", zeroPolicy.Limits.MaxInstructions)
	}
}

func TestRunBytecodeWithPolicyUsesExplicitInstructionBudget(t *testing.T) {
	program := &bytecode.BCProgram{
		Version: 1,
		Main: []bytecode.BCInstruction{
			{OpString: "LOAD_CONST", Op: bytecode.OpLoadConst, ValueOperand: "ok"},
			{OpString: "PRINT", Op: bytecode.OpPrint, IntOperand: 1},
		},
	}
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = len(program.Main)

	var stdout, stderr bytes.Buffer
	exitCode := RunBytecodeWithPolicy(program, nil, policy, nil, nil, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", exitCode, stderr.String())
	}
	if stdout.String() != "ok\n" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "ok\n")
	}
}
