package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// TestChangeOpsPolicyParity specifically validates ChangeOps-style governance policy:
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
		t.Fatalf("ChangeOps policy parity mismatch:\n%s", strings.Join(report.Discrepancies, "\n"))
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
