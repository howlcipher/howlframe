package hfir

import (
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func parseSource(t *testing.T, source string) *parser.Parser {
	t.Helper()
	return parser.NewParser(lexer.NewLexer(source), "constructs_test.howl")
}

func TestVerifyConstructsOnlyAppliesToBytecodeTarget(t *testing.T) {
	root := parseSource(t, `(cli_app (let (x 0) (match x (0 (print "zero")))))`).ParseExpression()

	if diags := VerifyConstructs(root, TargetBytecode); len(diags) != 1 {
		t.Fatalf("bytecode target: got %d diagnostics, want 1: %+v", len(diags), diags)
	}

	// -validate passes the empty target by contract; wasm, go, javascript
	// and interpreter have their own rules (or none) and must be unaffected.
	for _, target := range []string{"", "wasm", "go", "javascript", "interpreter"} {
		if diags := VerifyConstructs(root, target); len(diags) != 0 {
			t.Errorf("target %q: got %d diagnostics, want 0: %+v", target, len(diags), diags)
		}
	}
}

func TestVerifyConstructsEmitsTargetInfeasibleWithFullContext(t *testing.T) {
	root := parseSource(t, `(cli_app (defun f (a) (print a)) (test "void" (call f "hi")))`).ParseExpression()

	diags := VerifyConstructs(root, TargetBytecode)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %+v", len(diags), diags)
	}
	diag := diags[0]

	if diag.Code != "HFIR_TARGET_INFEASIBLE" {
		t.Errorf("Code = %q, want HFIR_TARGET_INFEASIBLE", diag.Code)
	}
	if diag.Severity != SeverityError {
		t.Errorf("Severity = %q, want %q", diag.Severity, SeverityError)
	}
	if diag.ContractVersion != DiagnosticContractVersion {
		t.Errorf("ContractVersion = %q, want %q", diag.ContractVersion, DiagnosticContractVersion)
	}
	if diag.Target != TargetBytecode {
		t.Errorf("Target = %q, want %q", diag.Target, TargetBytecode)
	}
	if diag.Location.Filename != "constructs_test.howl" {
		t.Errorf("Location.Filename = %q, want constructs_test.howl", diag.Location.Filename)
	}
	if diag.Location.Line == 0 && diag.Location.Column == 0 {
		t.Error("diagnostic carries no source location")
	}
	for _, want := range []string{`"test"`, "improvements.md #96"} {
		if !strings.Contains(diag.Message, want) {
			t.Errorf("Message = %q, missing %q", diag.Message, want)
		}
	}
}

// TestVerifyConstructsAcceptsSubFormsOfSupportedConstructs is the HFIR-level
// companion to internal/construct's scan tests. It is the reason this check
// runs over the AST: LowerAST would give each of these a node Kind named after
// a non-construct head.
func TestVerifyConstructsAcceptsSubFormsOfSupportedConstructs(t *testing.T) {
	sources := map[string]string{
		"let binding":             `(cli_app (let (x 0) (print x)))`,
		"try_let catch":           `(cli_app (try_let (b (read_file "f")) (catch e (print "err")) (print "ok")))`,
		"http_server route":       `(http_server 8080 (route "/x" (lambda (req) (res 200 "text/plain" "ok"))))`,
		"optimize_signature test": `(cli_app (optimize_signature p (metric "m") (test "go test ./...") (candidate "a" "A") (print "b")))`,
	}
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			root := parseSource(t, source).ParseExpression()
			if diags := VerifyConstructs(root, TargetBytecode); len(diags) != 0 {
				t.Errorf("got %d diagnostics on a valid program: %+v", len(diags), diags)
			}
		})
	}
}

// TestVerifyConstructsMatchesLoweredKindsWouldNotDocuments the concrete
// difference between the AST scan and a deny-list over HFIR node kinds: the
// same program lowers to a graph containing a node whose Kind is "catch",
// which a kind-based rule would reject.
func TestVerifyConstructsMatchesLoweredKindsWouldNot(t *testing.T) {
	root := parseSource(t, `(cli_app (try_let (b (read_file "f")) (catch e (print "err")) (print "ok")))`).ParseExpression()

	graph, err := LowerAST(root, "constructs_test.howl")
	if err != nil {
		t.Fatalf("LowerAST failed: %v", err)
	}
	sawCatchKind := false
	for _, node := range graph.Nodes {
		if node.Kind == "catch" {
			sawCatchKind = true
			break
		}
	}
	if !sawCatchKind {
		t.Fatal("expected LowerAST to produce a node with Kind \"catch\"; the AST-scan rationale in constructs.go needs revisiting")
	}
	if diags := VerifyConstructs(root, TargetBytecode); len(diags) != 0 {
		t.Errorf("VerifyConstructs rejected a valid try_let program: %+v", diags)
	}
}

func TestVerifyConstructsHandlesNilRoot(t *testing.T) {
	if diags := VerifyConstructs(nil, TargetBytecode); len(diags) != 0 {
		t.Errorf("VerifyConstructs(nil) = %+v, want none", diags)
	}
}
