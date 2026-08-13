package hfir

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestLowerToBytecodeBuildsValidatedProgramFromSemanticHFIR(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app (let (x 1) (if (> x 0) (print "yes") (print "no"))))`), "phase1.howl").ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, "phase1.howl")
	if err != nil {
		t.Fatalf("LowerAST() error = %v", err)
	}
	if diagnostics := NewVerifier(graph, TargetBytecode).Verify(); len(diagnostics) != 0 {
		t.Fatalf("Verify() diagnostics = %#v", diagnostics)
	}
	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatalf("ValidateProgram() error = %v", err)
	}
	if len(program.Main) == 0 {
		t.Fatal("LowerToBytecode() emitted no instructions")
	}
}

func TestLowerASTUsesExplicitCollectionOperandRoles(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app (let (items (list "a")) (let (record (dict ("key" "value"))) (do (append items "b") (map_set record "key" "next") (map_get record "key") (map_delete record "key") (list_get items 0)))))`), "roles.howl").ParseExpression()
	checker.Check(root)
	graph, err := LowerAST(root, "roles.howl")
	if err != nil {
		t.Fatalf("LowerAST() error = %v", err)
	}
	want := map[string][]string{
		"append":     {"item"},
		"map_set":    {"key", "value"},
		"map_get":    {"key"},
		"map_delete": {"key"},
		"list_get":   {"index"},
	}
	for _, node := range graph.Nodes {
		roles, ok := want[node.Kind]
		if !ok {
			continue
		}
		if len(node.DataInputs) != len(roles) {
			t.Fatalf("%s edges = %#v, want roles %v", node.Kind, node.DataInputs, roles)
		}
		for index, role := range roles {
			if node.DataInputs[index].Name != role {
				t.Fatalf("%s edge %d = %q, want %q", node.Kind, index, node.DataInputs[index].Name, role)
			}
		}
		delete(want, node.Kind)
	}
	if len(want) != 0 {
		t.Fatalf("missing semantic collection nodes: %#v", want)
	}
}

func TestLowerToBytecodeFailsClosedWithProvenance(t *testing.T) {
	graph := NewGraph()
	entry := graph.AddNode(&Node{
		Kind:       "semantic_match",
		Provenance: Provenance{Filename: "unsupported.howl", Line: 7, Column: 3},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil {
		t.Fatalf("LowerToBytecode() program = %#v, want nil", program)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v, want exactly one", diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != BytecodeUnsupportedCode || diagnostic.Severity != SeverityError || diagnostic.Target != TargetBytecode {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
	if diagnostic.RelatedNode != entry || diagnostic.Location.Filename != "unsupported.howl" || diagnostic.Location.Line != 7 || diagnostic.Location.Column != 3 {
		t.Fatalf("diagnostic provenance = %#v", diagnostic)
	}
}

func TestLowerToBytecodeRejectsMissingDataInput(t *testing.T) {
	graph := NewGraph()
	entry := graph.AddNode(&Node{
		Kind:       "print",
		Provenance: Provenance{Filename: "invalid.howl", Line: 3, Column: 1},
		DataInputs: []DataEdge{{Name: "value", SourceNode: "missing"}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
}
