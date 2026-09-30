package main

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/construct"
	"github.com/howlcipher/howlframe/internal/hfir"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

// TestProdFlipCriteriaLock holds the documented prod-flip checklist to the
// compilers. The checklist is docs/reference/lowered_hfir_prod_flip_criteria.md.
// Production -compile-bc stays on bytecode.CompileToBytecode.
func TestProdFlipCriteriaLock(t *testing.T) {
	source := readRepoFile(t, "howlframe.go")
	prod := stripLineComments(sliceBetween(t, source, "if *compileBc {", "if *compileHfirBc {"))
	if !strings.Contains(prod, "bytecode.CompileToBytecode(root)") {
		t.Fatal("production -compile-bc branch lost bytecode.CompileToBytecode(root)")
	}
	if strings.Contains(prod, "LowerToBytecode") {
		t.Fatal("production -compile-bc branch calls LowerToBytecode")
	}
	experimental := stripLineComments(sliceBetween(t, source, "if *compileHfirBc {", "if *compileWasm {"))
	if !strings.Contains(experimental, "hfir.LowerToBytecode(graph)") {
		t.Fatal("experimental -compile-hfir-bc branch lost hfir.LowerToBytecode(graph)")
	}

	criteria := readRepoFile(t, "docs/reference/lowered_hfir_prod_flip_criteria.md")
	for _, phrase := range []string{
		"#90 stays Partial",
		"## Decision",
		"## Promote",
		"## Kill",
		"## Defer",
		"## Dogfood path",
		"Assurance tip-lock",
		"HFIR_BYTECODE_UNSUPPORTED",
		"OpFetch",
		"model-adapter",
		"-compile-hfir-bc",
		"until the Owner authorizes a flip PR",
	} {
		if !strings.Contains(criteria, phrase) {
			t.Errorf("criteria doc missing %q", phrase)
		}
	}
	if got, want := blockerFence(t, criteria), promoteBlockers(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("promote-blocker fence = %v\ncomputed = %v", got, want)
	}

	fetch := bytecode.Registry[bytecode.OpFetch]
	if fetch.Name != "FETCH" || fetch.Pops != 2 || len(fetch.Operands) != 0 || fetch.Capability != capability.Network {
		t.Fatalf("OpFetch = %+v, want FETCH popping 2 with no operands and network", fetch)
	}
	if !reflect.DeepEqual(hfir.WasmInfeasibleKinds, []string{"exec", "spawn_agent", "http_server_start"}) {
		t.Fatalf("WasmInfeasibleKinds = %v", hfir.WasmInfeasibleKinds)
	}

	for _, kind := range []string{
		"defun", "call", "return", "while", "for",
		"read_file", "write_file", "mkdir", "exec", "fetch",
		"regex_match", "html_escape", "attr_escape", "match", "try",
	} {
		kind := kind
		t.Run("transport/"+kind, func(t *testing.T) {
			candidate, diags := hfir.DecodeCandidate(transportRejecting(kind))
			if candidate.Graph != nil || len(diags) != 1 || diags[0].Code != "HFIR_TRANSPORT_KIND" {
				t.Fatalf("DecodeCandidate(%s) = (%v, %#v), want HFIR_TRANSPORT_KIND", kind, candidate.Graph != nil, diags)
			}
		})
	}

	for _, sample := range []struct {
		name, source string
		op           bytecode.Opcode
	}{
		{"regex_match", `(cli_app (print (regex_match "^a$" "a")))`, bytecode.OpRegexMatch},
		{"attr_escape", `(cli_app (print (attr_escape "a<b")))`, bytecode.OpAttrEscape},
	} {
		sample := sample
		t.Run(sample.name, func(t *testing.T) {
			graph := mustGraph(t, sample.source)
			program, diags := hfir.LowerToBytecode(graph)
			if program != nil || len(diags) != 1 || diags[0].Code != hfir.BytecodeUnsupportedCode {
				t.Fatalf("LowerToBytecode() program=%v diags=%#v, want one %s", program != nil, diags, hfir.BytecodeUnsupportedCode)
			}
			astProgram := bytecode.CompileToBytecode(mustAST(t, sample.source))
			if !programHasOpcode(astProgram, sample.op) {
				t.Fatalf("production bytecode missing %s", bytecode.Registry[sample.op].Name)
			}
		})
	}

	t.Run("html_escape", func(t *testing.T) {
		const source = `(cli_app (print (html_escape "a<b")))`
		graph := mustGraph(t, source)
		program, diags := hfir.LowerToBytecode(graph)
		if len(diags) != 0 || program == nil || !programHasOpcode(program, bytecode.OpHTMLEscape) {
			t.Fatalf("LowerToBytecode() program=%v diags=%#v, want HTML_ESCAPE", program != nil, diags)
		}
		astProgram := bytecode.CompileToBytecode(mustAST(t, source))
		if !programHasOpcode(astProgram, bytecode.OpHTMLEscape) {
			t.Fatal("production bytecode missing HTML_ESCAPE")
		}
	})

	t.Run("fetch body", func(t *testing.T) {
		const body = "body-not-sent"
		source := `(cli_app (print (fetch "http://127.0.0.1/howlframe-flip" "PUT" "` + body + `")))`
		graph := mustGraph(t, source)
		lowered, diags := hfir.LowerToBytecode(graph)
		if len(diags) != 0 || lowered == nil {
			t.Fatalf("LowerToBytecode() diags=%#v program=%v", diags, lowered != nil)
		}
		astProgram := bytecode.CompileToBytecode(mustAST(t, source))
		for _, program := range []*bytecode.BCProgram{lowered, astProgram} {
			if !programHasOpcode(program, bytecode.OpFetch) {
				t.Fatal("fetch program missing FETCH")
			}
			if strings.Contains(programText(program), body) {
				t.Fatal("fetch artifact contains the body string")
			}
		}
	})

	t.Run("try control edges", func(t *testing.T) {
		source := `(cli_app (try_let (content (read_file "/tmp/howlframe-flip-missing")) (catch err (print err)) (print content)))`
		graph := mustGraph(t, source)
		found := false
		for _, node := range graph.Nodes {
			if node.Kind != "try" {
				continue
			}
			found = true
			if len(node.ControlEdges) != 0 {
				t.Fatalf("try ControlEdges = %#v, want empty", node.ControlEdges)
			}
		}
		if !found {
			t.Fatal("graph has no try node")
		}
		program, diags := hfir.LowerToBytecode(graph)
		if len(diags) != 0 || program == nil || !programHasOpcode(program, bytecode.OpTryLet) {
			t.Fatalf("LowerToBytecode() program=%v diags=%#v, want TRY_LET", program != nil, diags)
		}
	})

	t.Run("match control edges", func(t *testing.T) {
		entry, ok := construct.Lookup("match")
		if !ok || entry.Support != construct.Unsupported {
			t.Fatalf("match support = %+v, want Unsupported", entry)
		}
		source := `(cli_app (let (x 0) (match x (0 (print "zero")) (default (print "other")))))`
		graph := mustGraph(t, source)
		found := false
		for _, node := range graph.Nodes {
			if node.Kind != "match" {
				continue
			}
			found = true
			if len(node.ControlEdges) != 0 {
				t.Fatalf("match ControlEdges = %#v, want empty", node.ControlEdges)
			}
		}
		if !found {
			t.Fatal("graph has no match node")
		}
		program, diags := hfir.LowerToBytecode(graph)
		if program != nil || len(diags) != 1 || diags[0].Code != hfir.BytecodeUnsupportedCode {
			t.Fatalf("LowerToBytecode() program=%v diags=%#v, want one %s", program != nil, diags, hfir.BytecodeUnsupportedCode)
		}
	})
}

