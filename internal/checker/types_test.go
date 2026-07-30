package checker

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"zero/internal/ast"
	"zero/internal/ir"
	"zero/internal/lexer"
	"zero/internal/parser"
)

func parseTestProgram(t *testing.T, source string) *ast.Node {
	t.Helper()
	p := parser.NewParser(lexer.NewLexer(source), "types_test.zero")
	root := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		t.Fatalf("parser stopped at %s", p.Cur.Value)
	}
	return root
}

func TestAnalyzePropagatesTypesAndNativeLayout(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(defun add (a b)
			(type_hint a "int")
			(type_hint b "int")
			(type_hint return "int")
			(return (+ a b)))
		(let (items (list "a" "b"))
			(print (call add 1 2))))`)

	analysis := Analyze(root)
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", analysis.Diagnostics)
	}
	add := analysis.Functions["add"]
	if len(add.Params) != 2 || add.Params[0].Kind != ast.Int || add.Params[1].Kind != ast.Int {
		t.Fatalf("unexpected add signature: %+v", add)
	}
	if add.Return.Kind != ast.Int || add.Return.Size != 8 || add.Return.Align != 8 {
		t.Fatalf("unexpected return layout: %+v", add.Return)
	}
	var plus *ast.Node
	var findPlus func(*ast.Node)
	findPlus = func(node *ast.Node) {
		if node == nil {
			return
		}
		if node.Type == "List" && len(node.Children) > 0 && node.Children[0].Value == "+" {
			plus = node
		}
		for _, child := range node.Children {
			findPlus(child)
		}
	}
	findPlus(root)
	typedIR, ok := ir.LowerShared(plus)
	if !ok || typedIR.Type.Kind != ast.Int || typedIR.Type.Size != 8 {
		t.Fatalf("IR did not preserve inferred layout: ok=%v ir=%+v", ok, typedIR)
	}

	var listType ast.TypeInfo
	var visit func(*ast.Node)
	visit = func(node *ast.Node) {
		if node == nil {
			return
		}
		if node.Type == "List" && len(node.Children) > 0 && node.Children[0].Value == "list" {
			listType = node.Inferred
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(root)
	if listType.Kind != ast.List || listType.Element == nil || listType.Element.Kind != ast.String {
		t.Fatalf("unexpected list inference: %+v", listType)
	}
	if !listType.Pointer || listType.Size != 24 || listType.Align != 8 {
		t.Fatalf("unexpected list layout: %+v", listType)
	}

	dictRoot := parseTestProgram(t, `(cli_app (dict ("answer" 42)))`)
	dictAnalysis := Analyze(dictRoot)
	dictType := dictRoot.Children[1].Inferred
	if len(dictAnalysis.Diagnostics) != 0 || dictType.Kind != ast.Dict ||
		dictType.Key == nil || dictType.Key.Kind != ast.String ||
		dictType.Element == nil || dictType.Element.Kind != ast.Int {
		t.Fatalf("unexpected dictionary layout: diagnostics=%+v type=%+v", dictAnalysis.Diagnostics, dictType)
	}
}

func TestAnalyzeReportsProvableTypeErrors(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(defun bad () int (return "not an int"))
		(if 1 (print "never")))`)

	analysis := Analyze(root)
	if len(analysis.Diagnostics) != 2 {
		t.Fatalf("expected two diagnostics, got %+v", analysis.Diagnostics)
	}
	if analysis.Diagnostics[0].Line == 0 || analysis.Diagnostics[1].Line == 0 {
		t.Fatalf("diagnostics must retain source locations: %+v", analysis.Diagnostics)
	}
}

