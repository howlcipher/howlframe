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

func assertLowerToBytecodeUnsupported(t *testing.T, graph *Graph, related NodeID, wantMsg string) {
	t.Helper()
	program, diagnostics := LowerToBytecode(graph)
	if program != nil || len(diagnostics) != 1 || diagnostics[0].Code != BytecodeUnsupportedCode || (related != "" && diagnostics[0].RelatedNode != related) {
		t.Fatalf("LowerToBytecode() = (%#v, %#v)", program, diagnostics)
	}
	if wantMsg != "" && diagnostics[0].Message != wantMsg {
		t.Fatalf("message = %q, want %q", diagnostics[0].Message, wantMsg)
	}
}

type astRejectCase struct {
	name, source, kind string
}

func assertLowerASTRejectsBytecode(t *testing.T, filename string, cases []astRejectCase, baseKind, wantMsg string) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := parser.NewParser(lexer.NewLexer(tc.source), filename).ParseExpression()
			graph, err := LowerAST(root, filename)
			if err != nil {
				t.Fatal(err)
			}
			want := wantMsg
			if tc.kind != "" && tc.kind != baseKind {
				want = `node kind "` + tc.kind + `" is not in the Phase-1 executable subset`
			}
			assertLowerToBytecodeUnsupported(t, graph, "", want)
		})
	}
}

func assertSingleBareOpcode(t *testing.T, program *bytecode.BCProgram, instIndex int, wantLen int, wantOp bytecode.Opcode, wantName, wantCap string) {
	t.Helper()
	if len(program.Main) != wantLen {
		t.Fatalf("instructions = %#v, len %d want %d", program.Main, len(program.Main), wantLen)
	}
	last := program.Main[instIndex]
	if last.Op != wantOp || last.OpString != wantName || last.StringOperand != "" || last.IntOperand != 0 {
		t.Fatalf("instruction = %#v, want bare %s", last, wantName)
	}
	if string(bytecode.Registry[last.Op].Capability) != wantCap {
		t.Fatalf("%s capability = %q, want %q", wantName, bytecode.Registry[last.Op].Capability, wantCap)
	}
}

func assertControlEdgesMatchInputs(t *testing.T, node *Node, wantCount int, roleNames ...string) {
	t.Helper()
	if len(node.ControlEdges) != wantCount || len(node.DataInputs) != wantCount {
		t.Fatalf("%s %s control=%v data=%#v", node.Kind, node.ID, node.ControlEdges, node.DataInputs)
	}
	for i, role := range roleNames {
		if node.DataInputs[i].Name != role || node.ControlEdges[i] != node.DataInputs[i].SourceNode {
			t.Fatalf("%s %s edge %d role=%q want %q control=%v data=%#v", node.Kind, node.ID, i, node.DataInputs[i].Name, role, node.ControlEdges, node.DataInputs)
		}
	}
}

func mustLowerAndValidateGraph(t *testing.T, graph *Graph, wantOps ...bytecode.Opcode) *bytecode.BCProgram {
	t.Helper()
	program, diags := LowerToBytecode(graph)
	if len(diags) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() diags=%#v", diags)
	}
	for _, op := range wantOps {
		if !hasOp(program.Main, op) {
			t.Fatalf("main = %#v, missing op %v", program.Main, op)
		}
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	return program
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

	program := mustLowerAndValidateGraph(t, graph)
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
	assertLowerToBytecodeUnsupported(t, graph, entry, "")
}

func TestLowerToBytecodeExecRejectsArgumentBeforeCommand(t *testing.T) {
	graph := NewGraph()
	arg := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "phase2b-exec-marker"})
	cmd := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "printf"})
	entry := graph.AddNode(&Node{
		Kind:       "exec",
		DataInputs: []DataEdge{{Name: "arg", SourceNode: arg}, {Name: "cmd", SourceNode: cmd}},
	})
	graph.EntryNode = entry
	assertLowerToBytecodeUnsupported(t, graph, entry, "")
}

func TestLowerToBytecodeFetchEmitsExistingOpcode(t *testing.T) {
	graph := NewGraph()
	url := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "http://127.0.0.1:47653/howlframe-abi-v1-phase2d"})
	method := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "GET"})
	entry := graph.AddNode(&Node{
		Kind:       "fetch",
		DataInputs: []DataEdge{{Name: "url", SourceNode: url}, {Name: "method", SourceNode: method}},
	})
	graph.EntryNode = entry

	program := mustLowerAndValidateGraph(t, graph)
	if program.Main[0].ValueOperand != "http://127.0.0.1:47653/howlframe-abi-v1-phase2d" || program.Main[1].ValueOperand != "GET" {
		t.Fatalf("operands = %#v, %#v, want URL then method", program.Main[0].ValueOperand, program.Main[1].ValueOperand)
	}
	assertSingleBareOpcode(t, program, 2, 3, bytecode.OpFetch, "FETCH", "network")
}