// Names the AST compiler spells that LowerToBytecode already emits under
// another kind. The criteria doc records the same mapping.
var hfirKindAliases = map[string]string{
	"cli_app":         "program",
	"do":              "sequence",
	"try_let":         "try",
	"+":               "binary",
	"-":               "binary",
	"*":               "binary",
	"/":               "binary",
	"<":               "binary",
	">":               "binary",
	"<=":              "binary",
	">=":              "binary",
	"==":              "binary",
	"!=":              "binary",
	"=":               "binary",
	"and":             "binary",
	"or":              "binary",
	"to_int":          "convert",
	"to_float":        "convert",
	"to_string":       "convert",
	"bytes_to_string": "convert",
	"encode_json":     "convert",
}

func promoteBlockers(t *testing.T) []string {
	t.Helper()
	astCases := switchCases(t, "internal/bytecode/bytecode.go")
	hfirCases := switchCases(t, "internal/hfir/bytecode.go")
	var blockers []string
	for _, entry := range construct.Table() {
		if entry.Support != construct.Supported {
			continue
		}
		if _, ok := hfirCases[entry.Name]; ok {
			continue
		}
		if alias, ok := hfirKindAliases[entry.Name]; ok {
			if _, covered := hfirCases[alias]; covered {
				continue
			}
		}
		if _, ok := astCases[entry.Name]; !ok {
			t.Fatalf("supported construct %q has no compileNode case", entry.Name)
		}
		blockers = append(blockers, entry.Name)
	}
	return blockers
}