func TestAnalyzePropagatesDeferredControlFlow(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(let (items (list "a" "b"))
			(for item items (print item)))
		(try_let (n (to_int "1"))
			(catch err (print err))
			(if (> n 0) (print n) (print "no")))
		(spawn (lambda () (print "worker")))
		(match 1 (1 (print "one")) (default (print "other"))))`)

	analysis := Analyze(root)
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", analysis.Diagnostics)
	}
	var loopItem ast.TypeInfo
	var visit func(*ast.Node)
	visit = func(node *ast.Node) {
		if node == nil {
			return
		}
		if node.Type == "SYMBOL" && node.Value == "item" {
			loopItem = node.Inferred
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(root)
	if loopItem.Kind != ast.String || loopItem.Size != 16 || !loopItem.Pointer {
		t.Fatalf("for-loop element lost its inferred layout: %+v", loopItem)
	}
}

func TestAnalyzeBackendSpecificLayoutsAndFields(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(struct User (name string) (age int))
		(let (user (parse_json User payload))
			(let (age user.age)
				(let (token (env "TOKEN"))
					(let (raw (read_file "data.txt"))
						(print age token raw))))))`)

	analysis := Analyze(root)
	if len(analysis.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", analysis.Diagnostics)
	}
	user := analysis.Structs["User"]
	if user.Kind != ast.Struct || user.Size != 24 || user.Align != 8 || len(user.Fields) != 4 || user.FieldOffsets["name"] != 0 || user.FieldOffsets["age"] != 16 {
		t.Fatalf("unexpected User layout: %+v", user)
	}
	if user.Fields["age"].Kind != ast.Int || user.Fields["name"].Kind != ast.String {
		t.Fatalf("unexpected User fields: %+v", user.Fields)
	}

	var age, token, raw ast.TypeInfo
	var visit func(*ast.Node)
	visit = func(node *ast.Node) {
		if node == nil {
			return
		}
		switch node.Value {
		case "user.age":
			age = node.Inferred
		case "token":
			token = node.Inferred
		case "raw":
			raw = node.Inferred
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(root)
	if age.Kind != ast.Int || token.Kind != ast.String || raw.Kind != ast.Bytes {
		t.Fatalf("backend-specific types were not propagated: age=%+v token=%+v raw=%+v", age, token, raw)
	}
}

func TestAnalyzeSchemaBridgeConstrainsDeclaredStructAndVisitsSource(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(struct Profile (name string) (score int))
		(schema_bridge Profile (+ true false)))`)

	analysis := Analyze(root)
	if len(analysis.Bridges) != 1 {
		t.Fatalf("bridges = %+v, want one bridge", analysis.Bridges)
	}
	bridge := analysis.Bridges[0]
	if bridge.Target != "Profile" || bridge.Constraint.Kind != ast.Struct || bridge.Constraint.Name != "Profile" ||
		bridge.Constraint.Fields["score"].Kind != ast.Int {
		t.Fatalf("unexpected schema bridge: %+v", bridge)
	}
	bridgeNode := root.Children[2]
	if bridgeNode.Inferred.Kind != ast.Struct || bridgeNode.Inferred.Name != "Profile" {
		t.Fatalf("bridge inference = %+v, want Profile struct", bridgeNode.Inferred)
	}

	foundNestedError := false
	for _, diagnostic := range analysis.Diagnostics {
		if diagnostic.Reason == "+ requires numeric operands, got bool and bool" {
			foundNestedError = true
		}
	}
	if !foundNestedError {
		t.Fatalf("schema bridge did not visit source expression: %+v", analysis.Diagnostics)
	}
}

func TestAnalyzeSchemaBridgeRejectsUnknownTargetAtTargetLocation(t *testing.T) {
	root := parseTestProgram(t, `(cli_app (schema_bridge Missing (to_int "1")))`)
	analysis := Analyze(root)
	if len(analysis.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want unknown-target diagnostic", analysis.Diagnostics)
	}
	diagnostic := analysis.Diagnostics[0]
	if diagnostic.Reason != `schema_bridge target "Missing" is not a declared struct` {
		t.Fatalf("unexpected diagnostic: %+v", diagnostic)
	}
	target := root.Children[1].Children[1]
	if diagnostic.Line != target.Line || diagnostic.Column != target.Column {
		t.Fatalf("diagnostic location = %d:%d, want target location %d:%d", diagnostic.Line, diagnostic.Column, target.Line, target.Column)
	}
	if root.Children[1].Children[2].Inferred.Kind != ast.Int {
		t.Fatalf("unknown-target bridge did not infer source: %+v", root.Children[1].Children[2].Inferred)
	}
}

func TestAnalyzeRejectsInconsistentAggregateLayouts(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(let (items (list 1 "two"))
			(list_get items "zero"))
		(let (values (dict ("one" 1) ("two" "two")))
			(map_get values 2)))`)

	analysis := Analyze(root)
	reasons := make([]string, 0, len(analysis.Diagnostics))
	for _, diagnostic := range analysis.Diagnostics {
		reasons = append(reasons, diagnostic.Reason)
	}
	for _, expected := range []string{
		"list element 2 has type string, want int",
		"list_get index must be int, got string",
		"dict value 2 has type string, want int",
		"map_get key must be string, got int",
	} {
		found := false
		for _, reason := range reasons {
			if reason == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing diagnostic %q in %+v", expected, reasons)
		}
	}
}

