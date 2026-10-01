package checker

import (
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
)

func analyzeExpr(t *testing.T, expr string) *Analysis {
	t.Helper()
	return Analyze(parseTestProgram(t, "(cli_app "+expr+")"))
}

// inferredResult returns the type the checker infers for a single top-level
// expression, via a let binding that is read back from the analysis.
func inferredKind(t *testing.T, expr string) (ast.ValueKind, []Diagnostic) {
	t.Helper()
	root := parseTestProgram(t, "(cli_app (let (v "+expr+") v))")
	analysis := Analyze(root)
	var kind ast.ValueKind
	var walk func(n *ast.Node)
	walk = func(n *ast.Node) {
		if n.Type == "List" && len(n.Children) > 0 && n.Children[0].Value == "let" {
			kind = analysis.Types[n].Kind
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	return kind, analysis.Diagnostics
}

// TestNumericContractArithmeticTyping covers NUMERIC_CONTRACT sections 4 and 5
// (HFREC-063): mixed int/float arithmetic is valid and `/` always yields float.
func TestNumericContractArithmeticTyping(t *testing.T) {
	tests := []struct {
		expr string
		want ast.ValueKind
	}{
		{"(+ 1 2)", ast.Int}, {"(+ 1 2.5)", ast.Float}, {"(+ 1.5 2)", ast.Float}, {"(+ 1.5 2.5)", ast.Float},
		{"(- 1 2)", ast.Int}, {"(- 1 2.5)", ast.Float}, {"(- 1.5 2)", ast.Float}, {"(- 1.5 2.5)", ast.Float},
		{"(* 1 2)", ast.Int}, {"(* 1 2.5)", ast.Float}, {"(* 1.5 2)", ast.Float}, {"(* 1.5 2.5)", ast.Float},
		{"(/ 9 2)", ast.Float}, {"(/ 9 2.0)", ast.Float}, {"(/ 9.0 2)", ast.Float}, {"(/ 9.0 2.0)", ast.Float},
		{`(+ "a" "b")`, ast.String},
	}
	for _, tc := range tests {
		t.Run(tc.expr, func(t *testing.T) {
			got, diags := inferredKind(t, tc.expr)
			if len(diags) != 0 {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if got != tc.want {
				t.Fatalf("%s inferred %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

func TestNumericContractRejectsInvalidOperands(t *testing.T) {
	for _, expr := range []string{
		`(+ 1 "a")`, `(+ "a" 1)`, `(+ 1.5 "a")`, `(- "a" "b")`, `(* "a" 2)`, `(/ "a" 2)`,
		`(/ 2 "a")`, `(+ true 1)`, `(/ true false)`,
	} {
		t.Run(expr, func(t *testing.T) {
			analysis := analyzeExpr(t, expr)
			if len(analysis.Diagnostics) == 0 {
				t.Fatalf("%s was accepted", expr)
			}
		})
	}
}

// TestNumericContractComparisons keeps static comparison checks aligned with
// the runtime's exact int/float equality and ordering (sections 6.1 and 6.2).
func TestNumericContractComparisons(t *testing.T) {
	for _, op := range []string{"=", "==", "!=", "<", ">", "<=", ">="} {
		for _, pair := range []string{"1 1", "1 1.0", "1.0 1", "1.5 2.5"} {
			if d := analyzeExpr(t, "("+op+" "+pair+")").Diagnostics; len(d) != 0 {
				t.Errorf("(%s %s) rejected: %v", op, pair, d)
			}
		}
		for _, pair := range []string{`1 "a"`, `1.5 "a"`, `true 1`} {
			if d := analyzeExpr(t, "("+op+" "+pair+")").Diagnostics; len(d) == 0 {
				t.Errorf("(%s %s) was accepted", op, pair)
			}
		}
	}
}

// TestVoidInValuePositionIsRejected covers HFREC-009: void expressions must
// fail in the checker instead of underflowing the VM stack.
func TestVoidInValuePositionIsRejected(t *testing.T) {
	invalid := map[string]string{
		"let initializer":   `(let (items (list)) (let (copy (append items 6)) (print copy)))`,
		"function argument": `(defun take ((v int)) int (return v)) (let (items (list)) (print (call take (append items 6))))`,
		"direct argument":   `(defun take ((v int)) int (return v)) (let (items (list)) (print (take (append items 6))))`,
		"arithmetic":        `(let (items (list)) (print (+ 1 (append items 6))))`,
		"print argument":    `(let (items (list)) (print (append items 6)))`,
		"set value":         `(let (a 1) (let (items (list)) (set a (append items 6))))`,
		"list element":      `(let (items (list)) (print (list (append items 1))))`,
		"void function":     `(defun noop () void (print "x")) (let (v (call noop)) (print v))`,
	}
	for name, src := range invalid {
		t.Run(name, func(t *testing.T) {
			analysis := analyzeExpr(t, src)
			found := false
			for _, d := range analysis.Diagnostics {
				if strings.Contains(d.Reason, "void expression") {
					found = true
				}
			}
			if !found {
				t.Fatalf("void value was not rejected: %v", analysis.Diagnostics)
			}
		})
	}
	valid := `(let (items (list)) (do (append items 6) (print (list_len items))))`
	if d := analyzeExpr(t, valid).Diagnostics; len(d) != 0 {
		t.Fatalf("statement-position void rejected: %v", d)
	}
}

// TestBuiltinSignaturesRejectProvableMismatch covers HFREC-010.
func TestBuiltinSignaturesRejectProvableMismatch(t *testing.T) {
	invalid := []string{
		`(print (str_split (str_split "a,b" ",") ";"))`,
		`(print (str_split 1 ","))`,
		`(print (str_split "a" 2))`,
		`(print (str_join "a" ","))`,
		`(print (str_join (list "a") 1))`,
		`(print (regex_match 1 "a"))`,
		`(print (list_len "abc"))`,
		`(print (str_split "a"))`,
	}
	for _, src := range invalid {
		t.Run(src, func(t *testing.T) {
			if d := analyzeExpr(t, src).Diagnostics; len(d) == 0 {
				t.Fatal("invalid builtin call accepted")
			}
		})
	}
	valid := []string{
		`(print (str_split "a,b" ","))`,
		`(print (str_join (str_split "a,b" ",") "-"))`,
		`(print (regex_match "a+" "aaa"))`,
		`(print (list_len (list 1 2)))`,
		`(let (raw (read_line)) (print (list_len (str_split raw ","))))`,
		`(let (obj (dict ("k" "v"))) (print (str_split (map_get obj "x") ",")))`,
	}
	for _, src := range valid {
		t.Run(src, func(t *testing.T) {
			if d := analyzeExpr(t, src).Diagnostics; len(d) != 0 {
				t.Fatalf("valid builtin call rejected: %v", d)
			}
		})
	}
}