func blockerFence(t *testing.T, criteria string) []string {
	t.Helper()
	const start = "```promote-blockers\n"
	i := strings.Index(criteria, start)
	if i < 0 {
		t.Fatal("criteria doc has no promote-blockers fence")
	}
	rest := criteria[i+len(start):]
	j := strings.Index(rest, "\n```")
	if j < 0 {
		t.Fatal("promote-blockers fence is unclosed")
	}
	var names []string
	for _, line := range strings.Split(rest[:j], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		names = append(names, line)
	}
	return names
}

func switchCases(t *testing.T, path string) map[string]bool {
	t.Helper()
	text := readRepoFile(t, path)
	cases := map[string]bool{}
	quoted := regexp.MustCompile(`"([^"]+)"`)
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "case ") {
			continue
		}
		for _, match := range quoted.FindAllStringSubmatch(trimmed, -1) {
			cases[match[1]] = true
		}
	}
	if len(cases) == 0 {
		t.Fatalf("no switch cases in %s", path)
	}
	return cases
}

func transportRejecting(kind string) []byte {
	payload := map[string]any{
		"schema_version": hfir.ModelAdapterSchemaVersion,
		"graph_version":  "v1",
		"entry_node":     "program",
		"nodes": []map[string]any{
			{"id": "program", "kind": "program", "value": "", "inputs": []map[string]string{{"role": "body", "node_id": "print"}}, "provenance": map[string]string{"label": "black-box"}},
			{"id": "print", "kind": kind, "value": "", "inputs": []map[string]string{{"role": "value", "node_id": "value"}}, "provenance": map[string]string{"label": "black-box"}},
			{"id": "value", "kind": "const", "value": "ok", "literal_kind": "STRING", "provenance": map[string]string{"label": "black-box"}},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return encoded
}

func mustAST(t *testing.T, source string) *ast.Node {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(source), "prod_flip.howl").ParseExpression()
	checker.Check(root)
	return root
}

func mustGraph(t *testing.T, source string) *hfir.Graph {
	t.Helper()
	graph, err := hfir.LowerAST(mustAST(t, source), "prod_flip.howl")
	if err != nil {
		t.Fatalf("LowerAST() error = %v", err)
	}
	return graph
}

func programHasOpcode(program *bytecode.BCProgram, op bytecode.Opcode) bool {
	if program == nil {
		return false
	}
	for _, inst := range program.Main {
		if inst.Op == op {
			return true
		}
	}
	for _, fn := range program.Functions {
		for _, inst := range fn.Instructions {
			if inst.Op == op {
				return true
			}
		}
	}
	return false
}

func programText(program *bytecode.BCProgram) string {
	var buf bytes.Buffer
	writeInst := func(inst bytecode.BCInstruction) {
		buf.WriteString(inst.StringOperand)
		buf.WriteByte(0)
		buf.WriteString(inst.StringOperand2)
		buf.WriteByte(0)
		buf.WriteString(inst.StringOperand3)
		if text, ok := inst.ValueOperand.(string); ok {
			buf.WriteString(text)
		}
	}
	for _, inst := range program.Main {
		writeInst(inst)
	}
	for _, fn := range program.Functions {
		for _, inst := range fn.Instructions {
			writeInst(inst)
		}
	}
	return buf.String()
}

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func stripLineComments(source string) string {
	var b strings.Builder
	for _, line := range strings.Split(source, "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func sliceBetween(t *testing.T, source, start, end string) string {
	t.Helper()
	i := strings.Index(source, start)
	if i < 0 {
		t.Fatalf("missing %q", start)
	}
	rest := source[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		t.Fatalf("missing %q after %q", end, start)
	}
	return rest[:j]
}