func TestAnalyzeRejectsInvalidAggregateMutationTypes(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(let (items (list "one"))
			(do
				(append items 2)
				(map_set items "key" "value")))
		(let (values (dict ("one" "1")))
			(do
				(map_set values 2 "two")
				(map_set values "two" 2)
				(append values "three"))))`)

	analysis := Analyze(root)
	reasons := make([]string, 0, len(analysis.Diagnostics))
	for _, diagnostic := range analysis.Diagnostics {
		reasons = append(reasons, diagnostic.Reason)
	}
	for _, expected := range []string{
		"append item has type int, want string",
		"map_set target must be dict, got list",
		"map_set key must be string, got int",
		"map_set value has type int, want string",
		"append target must be list, got dict",
	} {
		found := false
		for _, reason := range reasons {
			if reason == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing diagnostic %q in %+v", expected, reasons)
		}
	}
}

func TestAnalyzeRejectsIncompatibleBranchAndCallLayouts(t *testing.T) {
	root := parseTestProgram(t, `(cli_app
		(defun choose ((value int)) int (return value))
		(call choose)
		(call choose 1 2)
		(if true 1 (to_float 2))
		(+ true false)
		(+ 1 (to_float 2)))`)

	analysis := Analyze(root)
	reasons := make([]string, 0, len(analysis.Diagnostics))
	for _, diagnostic := range analysis.Diagnostics {
		reasons = append(reasons, diagnostic.Reason)
	}
	for _, expected := range []string{
		`function "choose" expects 1 argument, got 0`,
		`function "choose" expects 1 argument, got 2`,
		"if branches have incompatible types int and float64",
		"+ requires numeric operands, got bool and bool",
		"+ requires matching numeric types, got int and float64",
	} {
		found := false
		for _, reason := range reasons {
			if reason == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing diagnostic %q in %+v", expected, reasons)
		}
	}
}

func TestAnalyzeReportsMalformedSharedForms(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{`(cli_app (return))`, "return expects 1 argument"},
		{`(cli_app (if true))`, "if expects 2 or 3 arguments"},
		{`(cli_app (while true))`, "while expects 2 arguments"},
		{`(cli_app (set value))`, "set expects 2 arguments"},
		{`(cli_app (match value))`, "match expects a value and at least one case"},
		{`(cli_app (match value (default)))`, "match cases expect a label and body"},
		{`(cli_app (sleep))`, "sleep expects 1 argument"},
		{`(cli_app (to_int))`, "to_int expects 1 argument"},
		{`(cli_app (to_float))`, "to_float expects 1 argument"},
		{`(cli_app (to_string))`, "to_string expects 1 argument"},
		{`(cli_app (bytes_to_string))`, "bytes_to_string expects 1 argument"},
		{`(cli_app (str_split value))`, "str_split expects 2 arguments"},
		{`(cli_app (str_join values))`, "str_join expects 2 arguments"},
		{`(cli_app (regex_match pattern))`, "regex_match expects 2 arguments"},
		{`(cli_app (append values))`, "append expects 2 arguments"},
		{`(cli_app (map_set values key))`, "map_set expects 3 arguments"},
		{`(cli_app (map_delete values))`, "map_delete expects 2 arguments"},
		{`(cli_app (map_get values))`, "map_get expects 2 arguments"},
		{`(cli_app (list_get values))`, "list_get expects 2 arguments"},
		{`(cli_app (+ 1))`, "+ expects 2 arguments"},
		{`(cli_app (call))`, "call expects at least a function name"},
		{`(cli_app (let))`, "let expects 2 arguments"},
		{`(cli_app (let value body))`, "let binding expects (var val)"},
		{`(cli_app (try_let (value 1) (catch err (print err))))`, "try_let expects 3 arguments"},
		{`(cli_app (try_let value (catch err (print err)) (print value)))`, "try_let binding expects (var val)"},
		{`(cli_app (try_let (value 1) err (print value)))`, "try_let expects (catch err body)"},
		{`(cli_app (spawn))`, "spawn expects 1 argument"},
		{`(cli_app (spawn worker))`, "spawn expects a lambda"},
		{`(cli_app (for item items))`, "for expects 3 arguments"},
		{`(cli_app (dict ("ok" "value") ("bad")))`, "dict expects (k v) pairs"},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			analysis := Analyze(parseTestProgram(t, test.source))
			if len(analysis.Diagnostics) == 0 {
				t.Fatalf("expected diagnostic containing %q", test.want)
			}
			if !strings.Contains(analysis.Diagnostics[0].Reason, test.want) {
				t.Fatalf("diagnostic %q does not contain %q", analysis.Diagnostics[0].Reason, test.want)
			}
			if analysis.Diagnostics[0].Line == 0 || analysis.Diagnostics[0].Column == 0 {
				t.Fatalf("diagnostic lacks source location: %+v", analysis.Diagnostics[0])
			}
		})
	}
}

func TestCheckReportsMalformedSharedForm(t *testing.T) {
	if os.Getenv("ZERO_CHECKER_HELPER") == "1" {
		Check(parseTestProgram(t, `(cli_app (return))`))
		return
	}

	command := exec.Command(os.Args[0], "-test.run=^TestCheckReportsMalformedSharedForm$")
	command.Env = append(os.Environ(), "ZERO_CHECKER_HELPER=1")
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("checker accepted malformed return")
	}
	exitError, ok := err.(*exec.ExitError)
	if !ok || exitError.ExitCode() != 1 {
		t.Fatalf("checker failed unexpectedly: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), `{"reason":"return expects 1 argument, got 0","line":1,"column":10}`) {
		t.Fatalf("checker did not emit the expected diagnostic: %s", output)
	}
}