// TestLowerToBytecodeFetchSendsBodyOperand: since #143 (Owner-approved change
// to the flip criteria) a fetch body is sent, as the interpreter sends it, via
// the existing HTTP_REQUEST opcode, keeping the response body bytes.
func TestLowerToBytecodeFetchSendsBodyOperand(t *testing.T) {
	graph := NewGraph()
	url := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "http://127.0.0.1:47653/howlframe-abi-v1-phase2d"})
	method := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "PUT"})
	body := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "phase2d-body-sent"})
	entry := graph.AddNode(&Node{
		Kind:       "fetch",
		DataInputs: []DataEdge{{Name: "url", SourceNode: url}, {Name: "method", SourceNode: method}, {Name: "body", SourceNode: body}},
	})
	graph.EntryNode = entry

	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	want := []bytecode.Opcode{bytecode.OpLoadConst, bytecode.OpLoadConst, bytecode.OpLoadConst, bytecode.OpConvert, bytecode.OpHttpRequest, bytecode.OpLoadConst, bytecode.OpMapGet}
	if len(program.Main) != len(want) {
		t.Fatalf("instructions = %#v, want url, method, body, CONVERT, HTTP_REQUEST 1, \"body\", MAP_GET", program.Main)
	}
	for i, op := range want {
		if program.Main[i].Op != op {
			t.Fatalf("instruction %d = %#v, want op %v", i, program.Main[i], op)
		}
	}
	if program.Main[0].ValueOperand != "http://127.0.0.1:47653/howlframe-abi-v1-phase2d" || program.Main[1].ValueOperand != "PUT" || program.Main[2].ValueOperand != "phase2d-body-sent" {
		t.Fatalf("operands = %#v", program.Main[:3])
	}
	if program.Main[3].StringOperand != "to_string" || program.Main[4].IntOperand != 1 || program.Main[5].ValueOperand != "body" {
		t.Fatalf("body lowering = %#v", program.Main[3:])
	}
	if bytecode.Registry[program.Main[4].Op].Capability != "network" {
		t.Fatalf("HTTP_REQUEST capability = %q, want network", bytecode.Registry[program.Main[4].Op].Capability)
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
	assertLowerToBytecodeUnsupported(t, graph, entry, "")
}

func TestLowerToBytecodeFetchRejectsMethodBeforeURL(t *testing.T) {
	graph := NewGraph()
	method := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "GET"})
	url := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "http://127.0.0.1:47653/howlframe-abi-v1-phase2d"})
	entry := graph.AddNode(&Node{
		Kind:       "fetch",
		DataInputs: []DataEdge{{Name: "method", SourceNode: method}, {Name: "url", SourceNode: url}},
	})
	graph.EntryNode = entry
	assertLowerToBytecodeUnsupported(t, graph, entry, "")
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
	assertLowerToBytecodeUnsupported(t, graph, entry, "")
}

func assertLowerSingleConstStringOp(t *testing.T, kind string, op bytecode.Opcode, opStr string) {
	t.Helper()
	graph := NewGraph()
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a&b<c>"})
	graph.EntryNode = graph.AddNode(&Node{Kind: kind, DataInputs: []DataEdge{{Name: "value", SourceNode: text}}})
	program := mustLowerAndValidateGraph(t, graph)
	if program.Main[0].Op != bytecode.OpLoadConst || program.Main[0].ValueOperand != "a&b<c>" {
		t.Fatalf("operand = %#v, want the text", program.Main[0])
	}
	assertSingleBareOpcode(t, program, 1, 2, op, opStr, "")
}

func TestLowerToBytecodeHTMLEscapeEmitsExistingOpcode(t *testing.T) {
	assertLowerSingleConstStringOp(t, "html_escape", bytecode.OpHTMLEscape, "HTML_ESCAPE")
}

