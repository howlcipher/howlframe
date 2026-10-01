package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

type stabilizationFixture struct {
	Stdout  string            `json:"stdout"`
	Exit    int               `json:"exit"`
	Targets map[Target]string `json:"targets"`
	Notes   map[Target]string `json:"notes"`
}

type stabilizationManifest struct {
	Fixtures map[string]stabilizationFixture `json:"fixtures"`
	// Reject maps a program under reject/ to a substring of the checker
	// diagnostic that must stop it before any backend runs.
	Reject map[string]string `json:"reject"`
}

// stabilizationOutcome names how one target behaved relative to the canonical
// bytecode VM. Unsupported and rejected targets keep their own labels so they
// can never be reported as parity.
func stabilizationOutcome(canonical, candidate ExecutionResult) string {
	switch {
	case candidate.Status == StatusBackendUnsupported && strings.Contains(candidate.ErrorMessage, "not available"):
		return "NOT_RUN"
	case candidate.Status == StatusBackendUnsupported:
		return "UNSUPPORTED"
	case candidate.Status == StatusCompileFailure:
		return "COMPILE_FAILURE"
	case candidate.Status == StatusRuntimeFailure:
		return "RUNTIME_FAILURE"
	case candidate.Status == StatusPass &&
		NormalizeOutput(candidate.Stdout) == NormalizeOutput(canonical.Stdout) &&
		candidate.ExitCode == canonical.ExitCode:
		return "PASS"
	}
	return "MISMATCH"
}

// TestStabilizationParityCorpus runs the stabilization corpus across the
// production VM (canonical), HFIR bytecode, interpreter, generated Go, and
// generated JavaScript, and compares every outcome to tests/parity_stabilization
// /manifest.json.
func TestStabilizationParityCorpus(t *testing.T) {
	dir := filepath.Join("..", "..", "tests", "parity_stabilization")
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest stabilizationManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.howl"))
	if len(files) != len(manifest.Fixtures) {
		t.Fatalf("fixtures on disk = %d, in manifest = %d", len(files), len(manifest.Fixtures))
	}
	targets := []Target{TargetBytecode, TargetHFIRBytecode, TargetInterpreter, TargetGo, TargetJavaScript}
	var matrix []string
	for _, file := range files {
		name := filepath.Base(file)
		want, ok := manifest.Fixtures[name]
		if !ok {
			t.Errorf("fixture %s missing from manifest", name)
			continue
		}
		t.Run(name, func(t *testing.T) {
			report, err := VerifyParityWithOptions(file, targets, RunOptions{JSRoot: "web_app"})
			if err != nil {
				t.Fatal(err)
			}
			canon := report.CanonicalResult
			if canon.ExitCode != want.Exit || NormalizeOutput(canon.Stdout) != NormalizeOutput(want.Stdout) {
				t.Fatalf("canonical VM: exit=%d stdout=%q, want exit=%d stdout=%q", canon.ExitCode, canon.Stdout, want.Exit, want.Stdout)
			}
			row := []string{name, "vm=PASS"}
			for _, target := range []Target{TargetHFIRBytecode, TargetInterpreter, TargetGo, TargetJavaScript} {
				got := stabilizationOutcome(canon, report.TargetResults[target])
				row = append(row, fmt.Sprintf("%s=%s", target, got))
				if got != want.Targets[target] {
					t.Errorf("%s: got %s, want %s (%s)", target, got, want.Targets[target], report.TargetResults[target].ErrorMessage)
				}
				// A target that ran must never produce an allow-shaped
				// divergence from the canonical decision.
				if got == "MISMATCH" {
					t.Errorf("%s silently diverged:\n%s", target, strings.Join(report.Discrepancies, "\n"))
				}
			}
			matrix = append(matrix, strings.Join(row, " "))
		})
	}
	sort.Strings(matrix)
	t.Log("\n" + strings.Join(matrix, "\n"))
}

// TestStabilizationRejectCorpus keeps the fail-early guarantees: programs the
// language cannot run correctly are rejected by the checker, not by a VM stack
// underflow or a late type error.
func TestStabilizationRejectCorpus(t *testing.T) {
	dir := filepath.Join("..", "..", "tests", "parity_stabilization")
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest stabilizationManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "reject", "*.howl"))
	if len(files) != len(manifest.Reject) {
		t.Fatalf("reject fixtures on disk = %d, in manifest = %d", len(files), len(manifest.Reject))
	}
	for name, want := range manifest.Reject {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(dir, "reject", name))
			if err != nil {
				t.Fatal(err)
			}
			root := parser.NewParser(lexer.NewLexer(string(source)), name).ParseExpression()
			var reasons []string
			for _, d := range checker.Analyze(root).Diagnostics {
				reasons = append(reasons, d.Reason)
			}
			if !strings.Contains(strings.Join(reasons, "\n"), want) {
				t.Fatalf("diagnostics %q do not contain %q", reasons, want)
			}
		})
	}
}
