package ast

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Compact-conditional density sugar. Every form here expands into existing,
// already-lowered constructs (let, for, when, not, str_split, to_int, list_get,
// comparisons), so it adds no opcode, capability or effect:
//
//   - (ver_gt a b) / (ver_ge a b): strict numeric x.y.z comparison of two
//     version strings (a > b, a >= b), lexicographic over major/minor/patch.
//   - (all x lst pred) / (any x lst pred): bind each element of lst to x and
//     test pred; all is true for an empty list, any is false.
//   - (list_get <expr> i): a non-symbol list operand is bound to a fresh
//     temporary first, since the lowering reads a variable name.
//   - (parse_json T <expr>): likewise binds a non-symbol text operand.

var compactGensym atomic.Uint64

func compactTemp(prefix string) string {
	return fmt.Sprintf("__%s%d", prefix, compactGensym.Add(1))
}

// buildTemplate parses a small s-expression template. Tokens of the form $name
// are replaced by args[name]; each placeholder may appear at most once per
// argument node supplied (callers bind values to temporaries first). Template
// strings never contain spaces or escapes.
func buildTemplate(src string, args map[string]*Node, at *Node) *Node {
	toks := strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(src))
	pos := 0
	mk := func(typ, val string) *Node {
		return &Node{Type: typ, Value: val, Line: at.Line, Column: at.Column, Filename: at.Filename}
	}
	var parse func() *Node
	parse = func() *Node {
		t := toks[pos]
		pos++
		if t == "(" {
			n := mk("List", "")
			for toks[pos] != ")" {
				n.Children = append(n.Children, parse())
			}
			pos++
			return n
		}
		if strings.HasPrefix(t, "$") {
			if a, ok := args[t[1:]]; ok {
				return a
			}
			panic("buildTemplate: missing arg " + t)
		}
		if strings.HasPrefix(t, "\"") {
			return mk("STRING", strings.Trim(t, "\""))
		}
		if t[0] >= '0' && t[0] <= '9' {
			return mk("INT", t)
		}
		return mk("SYMBOL", t)
	}
	return parse()
}

func symNode(name string, at *Node) *Node {
	return &Node{Type: "SYMBOL", Value: name, Line: at.Line, Column: at.Column, Filename: at.Filename}
}

func desugarVersionCompare(node *Node, orEqual bool) {
	if len(node.Children) != 3 {
		ReportError(node.Children[0].Value+" expects ("+node.Children[0].Value+" a b) over x.y.z strings", node.Line, node.Column)
		return
	}
	args := map[string]*Node{"a": node.Children[1], "b": node.Children[2]}
	names := []string{"A", "B", "a0", "a1", "a2", "b0", "b1", "b2"}
	for _, n := range names {
		args[n] = symNode(compactTemp("v"+n), node)
	}
	// Each temp symbol node is referenced multiple times below; give each
	// occurrence its own node so the tree stays a tree.
	ref := map[string]int{}
	tpl := "(let ($A (str_split $a \".\") $B (str_split $b \".\")" +
		" $a0 (to_int (list_get $A 0)) $a1 (to_int (list_get $A 1)) $a2 (to_int (list_get $A 2))" +
		" $b0 (to_int (list_get $B 0)) $b1 (to_int (list_get $B 1)) $b2 (to_int (list_get $B 2)))" +
		" (or (> $a0 $b0) (and (= $a0 $b0) (or (> $a1 $b1) (and (= $a1 $b1) (LAST $a2 $b2))))))"
	last := ">"
	if orEqual {
		last = ">="
	}
	tpl = strings.Replace(tpl, "LAST", last, 1)
	expanded := buildTemplateFresh(tpl, args, ref, node)
	node.Children = expanded.Children
}

// buildTemplateFresh is buildTemplate but clones SYMBOL placeholders on every
// use after the first, so temporaries can be referenced repeatedly.
func buildTemplateFresh(src string, args map[string]*Node, _ map[string]int, at *Node) *Node {
	used := map[string]bool{}
	wrapped := map[string]*Node{}
	for k, v := range args {
		wrapped[k] = v
	}
	toks := strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(src))
	for i, t := range toks {
		if strings.HasPrefix(t, "$") {
			k := t[1:]
			if used[k] {
				a := args[k]
				if a.Type != "SYMBOL" {
					panic("buildTemplateFresh: non-symbol placeholder reused: " + k)
				}
				alias := fmt.Sprintf("%s#%d", k, i)
				wrapped[alias] = symNode(a.Value, at)
				toks[i] = "$" + alias
			}
			used[k] = true
		}
	}
	return buildTemplate(strings.Join(toks, " "), wrapped, at)
}

func desugarAllAny(node *Node, isAll bool) {
	head := node.Children[0].Value
	if len(node.Children) != 4 || node.Children[1].Type != "SYMBOL" {
		ReportError(head+" expects ("+head+" var list predicate)", node.Line, node.Column)
		return
	}
	args := map[string]*Node{
		"x":    node.Children[1],
		"lst":  node.Children[2],
		"pred": node.Children[3],
		"L":    symNode(compactTemp("ql"), node),
		"F":    symNode(compactTemp("qf"), node),
	}
	var tpl string
	if isAll {
		tpl = "(let ($L $lst $F 1) (do (for $x $L (when (not $pred) (set $F 0))) (= $F 1)))"
	} else {
		tpl = "(let ($L $lst $F 0) (do (for $x $L (when $pred (set $F 1))) (= $F 1)))"
	}
	node.Children = buildTemplateFresh(tpl, args, nil, node).Children
}

// desugarBindOperand rewrites (head ... <expr at idx> ...) into
// (let (__t <expr>) (head ... __t ...)) when the operand is not a symbol.
func desugarBindOperand(node *Node, idx int, prefix string) bool {
	if len(node.Children) <= idx || node.Children[idx] == nil || node.Children[idx].Type != "List" {
		return false
	}
	tmp := compactTemp(prefix)
	inner := &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename}
	inner.Children = append(inner.Children, node.Children...)
	operand := inner.Children[idx]
	inner.Children[idx] = symNode(tmp, node)
	node.Children = []*Node{
		symNode("let", node),
		{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename, Children: []*Node{symNode(tmp, node), operand}},
		inner,
	}
	return true
}

func isListLiteral(node *Node, idx int) bool {
	if len(node.Children) <= idx {
		return false
	}
	op := node.Children[idx]
	return op != nil && op.Type == "List" && len(op.Children) > 0 && op.Children[0].Type == "SYMBOL" && op.Children[0].Value == "list"
}
