package hfir

import (
	"bytes"
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
	for index := range program.Main {
		if origin, ok := program.TrustedMainOriginAt(index); !ok || graph.NodeByID(NodeID(origin)) == nil {
			t.Fatalf("instruction %d origin = (%q, %v), want canonical HFIR node", index, origin, ok)
		}
	}
	var artifact bytes.Buffer
	if err := bytecode.WriteArtifact(&artifact, program); err != nil {
		t.Fatal(err)
	}
	decoded, err := bytecode.ReadArtifact(&artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded.TrustedMainOriginAt(0); ok {
		t.Fatal("durable HFBC artifact unexpectedly retained ephemeral HFIR provenance")
	}
	program.Main[0].StringOperand = "mutated-after-lowering"
	if _, ok := program.TrustedMainOriginAt(0); ok {
		t.Fatal("mutated bytecode retained trusted HFIR provenance")
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

func TestLowerToBytecodeExecEmitsExistingOpcode(t *testing.T) {
	graph := NewGraph()
	cmd := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "printf"})
	arg := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "phase2b-exec-marker"})
	entry := graph.AddNode(&Node{
		Kind: "exec",
		DataInputs: []DataEdge{
			{Name: "cmd", SourceNode: cmd},
			{Name: "arg", SourceNode: arg},
		},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	last := program.Main[len(program.Main)-1]
	if last.Op != bytecode.OpExec || last.OpString != "EXEC" || last.IntOperand != 1 {
		t.Fatalf("last instruction = %#v, want EXEC with one argument", last)
	}
	if bytecode.Registry[last.Op].Capability != "process" {
		t.Fatalf("EXEC capability = %q, want process", bytecode.Registry[last.Op].Capability)
	}
}

func TestLowerToBytecodeExecRequiresCommand(t *testing.T) {
	graph := NewGraph()
	arg := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "phase2b-exec-marker"})
	entry := graph.AddNode(&Node{
		Kind:       "exec",
		Provenance: Provenance{Filename: "exec.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{{Name: "arg", SourceNode: arg}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || diagnostics[0].RelatedNode != entry {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
}

func TestLowerToBytecodeExecRejectsArgumentBeforeCommand(t *testing.T) {
	graph := NewGraph()
	arg := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "phase2b-exec-marker"})
	cmd := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "printf"})
	entry := graph.AddNode(&Node{
		Kind: "exec",
		DataInputs: []DataEdge{
			{Name: "arg", SourceNode: arg},
			{Name: "cmd", SourceNode: cmd},
		},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || diagnostics[0].RelatedNode != entry {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
}

func TestLowerToBytecodeFetchEmitsExistingOpcode(t *testing.T) {
	graph := NewGraph()
	url := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "http://127.0.0.1:47653/howlframe-abi-v1-phase2d"})
	method := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "GET"})
	entry := graph.AddNode(&Node{
		Kind: "fetch",
		DataInputs: []DataEdge{
			{Name: "url", SourceNode: url},
			{Name: "method", SourceNode: method},
		},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	if len(program.Main) != 3 {
		t.Fatalf("instructions = %d, want URL, method, FETCH", len(program.Main))
	}
	last := program.Main[len(program.Main)-1]
	if last.Op != bytecode.OpFetch || last.OpString != "FETCH" || last.IntOperand != 0 || last.StringOperand != "" {
		t.Fatalf("last instruction = %#v, want bare FETCH", last)
	}
	if program.Main[0].ValueOperand != "http://127.0.0.1:47653/howlframe-abi-v1-phase2d" || program.Main[1].ValueOperand != "GET" {
		t.Fatalf("operands = %#v, %#v, want URL then method", program.Main[0].ValueOperand, program.Main[1].ValueOperand)
	}
	if bytecode.Registry[last.Op].Capability != "network" {
		t.Fatalf("FETCH capability = %q, want network", bytecode.Registry[last.Op].Capability)
	}
}

func TestLowerToBytecodeFetchSkipsBodyOperand(t *testing.T) {
	graph := NewGraph()
	url := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "http://127.0.0.1:47653/howlframe-abi-v1-phase2d"})
	method := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "PUT"})
	body := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "phase2d-body-not-sent"})
	entry := graph.AddNode(&Node{
		Kind: "fetch",
		DataInputs: []DataEdge{
			{Name: "url", SourceNode: url},
			{Name: "method", SourceNode: method},
			{Name: "body", SourceNode: body},
		},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if len(program.Main) != 3 || program.Main[2].Op != bytecode.OpFetch {
		t.Fatalf("instructions = %#v, want URL, method, FETCH", program.Main)
	}
	if program.Main[0].ValueOperand != "http://127.0.0.1:47653/howlframe-abi-v1-phase2d" || program.Main[1].ValueOperand != "PUT" {
		t.Fatalf("operands = %#v, %#v, want URL then method", program.Main[0].ValueOperand, program.Main[1].ValueOperand)
	}
	for _, inst := range program.Main {
		if inst.ValueOperand == "phase2d-body-not-sent" || inst.StringOperand == "phase2d-body-not-sent" {
			t.Fatalf("FETCH compiled the body: %#v", program.Main)
		}
	}
}

func TestLowerToBytecodeFetchRequiresURLAndMethod(t *testing.T) {
	graph := NewGraph()
	method := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "GET"})
	entry := graph.AddNode(&Node{
		Kind:       "fetch",
		Provenance: Provenance{Filename: "fetch.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{{Name: "method", SourceNode: method}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || diagnostics[0].RelatedNode != entry {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
}

func TestLowerToBytecodeFetchRejectsMethodBeforeURL(t *testing.T) {
	graph := NewGraph()
	method := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "GET"})
	url := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "http://127.0.0.1:47653/howlframe-abi-v1-phase2d"})
	entry := graph.AddNode(&Node{
		Kind: "fetch",
		DataInputs: []DataEdge{
			{Name: "method", SourceNode: method},
			{Name: "url", SourceNode: url},
		},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || diagnostics[0].RelatedNode != entry {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
}

func TestLowerToBytecodeWriteFileRequiresPathAndData(t *testing.T) {
	graph := NewGraph()
	path := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "/tmp/x"})
	entry := graph.AddNode(&Node{
		Kind:       "write_file",
		Provenance: Provenance{Filename: "write.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{{Name: "path", SourceNode: path}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || diagnostics[0].RelatedNode != entry {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
}

func TestLowerToBytecodeHTMLEscapeEmitsExistingOpcode(t *testing.T) {
	graph := NewGraph()
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a&b<c>"})
	entry := graph.AddNode(&Node{
		Kind:       "html_escape",
		DataInputs: []DataEdge{{Name: "value", SourceNode: text}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	if len(program.Main) != 2 {
		t.Fatalf("instructions = %#v, want text then HTML_ESCAPE", program.Main)
	}
	if program.Main[0].Op != bytecode.OpLoadConst || program.Main[0].ValueOperand != "a&b<c>" {
		t.Fatalf("operand = %#v, want the text", program.Main[0])
	}
	last := program.Main[1]
	if last.Op != bytecode.OpHTMLEscape || last.OpString != "HTML_ESCAPE" || last.StringOperand != "" || last.IntOperand != 0 {
		t.Fatalf("last instruction = %#v, want bare HTML_ESCAPE", last)
	}
	if bytecode.Registry[last.Op].Capability != "" {
		t.Fatalf("HTML_ESCAPE capability = %q, want none", bytecode.Registry[last.Op].Capability)
	}
}

func TestLowerToBytecodeHTMLEscapeRequiresValue(t *testing.T) {
	graph := NewGraph()
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a&b"})
	entry := graph.AddNode(&Node{
		Kind:       "html_escape",
		Provenance: Provenance{Filename: "escape.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{{Name: "text", SourceNode: text}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || diagnostics[0].RelatedNode != entry {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
	if diagnostics[0].Message != "html_escape requires value" {
		t.Fatalf("message = %q", diagnostics[0].Message)
	}
}

func TestLowerASTHTMLEscapeRejectsSiblings(t *testing.T) {
	cases := []struct {
		name   string
		source string
		kind   string
	}{
		{name: "missing text", source: `(cli_app (print (html_escape)))`, kind: "html_escape"},
		{name: "extra text", source: `(cli_app (print (html_escape "a" "b")))`, kind: "html_escape"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := parser.NewParser(lexer.NewLexer(tc.source), "escape.howl").ParseExpression()
			graph, err := LowerAST(root, "escape.howl")
			if err != nil {
				t.Fatal(err)
			}
			program, diagnostics := LowerToBytecode(graph)
			if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode {
				t.Fatalf("LowerToBytecode() program=%v diags=%#v", program != nil, diagnostics)
			}
			want := "html_escape requires value"
			if tc.kind != "html_escape" {
				want = "node kind \"" + tc.kind + "\" is not in the Phase-1 executable subset"
			}
			if diagnostics[0].Message != want {
				t.Fatalf("message = %q, want %q", diagnostics[0].Message, want)
			}
		})
	}
}

func TestLowerToBytecodeAttrEscapeEmitsExistingOpcode(t *testing.T) {
	graph := NewGraph()
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a&b<c>"})
	entry := graph.AddNode(&Node{
		Kind:       "attr_escape",
		DataInputs: []DataEdge{{Name: "value", SourceNode: text}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	if len(program.Main) != 2 {
		t.Fatalf("instructions = %#v, want text then ATTR_ESCAPE", program.Main)
	}
	if program.Main[0].Op != bytecode.OpLoadConst || program.Main[0].ValueOperand != "a&b<c>" {
		t.Fatalf("operand = %#v, want the text", program.Main[0])
	}
	last := program.Main[1]
	if last.Op != bytecode.OpAttrEscape || last.OpString != "ATTR_ESCAPE" || last.StringOperand != "" || last.IntOperand != 0 {
		t.Fatalf("last instruction = %#v, want bare ATTR_ESCAPE", last)
	}
	if bytecode.Registry[last.Op].Capability != "" {
		t.Fatalf("ATTR_ESCAPE capability = %q, want none", bytecode.Registry[last.Op].Capability)
	}
}

func TestLowerToBytecodeAttrEscapeRequiresValue(t *testing.T) {
	graph := NewGraph()
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a&b"})
	entry := graph.AddNode(&Node{
		Kind:       "attr_escape",
		Provenance: Provenance{Filename: "escape.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{{Name: "text", SourceNode: text}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || diagnostics[0].RelatedNode != entry {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
	if diagnostics[0].Message != "attr_escape requires value" {
		t.Fatalf("message = %q", diagnostics[0].Message)
	}
}

func TestLowerASTAttrEscapeRejectsSiblings(t *testing.T) {
	cases := []struct {
		name   string
		source string
		kind   string
	}{
		{name: "missing text", source: `(cli_app (print (attr_escape)))`, kind: "attr_escape"},
		{name: "extra text", source: `(cli_app (print (attr_escape "a" "b")))`, kind: "attr_escape"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := parser.NewParser(lexer.NewLexer(tc.source), "escape.howl").ParseExpression()
			graph, err := LowerAST(root, "escape.howl")
			if err != nil {
				t.Fatal(err)
			}
			program, diagnostics := LowerToBytecode(graph)
			if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode {
				t.Fatalf("LowerToBytecode() program=%v diags=%#v", program != nil, diagnostics)
			}
			want := "attr_escape requires value"
			if tc.kind != "attr_escape" {
				want = "node kind \"" + tc.kind + "\" is not in the Phase-1 executable subset"
			}
			if diagnostics[0].Message != want {
				t.Fatalf("message = %q, want %q", diagnostics[0].Message, want)
			}
		})
	}
}

func TestLowerToBytecodeRegexMatchEmitsExistingOpcode(t *testing.T) {
	spec := bytecode.Registry[bytecode.OpRegexMatch]
	if spec.Name != "REGEX_MATCH" || spec.Pops != 2 || spec.Pushes != 1 || len(spec.Operands) != 0 || spec.Capability != "" {
		t.Fatalf("REGEX_MATCH = %+v, want pops 2, pushes 1, no operands, no capability", spec)
	}
	graph := NewGraph()
	pattern := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "^a$"})
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a"})
	entry := graph.AddNode(&Node{
		Kind: "regex_match",
		DataInputs: []DataEdge{
			{Name: "pattern", SourceNode: pattern},
			{Name: "string", SourceNode: text},
		},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	if len(program.Main) != 3 {
		t.Fatalf("instructions = %#v, want pattern, string, then REGEX_MATCH", program.Main)
	}
	if program.Main[0].Op != bytecode.OpLoadConst || program.Main[0].ValueOperand != "^a$" {
		t.Fatalf("pattern operand = %#v", program.Main[0])
	}
	if program.Main[1].Op != bytecode.OpLoadConst || program.Main[1].ValueOperand != "a" {
		t.Fatalf("string operand = %#v", program.Main[1])
	}
	last := program.Main[2]
	if last.Op != bytecode.OpRegexMatch || last.OpString != "REGEX_MATCH" || last.StringOperand != "" || last.IntOperand != 0 {
		t.Fatalf("last instruction = %#v, want bare REGEX_MATCH", last)
	}
	if bytecode.Registry[last.Op].Capability != "" {
		t.Fatalf("REGEX_MATCH capability = %q, want none", bytecode.Registry[last.Op].Capability)
	}
}

func TestLowerToBytecodeRegexMatchRequiresPatternAndString(t *testing.T) {
	graph := NewGraph()
	pattern := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "^a$"})
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a"})
	entry := graph.AddNode(&Node{
		Kind:       "regex_match",
		Provenance: Provenance{Filename: "match.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{
			{Name: "string", SourceNode: text},
			{Name: "pattern", SourceNode: pattern},
		},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || diagnostics[0].RelatedNode != entry {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
	if diagnostics[0].Message != "regex_match requires pattern and string" {
		t.Fatalf("message = %q", diagnostics[0].Message)
	}
}

func TestLowerASTRegexMatchRejectsSiblings(t *testing.T) {
	cases := []struct {
		name   string
		source string
		kind   string
	}{
		{name: "missing args", source: `(cli_app (print (regex_match)))`, kind: "regex_match"},
		{name: "one arg", source: `(cli_app (print (regex_match "^a$")))`, kind: "regex_match"},
		{name: "extra arg", source: `(cli_app (print (regex_match "^a$" "a" "b")))`, kind: "regex_match"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := parser.NewParser(lexer.NewLexer(tc.source), "match.howl").ParseExpression()
			graph, err := LowerAST(root, "match.howl")
			if err != nil {
				t.Fatal(err)
			}
			program, diagnostics := LowerToBytecode(graph)
			if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode {
				t.Fatalf("LowerToBytecode() program=%v diags=%#v", program != nil, diagnostics)
			}
			want := "regex_match requires pattern and string"
			if tc.kind != "regex_match" {
				want = "node kind \"" + tc.kind + "\" is not in the Phase-1 executable subset"
			}
			if diagnostics[0].Message != want {
				t.Fatalf("message = %q, want %q", diagnostics[0].Message, want)
			}
		})
	}
}

func TestLowerToBytecodeTimeNowEmitsExistingOpcode(t *testing.T) {
	spec := bytecode.Registry[bytecode.OpTimeNow]
	if spec.Name != "TIME_NOW" || spec.Pops != 0 || spec.Pushes != 1 || len(spec.Operands) != 0 || spec.Capability != "" {
		t.Fatalf("TIME_NOW = %+v, want pops 0, pushes 1, no operands, no capability", spec)
	}
	graph := NewGraph()
	entry := graph.AddNode(&Node{Kind: "time_now"})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	if len(program.Main) != 1 {
		t.Fatalf("instructions = %#v, want one TIME_NOW", program.Main)
	}
	inst := program.Main[0]
	if inst.Op != bytecode.OpTimeNow || inst.OpString != "TIME_NOW" || inst.StringOperand != "" || inst.IntOperand != 0 || inst.ValueOperand != nil {
		t.Fatalf("instruction = %#v, want bare TIME_NOW", inst)
	}
	if bytecode.Registry[inst.Op].Capability != "" {
		t.Fatalf("TIME_NOW capability = %q, want none", bytecode.Registry[inst.Op].Capability)
	}
}

func TestLowerToBytecodeTimeNowRejectsDataEdge(t *testing.T) {
	graph := NewGraph()
	extra := graph.AddNode(&Node{Kind: "const", LiteralKind: "INT", Value: "1"})
	entry := graph.AddNode(&Node{
		Kind:       "time_now",
		Provenance: Provenance{Filename: "time.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{{Name: "value", SourceNode: extra}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || diagnostics[0].RelatedNode != entry {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
	if diagnostics[0].Message != "time_now takes no arguments" {
		t.Fatalf("message = %q", diagnostics[0].Message)
	}
}

func TestLowerASTTimeNowHasNoDataEdge(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app (print (time_now)))`), "time.howl").ParseExpression()
	graph, err := LowerAST(root, "time.howl")
	if err != nil {
		t.Fatal(err)
	}
	var found int
	for _, node := range graph.Nodes {
		if node.Kind != "time_now" {
			continue
		}
		found++
		if len(node.DataInputs) != 0 || len(node.ControlEdges) != 0 {
			t.Fatalf("time_now edges %#v control %v, want none", node.DataInputs, node.ControlEdges)
		}
	}
	if found != 1 {
		t.Fatalf("time_now nodes = %d, want 1", found)
	}
	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() program=%v diags=%#v", program != nil, diagnostics)
	}
	var clocks int
	for _, inst := range program.Main {
		if inst.Op == bytecode.OpTimeNow {
			clocks++
			if inst.StringOperand != "" || inst.IntOperand != 0 || inst.ValueOperand != nil {
				t.Fatalf("TIME_NOW = %#v, want a bare instruction", inst)
			}
		}
		if inst.Op == bytecode.OpLoadConst {
			t.Fatalf("program loaded a constant %#v", inst)
		}
	}
	if clocks != 1 {
		t.Fatalf("TIME_NOW count = %d, want 1", clocks)
	}
}

func TestLowerASTTimeNowRejectsArgument(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app (print (time_now 1)))`), "time.howl").ParseExpression()
	graph, err := LowerAST(root, "time.howl")
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode {
		t.Fatalf("LowerToBytecode() program=%v diags=%#v", program != nil, diagnostics)
	}
	if diagnostics[0].Message != "time_now takes no arguments" {
		t.Fatalf("message = %q", diagnostics[0].Message)
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
