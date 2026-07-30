package masking

import (
	"encoding/json"
	"testing"
	"zero/internal/ast"
	"zero/internal/checker"
	"zero/internal/lexer"
	"zero/internal/parser"
)

func TestCompileTypePrimitiveConstraints(t *testing.T) {
	tests := []struct {
		kind         ast.ValueKind
		tokenClasses []string
		literals     []string
		encoding     string
	}{
		{kind: ast.Bool, tokenClasses: []string{"boolean"}, literals: []string{"false", "true"}},
		{kind: ast.Int, tokenClasses: []string{"integer"}},
		{kind: ast.Float, tokenClasses: []string{"number"}},
		{kind: ast.String, tokenClasses: []string{"string"}},
		{kind: ast.Bytes, tokenClasses: []string{"string"}, encoding: "bytes"},
		{kind: ast.Void, tokenClasses: []string{"empty"}},
		{kind: ast.Any, tokenClasses: unconstrainedTokenClasses},
		{kind: ast.Unknown, tokenClasses: unconstrainedTokenClasses},
	}

	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			plan := CompileType(ast.Layout(test.kind))
			assertStringsEqual(t, plan.TokenClasses, test.tokenClasses)
			assertStringsEqual(t, plan.Literals, test.literals)
			if plan.Encoding != test.encoding {
				t.Fatalf("encoding = %q, want %q", plan.Encoding, test.encoding)
			}
		})
	}
}

func TestCompileTypeListAndDictConstraints(t *testing.T) {
	integer := ast.Layout(ast.Int)
	list := ast.Layout(ast.List)
	list.Element = &integer

	listPlan := CompileType(list)
	if listPlan.Delimiters == nil || listPlan.Delimiters.Open != "[" ||
		listPlan.Delimiters.Close != "]" || listPlan.Delimiters.ItemSeparator != "," {
		t.Fatalf("unexpected list delimiters: %+v", listPlan.Delimiters)
	}
	if listPlan.Element == nil || listPlan.Element.Kind != ast.Int {
		t.Fatalf("unexpected list element constraint: %+v", listPlan.Element)
	}

	key := ast.Layout(ast.String)
	boolean := ast.Layout(ast.Bool)
	dict := ast.Layout(ast.Dict)
	dict.Key = &key
	dict.Element = &boolean

	dictPlan := CompileType(dict)
	if dictPlan.Delimiters == nil || dictPlan.Delimiters.Open != "{" ||
		dictPlan.Delimiters.Close != "}" || dictPlan.Delimiters.ItemSeparator != "," ||
		dictPlan.Delimiters.KeyValueSeparator != ":" {
		t.Fatalf("unexpected dict delimiters: %+v", dictPlan.Delimiters)
	}
	if dictPlan.Key == nil || dictPlan.Key.Kind != ast.String {
		t.Fatalf("unexpected dict key constraint: %+v", dictPlan.Key)
	}
	if dictPlan.Element == nil || dictPlan.Element.Kind != ast.Bool {
		t.Fatalf("unexpected dict element constraint: %+v", dictPlan.Element)
	}
}

func TestCompileTypeRecursiveConstraintFallsBackToUnknown(t *testing.T) {
	list := ast.Layout(ast.List)
	list.Element = &list

	plan := CompileType(list)
	if plan.Element == nil || plan.Element.Element == nil || plan.Element.Element.Kind != ast.Unknown {
		t.Fatalf("recursive edge did not fall back to unknown: %+v", plan.Element)
	}
	assertStringsEqual(t, plan.Element.Element.TokenClasses, unconstrainedTokenClasses)
}

func TestCompileTypeStructFieldsAreStable(t *testing.T) {
	info := ast.Layout(ast.Struct)
	info.Name = "User"
	info.Fields = map[string]ast.TypeInfo{
		"active": ast.Layout(ast.Bool),
		"name":   ast.Layout(ast.String),
	}

	plan := CompileType(info)
	if len(plan.Fields) != 2 {
		t.Fatalf("field count = %d, want 2: %+v", len(plan.Fields), plan.Fields)
	}
	if plan.Fields[0].Name != "active" || plan.Fields[0].Constraint.Kind != ast.Bool ||
		plan.Fields[1].Name != "name" || plan.Fields[1].Constraint.Kind != ast.String {
		t.Fatalf("unexpected ordered fields: %+v", plan.Fields)
	}
	for _, field := range plan.Fields {
		if !field.Required {
			t.Fatalf("field %q must be required", field.Name)
		}
	}
}

func TestCompileTypePreservesDistinctFieldsWithoutCheckerAliases(t *testing.T) {
	info := ast.Layout(ast.Struct)
	info.Fields = map[string]ast.TypeInfo{
		"Name": ast.Layout(ast.String),
		"name": ast.Layout(ast.String),
	}

	plan := CompileType(info)
	if len(plan.Fields) != 2 || plan.Fields[0].Name != "Name" || plan.Fields[1].Name != "name" {
		t.Fatalf("distinct fields were treated as checker aliases: %+v", plan.Fields)
	}
}

func TestCompileAnalysisProducesStableProgramPlan(t *testing.T) {
	root := parseProgram(t, `(cli_app
		(struct User (name string) (age int))
		(defun describe ((user User) (verbose bool))
			(type_hint return "string")
			(return user.name)))`)
	analysis := checker.Analyze(root)
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", analysis.Diagnostics)
	}

	plan := CompileAnalysis(analysis)
	if plan.Format != FormatV1 {
		t.Fatalf("format = %q, want %q", plan.Format, FormatV1)
	}
	if len(plan.Structs) != 1 || plan.Structs[0].Name != "User" {
		t.Fatalf("unexpected structs: %+v", plan.Structs)
	}
	fields := plan.Structs[0].Constraint.Fields
	if len(fields) != 2 || fields[0].Name != "age" || fields[1].Name != "name" {
		t.Fatalf("checker field aliases were not normalized: %+v", fields)
	}
	if len(plan.Functions) != 1 || plan.Functions[0].Name != "describe" {
		t.Fatalf("unexpected functions: %+v", plan.Functions)
	}
	function := plan.Functions[0]
	if len(function.Params) != 2 || function.Params[0].Index != 0 ||
		function.Params[0].Constraint.Name != "User" ||
		function.Params[1].Constraint.Kind != ast.Bool ||
		function.Return.Kind != ast.String {
		t.Fatalf("unexpected function constraint: %+v", function)
	}

	first, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal mask plan: %v", err)
	}
	second, err := json.Marshal(CompileAnalysis(analysis))
	if err != nil {
		t.Fatalf("marshal second mask plan: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("mask plan JSON is not deterministic:\n%s\n%s", first, second)
	}
}

func TestCompileAnalysisAcceptsNil(t *testing.T) {
	plan := CompileAnalysis(nil)
	if plan.Format != FormatV1 || len(plan.Structs) != 0 || len(plan.Functions) != 0 {
		t.Fatalf("unexpected nil analysis plan: %+v", plan)
	}
}

func parseProgram(t *testing.T, source string) *ast.Node {
	t.Helper()
	p := parser.NewParser(lexer.NewLexer(source), "masking_test.zero")
	root := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		t.Fatalf("parser stopped at %s", p.Cur.Value)
	}
	return root
}

func assertStringsEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