func assertEscapeRequiresValue(t *testing.T, kind string) {
	t.Helper()
	graph := NewGraph()
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a&b"})
	entry := graph.AddNode(&Node{
		Kind:       kind,
		Provenance: Provenance{Filename: "escape.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{{Name: "text", SourceNode: text}},
	})
	graph.EntryNode = entry
	assertLowerToBytecodeUnsupported(t, graph, entry, kind+" requires value")
}

func TestLowerToBytecodeHTMLEscapeRequiresValue(t *testing.T) {
	assertEscapeRequiresValue(t, "html_escape")
}

func TestLowerASTHTMLEscapeRejectsSiblings(t *testing.T) {
	assertLowerASTRejectsBytecode(t, "escape.howl", []astRejectCase{
		{name: "missing text", source: `(cli_app (print (html_escape)))`, kind: "html_escape"},
		{name: "extra text", source: `(cli_app (print (html_escape "a" "b")))`, kind: "html_escape"},
	}, "html_escape", "html_escape requires value")
}

func TestLowerToBytecodeAttrEscapeEmitsExistingOpcode(t *testing.T) {
	assertLowerSingleConstStringOp(t, "attr_escape", bytecode.OpAttrEscape, "ATTR_ESCAPE")
}

func TestLowerToBytecodeAttrEscapeRequiresValue(t *testing.T) {
	assertEscapeRequiresValue(t, "attr_escape")
}

func TestLowerASTAttrEscapeRejectsSiblings(t *testing.T) {
	assertLowerASTRejectsBytecode(t, "escape.howl", []astRejectCase{
		{name: "missing text", source: `(cli_app (print (attr_escape)))`, kind: "attr_escape"},
		{name: "extra text", source: `(cli_app (print (attr_escape "a" "b")))`, kind: "attr_escape"},
	}, "attr_escape", "attr_escape requires value")
}

func assertOpcodeSpec(t *testing.T, op bytecode.Opcode, name string, pops, pushes int) {
	t.Helper()
	spec := bytecode.Registry[op]
	if spec.Name != name || spec.Pops != pops || spec.Pushes != pushes || len(spec.Operands) != 0 || spec.Capability != "" {
		t.Fatalf("%s = %+v, want pops %d, pushes %d, no operands, no capability", name, spec, pops, pushes)
	}
}

func TestLowerToBytecodeRegexMatchEmitsExistingOpcode(t *testing.T) {
	assertOpcodeSpec(t, bytecode.OpRegexMatch, "REGEX_MATCH", 2, 1)
	graph := NewGraph()
	pattern := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "^a$"})
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a"})
	graph.EntryNode = graph.AddNode(&Node{
		Kind:       "regex_match",
		DataInputs: []DataEdge{{Name: "pattern", SourceNode: pattern}, {Name: "string", SourceNode: text}},
	})
	program := mustLowerAndValidateGraph(t, graph)
	if program.Main[0].Op != bytecode.OpLoadConst || program.Main[0].ValueOperand != "^a$" {
		t.Fatalf("pattern operand = %#v", program.Main[0])
	}
	if program.Main[1].Op != bytecode.OpLoadConst || program.Main[1].ValueOperand != "a" {
		t.Fatalf("string operand = %#v", program.Main[1])
	}
	assertSingleBareOpcode(t, program, 2, 3, bytecode.OpRegexMatch, "REGEX_MATCH", "")
}

func TestLowerToBytecodeRegexMatchRequiresPatternAndString(t *testing.T) {
	graph := NewGraph()
	pattern := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "^a$"})
	text := graph.AddNode(&Node{Kind: "const", LiteralKind: "STRING", Value: "a"})
	entry := graph.AddNode(&Node{
		Kind:       "regex_match",
		Provenance: Provenance{Filename: "match.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{{Name: "string", SourceNode: text}, {Name: "pattern", SourceNode: pattern}},
	})
	graph.EntryNode = entry
	assertLowerToBytecodeUnsupported(t, graph, entry, "regex_match requires pattern and string")
}

func TestLowerASTRegexMatchRejectsSiblings(t *testing.T) {
	assertLowerASTRejectsBytecode(t, "match.howl", []astRejectCase{
		{name: "missing args", source: `(cli_app (print (regex_match)))`, kind: "regex_match"},
		{name: "one arg", source: `(cli_app (print (regex_match "^a$")))`, kind: "regex_match"},
		{name: "extra arg", source: `(cli_app (print (regex_match "^a$" "a" "b")))`, kind: "regex_match"},
	}, "regex_match", "regex_match requires pattern and string")
}

