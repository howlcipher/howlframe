package optimization

import (
	"encoding/json"
	"testing"
	"zero/internal/ast"
	"zero/internal/checker"
	"zero/internal/lexer"
	"zero/internal/parser"
)

func TestCompileAnalysisProducesDeterministicPlan(t *testing.T) {
	root := parseProgram(t, `(cli_app
		(optimize_signature first
			(metric "accuracy")
			(test "go test ./...")
			(candidate "baseline" "plain")
			(candidate "strict" "verified")
			(+ 1 2))
		(optimize_signature second
			(metric "latency")
			(test "go test ./internal/vm")
			(candidate "fast" "compact")
			(print "done")))`)
	analysis := checker.Analyze(root)
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", analysis.Diagnostics)
	}

	plan := CompileAnalysis(analysis)
	if plan.Format != FormatV1 || len(plan.Signatures) != 2 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	first := plan.Signatures[0]
	if first.Name != "first" || first.Metric != "accuracy" ||
		first.Line != 2 || first.Column != 3 || first.BodyType != "int" ||
		len(first.Tests) != 1 || first.Tests[0] != "go test ./..." ||
		len(first.Candidates) != 2 || first.Candidates[1].Label != "strict" ||
		first.Candidates[1].Payload != "verified" {
		t.Fatalf("unexpected first signature: %+v", first)
	}

	encodedFirst, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	encodedSecond, err := json.Marshal(CompileAnalysis(analysis))
	if err != nil {
		t.Fatalf("marshal second plan: %v", err)
	}
	if string(encodedFirst) != string(encodedSecond) {
		t.Fatalf("optimization plan JSON is not deterministic:\n%s\n%s", encodedFirst, encodedSecond)
	}
}

func TestCompileAnalysisAcceptsNil(t *testing.T) {
	plan := CompileAnalysis(nil)
	if plan.Format != FormatV1 || plan.Signatures == nil || len(plan.Signatures) != 0 {
		t.Fatalf("unexpected nil analysis plan: %+v", plan)
	}
}

func parseProgram(t *testing.T, source string) *ast.Node {
	t.Helper()
	p := parser.NewParser(lexer.NewLexer(source), "optimization_test.zero")
	root := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		t.Fatalf("parser stopped at %s", p.Cur.Value)
	}
	return root
}
