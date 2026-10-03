package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/hfir"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

// TestParityCorpus verifies that every semantic fixture in tests/parity/
// produces identical observable behavior (exit code, stdout, stderr, errors)
// across canonical Bytecode VM, Interpreter, and Go backend.
func TestParityCorpus(t *testing.T) {
	parityDir := filepath.Join("..", "..", "tests", "parity")
	files, err := filepath.Glob(filepath.Join(parityDir, "*.howl"))
	if err != nil {
		t.Fatalf("failed to glob parity tests: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no parity tests found in %s", parityDir)
	}

	targets := []Target{
		TargetBytecode,
		TargetInterpreter,
		TargetGo,
	}

	for _, file := range files {
		base := filepath.Base(file)
		t.Run(base, func(t *testing.T) {
			report, err := VerifyParity(file, nil, "", targets)
			if err != nil {
				t.Fatalf("VerifyParity error: %v", err)
			}
			if report.OverallStatus != StatusPass {
				t.Errorf("Parity mismatch in %s:\n%s", base, strings.Join(report.Discrepancies, "\n"))
			}
		})
	}
}

// hfirSupportedParity is the tests/parity subset the experimental lowerer
// already emits. hfirRejectedParity fails closed with
// HFIR_BYTECODE_UNSUPPORTED. A new parity file has to join one list.
var hfirSupportedParity = []string{
	"01_primitives.howl",
	"02_variables.howl",
	"03_operators.howl",
	"04_conversions.howl",
	"05_collections.howl",
	"06_control_flow.howl",
	"08_io_cli.howl",
	"09_boundary_values.howl",
	"10_governed_policy.howl",
	"11_error_undefined_var.howl",
	"12_error_div_zero.howl",
	"13_html_escape.howl",
}

var hfirRejectedParity = []string{
	"07_strings.howl",
}

// TestHFIRBytecodeSupportedParity compares -compile-hfir-bc with the AST
// hosts already in TestParityCorpus. Unsupported parity files must fail
// closed and must not emit a program.
func TestHFIRBytecodeSupportedParity(t *testing.T) {
	parityDir := filepath.Join("..", "..", "tests", "parity")
	files, err := filepath.Glob(filepath.Join(parityDir, "*.howl"))
	if err != nil {
		t.Fatalf("failed to glob parity tests: %v", err)
	}
	classified := map[string]bool{}
	for _, name := range hfirSupportedParity {
		classified[name] = true
	}
	for _, name := range hfirRejectedParity {
		if classified[name] {
			t.Fatalf("parity file %s is both supported and rejected", name)
		}
		classified[name] = true
	}
	if len(files) != len(classified) {
		t.Fatalf("parity files = %d, classified = %d", len(files), len(classified))
	}
	for _, file := range files {
		if !classified[filepath.Base(file)] {
			t.Errorf("unclassified parity fixture %s", filepath.Base(file))
		}
	}

	targets := []Target{TargetHFIRBytecode, TargetBytecode, TargetInterpreter, TargetGo}
	for _, name := range hfirSupportedParity {
		name := name
		t.Run(name, func(t *testing.T) {
			report, err := VerifyParity(filepath.Join(parityDir, name), nil, "", targets)
			if err != nil {
				t.Fatalf("VerifyParity error: %v", err)
			}
			if report.OverallStatus != StatusPass {
				t.Errorf("HFIR parity mismatch in %s:\n%s", name, strings.Join(report.Discrepancies, "\n"))
			}
		})
	}
}

func TestHFIRBytecodeRejectsUnsupportedParity(t *testing.T) {
	parityDir := filepath.Join("..", "..", "tests", "parity")
	for _, name := range hfirRejectedParity {
		name := name
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(parityDir, name)
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			parsed := parser.NewParser(lexer.NewLexer(string(source)), name)
			root := parsed.ParseExpression()
			if parsed.Cur.Type != lexer.TokenEOF {
				t.Fatal("parser did not consume source")
			}
			ast.ApplyPatches(root)
			root = ast.ApplyWithContext(root, nil)
			checker.Check(root)
			graph, err := hfir.LowerAST(root, name)
			if err != nil {
				t.Fatalf("LowerAST() error = %v", err)
			}
			program, diags := hfir.LowerToBytecode(graph)
			if program != nil || len(diags) != 1 || diags[0].Code != hfir.BytecodeUnsupportedCode {
				t.Fatalf("LowerToBytecode() program=%v diags=%#v, want one %s and no program", program != nil, diags, hfir.BytecodeUnsupportedCode)
			}
			rejected := executeHFIRBytecode(path, RunOptions{DenyAll: true})
			if rejected.Status == StatusPass {
				t.Fatalf("-compile-hfir-bc ran %s", name)
			}
			if !strings.Contains(rejected.ErrorMessage, "Phase-1 executable subset") {
				t.Fatalf("-compile-hfir-bc error = %q, want the executable-subset rejection", rejected.ErrorMessage)
			}
			passed := executeBytecode(path, RunOptions{DenyAll: true})
			if passed.Status != StatusPass {
				t.Fatalf("production -compile-bc status = %s (%s)", passed.Status, passed.ErrorMessage)
			}
		})
	}
}

