package hfir

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func lowerBothIssue159(t *testing.T, source string) (hfirMain, astMain []bytecode.BCInstruction, graph *Graph) {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "i159.howl").ParseExpression()
	ast.Desugar(root)
	checker.Check(root)
	graph, err := LowerAST(root, "i159.howl")
	if err != nil {
		t.Fatalf("LowerAST: %v", err)
	}
	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode diagnostics = %#v", diagnostics)
	}
	return program.Main, bytecode.CompileToBytecode(root).Main, graph
}

func sameInstructionStream(t *testing.T, got, want []bytecode.BCInstruction) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("HFIR emitted %d instructions, AST compiler %d:\nhfir %v\nast  %v", len(got), len(want), got, want)
	}
	for i := range got {
		named := got[i].Op == bytecode.OpLoadVar || got[i].Op == bytecode.OpStoreVar || got[i].Op == bytecode.OpParseJson || got[i].Op == bytecode.OpMapGet
		if got[i].OpString != want[i].OpString || got[i].IntOperand != want[i].IntOperand || (named && got[i].StringOperand != want[i].StringOperand) {
			t.Fatalf("instruction %d: HFIR %s %q %d, AST %s %q %d", i, got[i].OpString, got[i].StringOperand, got[i].IntOperand, want[i].OpString, want[i].StringOperand, want[i].IntOperand)
		}
	}
}

// Root cause of #155 and of the x02/x03/x07/x08/x15/x27 divergences:
// PARSE_JSON reads its body by variable name, but HFIR also evaluated the
// body symbol, leaving a stray string on the operand stack.
func TestIssue159ParseJSONDoesNotLeakItsBody(t *testing.T) {
	got, want, _ := lowerBothIssue159(t, `(cli_app (let (s "{\"z\": 1}") (print (encode_json (parse_json Any s)) (let (v (parse_json Any s)) (map_get v "z")))))`)
	sameInstructionStream(t, got, want)
	for i, inst := range got {
		if inst.Op == bytecode.OpParseJson && i > 0 && got[i-1].Op == bytecode.OpLoadVar && got[i-1].StringOperand == inst.StringOperand {
			t.Fatalf("instruction %d: PARSE_JSON preceded by a LOAD_VAR of its own body", i)
		}
	}
}

// PARSE_JSON can only name a variable; a content edge that is not that bound
// symbol fails closed instead of compiling to a wrong program.
func TestIssue159ParseJSONNonSymbolContentFailsClosed(t *testing.T) {
	_, _, graph := lowerBothIssue159(t, `(cli_app (let (s "[1]") (print (parse_json Any s))))`)
	for _, node := range graph.Nodes {
		if node.Kind == "parse_json" {
			constant := &Node{Kind: "const", LiteralKind: "STRING", Value: "[1]"}
			node.DataInputs[0].SourceNode = graph.AddNode(constant)
		}
	}
	if program, diagnostics := LowerToBytecode(graph); program != nil || len(diagnostics) == 0 {
		t.Fatalf("non-symbol parse_json content lowered: %v %v", program, diagnostics)
	}
}

// Root cause of the x10/x11 compile failures: map_get whose dict is an
// expression had no HFIR shape. It now matches the AST compiler exactly.
func TestIssue159MapGetExpressionTarget(t *testing.T) {
	got, want, _ := lowerBothIssue159(t, `(cli_app (let (d (dict ("m" (dict ("k" 7))))) (print (map_get (map_get d "m") "k"))))`)
	sameInstructionStream(t, got, want)
}
