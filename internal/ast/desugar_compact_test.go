package ast_test

import (
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
)

// pureHeads is the closed set of construct heads compact-conditional sugar may
// expand into. None of them reads, writes, spawns, fetches or otherwise needs a
// capability; any new head appearing in an expansion fails this test.
var pureHeads = map[string]bool{
	"let": true, "do": true, "for": true, "if": true, "set": true,
	"str_split": true, "to_int": true, "list_get": true, "parse_json": true,
	"or": true, "and": true, "=": true, ">": true, ">=": true,
}

func collectHeads(n *ast.Node, out map[string]bool) {
	if n == nil || n.Type != "List" || len(n.Children) == 0 {
		return
	}
	// Temporaries (__name) head binding lists, not construct calls.
	if h := n.Children[0]; h.Type == "SYMBOL" && !strings.HasPrefix(h.Value, "__") {
		out[h.Value] = true
	}
	for _, c := range n.Children {
		collectHeads(c, out)
	}
}

func TestCompactConditionalSugarExpandsOnlyToPureConstructs(t *testing.T) {
	cases := map[string]string{
		"ver_gt":           `(cli_app (print (ver_gt "1.2.3" "1.2.0")))`,
		"ver_ge":           `(cli_app (print (ver_ge v w)))`,
		"all":              `(cli_app (print (all x xs (= x 1))))`,
		"any":              `(cli_app (print (any x xs (> x 1))))`,
		"list_get_expr":    `(cli_app (print (list_get (str_split s ".") 1)))`,
		"parse_json_expr":  `(cli_app (print (parse_json T (str_split s "."))))`,
		"nested_all_inner": `(cli_app (print (all x xs (any y ys (= x y)))))`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			root := parseDesugar(t, src)
			heads := map[string]bool{}
			collectHeads(root.Children[1].Children[1], heads)
			for h := range heads {
				if !pureHeads[h] {
					t.Errorf("expansion contains non-pure or unexpected head %q", h)
				}
			}
			for _, sugar := range []string{"ver_gt", "ver_ge", "all", "any", "when", "not"} {
				if heads[sugar] {
					t.Errorf("sugar head %q left unexpanded", sugar)
				}
			}
		})
	}
}

func TestCompactBindOperandLeavesSymbolOperandsAlone(t *testing.T) {
	root := parseDesugar(t, `(cli_app (print (list_get xs 0)) (print (parse_json T raw)))`)
	if got := root.Children[1].Children[1].Children[0].Value; got != "list_get" {
		t.Fatalf("symbol list_get rewritten to %s", got)
	}
	if got := root.Children[2].Children[1].Children[0].Value; got != "parse_json" {
		t.Fatalf("symbol parse_json rewritten to %s", got)
	}
}