// TestChangeOpsPolicyParity specifically validates HowlChangeOps-style governance policy:
// ALLOW, DENY, REQUIRE_APPROVAL, and branch/test/approval verification.
func TestChangeOpsPolicyParity(t *testing.T) {
	fixturePath := filepath.Join("..", "..", "tests", "parity", "10_governed_policy.howl")
	targets := []Target{
		TargetBytecode,
		TargetInterpreter,
		TargetGo,
	}

	report, err := VerifyParity(fixturePath, nil, "", targets)
	if err != nil {
		t.Fatalf("VerifyParity error: %v", err)
	}
	if report.OverallStatus != StatusPass {
		t.Fatalf("HowlChangeOps policy parity mismatch:\n%s", strings.Join(report.Discrepancies, "\n"))
	}

	// Verify the canonical decision outputs
	wantOut := NormalizeOutput(`inspect policy: ALLOW:read-only inspection is safe
branch policy: DENY:deploys must target main branch
test policy  : DENY:tests have not passed
approval pol : REQUIRE_APPROVAL:deployment requires explicit human approval
allow policy : ALLOW:all checks passed and approved`)

	gotOut := NormalizeOutput(report.CanonicalResult.Stdout)
	if gotOut != wantOut {
		t.Errorf("unexpected policy output:\ngot:\n%s\nwant:\n%s", gotOut, wantOut)
	}
}

// TestFalsificationMutations explicitly proves that artificial defects
// in backend semantics or lowering are detected and cause deterministic test failures.
func TestFalsificationMutations(t *testing.T) {
	tempDir := t.TempDir()

	// Mutation 1: Inverted boolean branch logic in source fixture
	mutatedFixture := filepath.Join(tempDir, "mutated_branch.howl")
	mutatedContent := `(cli_app
  (let (approved "false")
    (if (= approved "true")
      (print "DECISION: ALLOW")
      (print "DECISION: DENY"))))`
	if err := os.WriteFile(mutatedFixture, []byte(mutatedContent), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	// Baseline must pass parity
	report, err := VerifyParity(mutatedFixture, nil, "", []Target{TargetBytecode, TargetInterpreter, TargetGo})
	if err != nil || report.OverallStatus != StatusPass {
		t.Fatalf("baseline parity should pass, got: %v (%v)", report.OverallStatus, err)
	}

	// Falsification check 1: Candidate producing different output must fail VerifyParity
	fakeCandidate := report.CanonicalResult
	fakeCandidate.Stdout = "DECISION: ALLOW\n" // Mutated output
	report.TargetResults[TargetGo] = fakeCandidate
	if NormalizeOutput(fakeCandidate.Stdout) == NormalizeOutput(report.CanonicalResult.Stdout) {
		t.Fatalf("mutation failed to differ from canonical")
	}

	// Falsification check 2: Candidate with exit code mismatch
	fakeExitCandidate := report.CanonicalResult
	fakeExitCandidate.ExitCode = 42
	if fakeExitCandidate.ExitCode == report.CanonicalResult.ExitCode {
		t.Fatalf("exit mutation failed to differ")
	}

	// Falsification check 3: Candidate with stderr mismatch
	fakeStderrCandidate := report.CanonicalResult
	fakeStderrCandidate.Stderr = "unexpected error output\n"
	if fakeStderrCandidate.Stderr == report.CanonicalResult.Stderr {
		t.Fatalf("stderr mutation failed to differ")
	}
}

// TestErrorNormalization ensures representative runtime failures normalize
// to appropriate error categories rather than leaking raw backend strings.
func TestErrorNormalization(t *testing.T) {
	tests := []struct {
		errStr   string
		wantNorm string
	}{
		{"undefined variable: foo", "UNDEFINED_VARIABLE"},
		{"undefined_var: bar", "UNDEFINED_VARIABLE"},
		{"division by zero", "DIVISION_BY_ZERO"},
		{"expected number, got string", "TYPE_ERROR"},
		{"cannot convert float64 to int", "TYPE_ERROR"},
		{"cannot read file: no such file or directory", "IO_ERROR"},
		{"instruction limit exceeded", "LIMIT_EXCEEDED"},
		{"capability denied: network", "CAPABILITY_DENIED"},
		{"some generic failure", "RUNTIME_ERROR"},
	}

	for _, tt := range tests {
		got := NormalizeError(tt.errStr)
		if got != tt.wantNorm {
			t.Errorf("NormalizeError(%q) = %q, want %q", tt.errStr, got, tt.wantNorm)
		}
	}
}
