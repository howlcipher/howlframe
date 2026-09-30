package main

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/hfir"
)

type conformanceFile struct {
	ABI   string            `json:"abi"`
	Cases []conformanceCase `json:"cases"`
}

type conformanceCase struct {
	Name           string   `json:"name"`
	Fixture        string   `json:"fixture"`
	AllowCaps      *string  `json:"allow_caps"`
	DenyAll        bool     `json:"deny_all"`
	Targets        []Target `json:"targets"`
	JavaScriptRoot string   `json:"javascript_root"`
	Expect         string   `json:"expect"`
	Stdout         string   `json:"stdout"`
	StdoutKeys     []string `json:"stdout_keys"`
	Forbid         string   `json:"forbid"`
}

func loadConformance(t *testing.T) (string, conformanceFile) {
	t.Helper()
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	path := filepath.Join(root, "tests", "conformance", "lowered_hfir_abi_v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read conformance manifest: %v", err)
	}
	var file conformanceFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("parse conformance manifest: %v", err)
	}
	return root, file
}

func (c conformanceCase) runOptions() RunOptions {
	return RunOptions{
		AllowCaps: c.AllowCaps,
		DenyAll:   c.DenyAll,
		JSRoot:    c.JavaScriptRoot,
	}
}

func (c conformanceCase) wantStdout() string {
	if len(c.StdoutKeys) == 0 {
		return c.Stdout
	}
	keys := append([]string(nil), c.StdoutKeys...)
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// TestLoweredHFIRABIConformance runs the v1 suite through the existing
// differential harness. Passing cases must share stdout. Rejections must
// share one error class and must not print a forbidden value.
func TestLoweredHFIRABIConformance(t *testing.T) {
	t.Setenv("HOWLFRAME_ABI_SECRET", "phase1-token")
	root, file := loadConformance(t)
	if file.ABI != hfir.LoweredABIV1 {
		t.Fatalf("manifest abi = %q, want %q", file.ABI, hfir.LoweredABIV1)
	}
	if len(file.Cases) == 0 {
		t.Fatal("conformance manifest has no cases")
	}

	for _, tc := range file.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.DenyAll && tc.AllowCaps != nil {
				t.Fatal("case sets both deny_all and allow_caps")
			}
			fixture := filepath.Join(root, tc.Fixture)
			report, err := VerifyParityWithOptions(fixture, tc.Targets, tc.runOptions())
			if err != nil {
				t.Fatalf("VerifyParityWithOptions: %v", err)
			}
			if report.OverallStatus != StatusPass {
				t.Fatalf("parity: %s", strings.Join(report.Discrepancies, "\n"))
			}

			wantOut := tc.wantStdout()
			if tc.Expect == "PASS" {
				if NormalizeOutput(report.CanonicalResult.Stdout) != NormalizeOutput(wantOut) {
					t.Fatalf("canonical stdout = %q, want %q", NormalizeOutput(report.CanonicalResult.Stdout), NormalizeOutput(wantOut))
				}
				if report.CanonicalResult.Status != StatusPass {
					t.Fatalf("canonical status = %s, want PASS (%s)", report.CanonicalResult.Status, report.CanonicalResult.ErrorMessage)
				}
			} else {
				if report.CanonicalResult.Status != StatusRuntimeFailure {
					t.Fatalf("canonical status = %s, want runtime failure (%s)", report.CanonicalResult.Status, report.CanonicalResult.ErrorMessage)
				}
				if report.CanonicalResult.ErrorClass != tc.Expect {
					t.Fatalf("canonical error class = %q, want %q\n%s", report.CanonicalResult.ErrorClass, tc.Expect, report.CanonicalResult.ErrorMessage)
				}
			}

			for _, tgt := range tc.Targets {
				res := report.TargetResults[tgt]
				if tgt == TargetJavaScript && res.Status == StatusBackendUnsupported && strings.Contains(res.ErrorMessage, "node runtime not available") {
					t.Log("javascript skipped: node runtime not available")
					continue
				}
				if tc.Expect != "PASS" {
					if res.ErrorClass != tc.Expect {
						t.Errorf("%s error class = %q, want %q\n%s", tgt, res.ErrorClass, tc.Expect, res.ErrorMessage)
					}
					if res.ExitCode == 0 {
						t.Errorf("%s exit = 0, want rejection", tgt)
					}
					if strings.TrimSpace(res.Stdout) != "" {
						t.Errorf("%s stdout = %q, want empty on rejection", tgt, res.Stdout)
					}
				}
				if tc.Forbid != "" && strings.Contains(res.Stdout+res.Stderr+res.ErrorMessage, tc.Forbid) {
					t.Errorf("%s leaked %q", tgt, tc.Forbid)
				}
			}
		})
	}
}