func TestLowerToBytecodeTimeNowEmitsExistingOpcode(t *testing.T) {
	assertOpcodeSpec(t, bytecode.OpTimeNow, "TIME_NOW", 0, 1)
	graph := NewGraph()
	graph.EntryNode = graph.AddNode(&Node{Kind: "time_now"})
	program := mustLowerAndValidateGraph(t, graph)
	if program.Main[0].ValueOperand != nil {
		t.Fatalf("instruction = %#v, want bare TIME_NOW", program.Main[0])
	}
	assertSingleBareOpcode(t, program, 0, 1, bytecode.OpTimeNow, "TIME_NOW", "")
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
	assertLowerToBytecodeUnsupported(t, graph, entry, "time_now takes no arguments")
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
	assertLowerASTRejectsBytecode(t, "time.howl", []astRejectCase{{name: "arg", source: `(cli_app (print (time_now 1)))`}}, "", "time_now takes no arguments")
}

func TestLowerToBytecodeSleepEmitsExistingOpcode(t *testing.T) {
	assertOpcodeSpec(t, bytecode.OpSleep, "SLEEP", 1, 0)
	graph := NewGraph()
	duration := graph.AddNode(&Node{Kind: "const", LiteralKind: "INT", Value: "0"})
	graph.EntryNode = graph.AddNode(&Node{Kind: "sleep", DataInputs: []DataEdge{{Name: "", SourceNode: duration}}})
	program := mustLowerAndValidateGraph(t, graph)
	if len(program.Main) != 2 || program.Main[0].Op != bytecode.OpLoadConst || program.Main[0].ValueOperand != int64(0) {
		t.Fatalf("instructions = %#v, want const 0 then SLEEP", program.Main)
	}
	last := program.Main[1]
	if last.StringOperand2 != "" || last.StringOperand3 != "" || last.IntOperand2 != 0 || last.IntOperand3 != 0 || last.ValueOperand != nil {
		t.Fatalf("last instruction = %#v, want SLEEP with empty operands", last)
	}
	assertSingleBareOpcode(t, program, 1, 2, bytecode.OpSleep, "SLEEP", "")
}

func TestLowerToBytecodeSleepRejectsNamedEdge(t *testing.T) {
	graph := NewGraph()
	duration := graph.AddNode(&Node{Kind: "const", LiteralKind: "INT", Value: "0"})
	entry := graph.AddNode(&Node{
		Kind:       "sleep",
		Provenance: Provenance{Filename: "sleep.howl", Line: 2, Column: 3},
		DataInputs: []DataEdge{{Name: "duration", SourceNode: duration}},
	})
	graph.EntryNode = entry
	assertLowerToBytecodeUnsupported(t, graph, entry, "sleep requires one duration")
}

func TestLowerASTSleepKeepsDurationEdgeUnnamed(t *testing.T) {
	root := parser.NewParser(lexer.NewLexer(`(cli_app (sleep (+ 0 0)) (print "ok"))`), "sleep.howl").ParseExpression()
	graph, err := LowerAST(root, "sleep.howl")
	if err != nil {
		t.Fatal(err)
	}
	var found int
	for _, node := range graph.Nodes {
		if node.Kind != "sleep" {
			continue
		}
		found++
		if len(node.DataInputs) != 1 || node.DataInputs[0].Name != "" || len(node.ControlEdges) != 0 {
			t.Fatalf("sleep edges %#v control %v, want one unnamed duration", node.DataInputs, node.ControlEdges)
		}
	}
	if found != 1 {
		t.Fatalf("sleep nodes = %d, want 1", found)
	}
	program, diagnostics := LowerToBytecode(graph)
	if len(diagnostics) != 0 || program == nil {
		t.Fatalf("LowerToBytecode() program=%v diags=%#v", program != nil, diagnostics)
	}
	want := []bytecode.Opcode{bytecode.OpLoadConst, bytecode.OpLoadConst, bytecode.OpBinop, bytecode.OpSleep, bytecode.OpLoadConst, bytecode.OpPrint}
	if len(program.Main) != len(want) {
		t.Fatalf("instructions = %#v, want duration binop, SLEEP, then print", program.Main)
	}
	for index, op := range want {
		if program.Main[index].Op != op {
			t.Fatalf("instruction %d = %#v, want %v", index, program.Main[index], op)
		}
	}
	sleep := program.Main[3]
	if sleep.OpString != "SLEEP" || sleep.StringOperand != "" || sleep.IntOperand != 0 || sleep.ValueOperand != nil {
		t.Fatalf("SLEEP = %#v, want empty operands after the duration", sleep)
	}
}

func TestLowerASTSleepRejectsArity(t *testing.T) {
	assertLowerASTRejectsBytecode(t, "sleep.howl", []astRejectCase{
		{name: "missing duration", source: `(cli_app (sleep))`},
		{name: "extra duration", source: `(cli_app (sleep 0 1))`},
	}, "", "sleep requires one duration")
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