func TestCompareToCanonicalDetectsDrift(t *testing.T) {
	canonical := ExecutionResult{Target: TargetBytecode, Status: StatusPass, Stdout: "14\n", ExitCode: 0}
	drifted := canonical
	drifted.Stdout = "15\n"
	if diffs := compareToCanonical(canonical, drifted, TargetGo); len(diffs) == 0 {
		t.Fatal("stdout drift was not detected")
	}

	denied := ExecutionResult{Status: StatusRuntimeFailure, ErrorClass: "CAPABILITY_DENIED", ExitCode: 1}
	other := ExecutionResult{Status: StatusRuntimeFailure, ErrorClass: "TYPE_ERROR", ExitCode: 1}
	if diffs := compareToCanonical(denied, other, TargetInterpreter); len(diffs) == 0 {
		t.Fatal("error-class drift was not detected")
	}

	same := denied
	if diffs := compareToCanonical(denied, same, TargetInterpreter); len(diffs) != 0 {
		t.Fatalf("matching rejection reported drift: %v", diffs)
	}
}

func TestABIPropertyArithmetic(t *testing.T) {
	rng := rand.New(rand.NewSource(90))
	targets := []Target{TargetBytecode, TargetInterpreter, TargetGo, TargetJavaScript}
	opts := RunOptions{DenyAll: true, JSRoot: "web_app"}
	var firstStdout string
	var firstSource string

	for i := 0; i < 8; i++ {
		node := genArith(rng, 2)
		expr := formatArith(node)
		want := strconv.Itoa(evalArith(node))
		dir := t.TempDir()
		path := filepath.Join(dir, "arith.howl")
		source := "(cli_app (print " + expr + "))\n"
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		report, err := VerifyParityWithOptions(path, targets, opts)
		if err != nil {
			t.Fatalf("case %d %s: %v", i, expr, err)
		}
		if report.OverallStatus != StatusPass {
			t.Fatalf("case %d %s parity:\n%s", i, expr, strings.Join(report.Discrepancies, "\n"))
		}
		if got := NormalizeOutput(report.CanonicalResult.Stdout); got != want {
			t.Fatalf("case %d %s stdout = %q, want %q", i, expr, got, want)
		}
		if i == 0 {
			firstStdout = report.CanonicalResult.Stdout
			firstSource = source
			again, err := VerifyParityWithOptions(path, []Target{TargetBytecode}, opts)
			if err != nil {
				t.Fatal(err)
			}
			if again.CanonicalResult.Stdout != firstStdout {
				t.Fatalf("repeat of %s stdout = %q, want %q", firstSource, again.CanonicalResult.Stdout, firstStdout)
			}
		}
	}
}

type arithExpr struct {
	op    string
	value int
	left  *arithExpr
	right *arithExpr
}

func genArith(rng *rand.Rand, depth int) *arithExpr {
	if depth == 0 || rng.Intn(4) == 0 {
		// Literals stay non-negative. The lexer has no negative integer token.
		return &arithExpr{value: rng.Intn(13)}
	}
	ops := []string{"+", "-", "*"}
	return &arithExpr{
		op:    ops[rng.Intn(len(ops))],
		left:  genArith(rng, depth-1),
		right: genArith(rng, depth-1),
	}
}

func evalArith(node *arithExpr) int {
	switch node.op {
	case "":
		return node.value
	case "+":
		return evalArith(node.left) + evalArith(node.right)
	case "-":
		return evalArith(node.left) - evalArith(node.right)
	case "*":
		return evalArith(node.left) * evalArith(node.right)
	default:
		panic("unknown op " + node.op)
	}
}

func formatArith(node *arithExpr) string {
	if node.op == "" {
		return strconv.Itoa(node.value)
	}
	return "(" + node.op + " " + formatArith(node.left) + " " + formatArith(node.right) + ")"
}
