package ast

// Core-library sugar (heldout-v1). Every form expands into existing, already
// lowered constructs (let, for, while, when, append, list_get, list_len,
// str_split, str_join, map_get, map_set, comparisons, arithmetic), so it adds
// no opcode, capability or effect and runs identically on the interpreter,
// AST bytecode and HFIR bytecode:
//
//   (list_slice lst a b)   elements a..b-1; negative a/b count from the end;
//                          out-of-range indices are clamped by the scan.
//                          (list_slice lst a) runs to the end.
//   (str_slice s a b)      same over the bytes of s; (str_slice s a) too.
//   (str_len s)            byte length of s (bytes, not characters).
//   (index_of lst v)       first 0-based index of v, or -1.
//   (map_list x lst expr)  list of expr for each x.
//   (filter x lst pred)    elements x where pred holds.
//   (reduce acc init x lst expr)  fold: acc starts at init, set to expr per x.
//   (sum lst)              numeric sum (0 for empty).
//   (min lst) / (max lst)  smallest / largest element (first on ties). An
//                          empty list is a runtime error: stderr
//                          "VALUE_ERROR: min of empty list" and exit 1, the
//                          same stderr+exit shape as assert, on every path.
//   (reverse lst)          list in reverse order.
//   (count_by x lst key)   dict mapping each key value to its integer count.
//   (sort lst)             ascending, stable; numbers or strings (byte order).
//                          Insertion sort: O(n^2) comparisons and copies.

var corelibTemplates = map[string]struct {
	arity int
	vars  []string // user-supplied placeholders in order (children 1..)
	temps []string
	tpl   string
	// binder is the index (into vars) of a loop-variable symbol, or -1.
	binder int
}{
	"list_slice": {3, []string{"lst", "a", "b"}, []string{"L", "A", "B", "R", "I", "X"},
		"(let ($L $lst $A $a $B $b $R (list) $I 0) (do (when (< $A 0) (set $A (+ $A (list_len $L)))) (when (< $B 0) (set $B (+ $B (list_len $L)))) (for $X $L (do (when (and (>= $I $A) (< $I $B)) (append $R $X)) (set $I (+ $I 1)))) $R))", -1},
	"str_slice": {3, []string{"s", "a", "b"}, nil,
		"(str_join (list_slice (str_split $s \"\") $a $b) \"\")", -1},
	"str_len": {1, []string{"s"}, nil,
		"(list_len (str_split $s \"\"))", -1},
	"index_of": {2, []string{"lst", "v"}, []string{"L", "V", "R", "I", "X"},
		"(let ($L $lst $V $v $R (- 0 1) $I 0) (do (for $X $L (do (when (and (= $R (- 0 1)) (= $X $V)) (set $R $I)) (set $I (+ $I 1)))) $R))", -1},
	"map_list": {3, []string{"x", "lst", "expr"}, []string{"L", "R"},
		"(let ($L $lst $R (list)) (do (for $x $L (append $R $expr)) $R))", 0},
	"filter": {3, []string{"x", "lst", "pred"}, []string{"L", "R"},
		"(let ($L $lst $R (list)) (do (for $x $L (when $pred (append $R $x))) $R))", 0},
	"reduce": {5, []string{"acc", "init", "x", "lst", "expr"}, []string{"L"},
		"(let ($acc $init $L $lst) (do (for $x $L (set $acc $expr)) $acc))", 2},
	"sum": {1, []string{"lst"}, []string{"L", "S", "X"},
		"(let ($L $lst $S 0) (do (for $X $L (set $S (+ $S $X))) $S))", -1},
	"min": {1, []string{"lst"}, []string{"L", "M", "X"},
		"(let ($L $lst $M (list_get $L 0)) (do (when (= (list_len $L) 0) (do (stderr $msg) (exit 1))) (for $X $L (when (< $X $M) (set $M $X))) $M))", -1},
	"max": {1, []string{"lst"}, []string{"L", "M", "X"},
		"(let ($L $lst $M (list_get $L 0)) (do (when (= (list_len $L) 0) (do (stderr $msg) (exit 1))) (for $X $L (when (> $X $M) (set $M $X))) $M))", -1},
	"reverse": {1, []string{"lst"}, []string{"L", "R", "N"},
		"(let ($L $lst $R (list) $N (list_len $L)) (do (while (> $N 0) (do (set $N (- $N 1)) (append $R (list_get $L $N)))) $R))", -1},
	"count_by": {3, []string{"x", "lst", "key"}, []string{"L", "D", "K"},
		"(let ($L $lst $D (dict)) (do (for $x $L (let ($K $key) (map_set $D $K (+ (nz (map_get $D $K) 0) 1)))) $D))", 0},
	"sort": {1, []string{"lst"}, []string{"L", "R", "X", "N", "F", "Y"},
		"(let ($L $lst $R (list)) (do (for $X $L (let ($N (list) $F 0) (do (for $Y $R (do (when (and (= $F 0) (< $X $Y)) (do (append $N $X) (set $F 1))) (append $N $Y))) (when (= $F 0) (append $N $X)) (set $R $N)))) $R))", -1},
}

// IsCorelibSugar reports whether head is a core-library sugar form.
func IsCorelibSugar(head string) bool {
	_, ok := corelibTemplates[head]
	return ok
}

func desugarCorelib(node *Node) bool {
	head := node.Children[0].Value
	spec := corelibTemplates[head]
	// The two-argument slice forms run to the end: b defaults to the length
	// (any b >= length selects the rest; the operand is evaluated once).
	if (head == "list_slice" || head == "str_slice") && len(node.Children) == 3 {
		big := &Node{Type: "INT", Value: "9223372036854775807", Line: node.Line, Column: node.Column, Filename: node.Filename}
		node.Children = append(node.Children, big)
	}
	if len(node.Children) != spec.arity+1 {
		ReportError(head+" expects "+itoa(spec.arity)+" arguments", node.Line, node.Column)
		return false
	}
	if spec.binder >= 0 && node.Children[spec.binder+1].Type != "SYMBOL" {
		ReportError(head+" expects a variable name as argument "+itoa(spec.binder+1), node.Line, node.Column)
		return false
	}
	if head == "reduce" && node.Children[1].Type != "SYMBOL" {
		ReportError("reduce expects an accumulator name as argument 1", node.Line, node.Column)
		return false
	}
	args := map[string]*Node{}
	for i, v := range spec.vars {
		args[v] = node.Children[i+1]
	}
	if head == "min" || head == "max" {
		args["msg"] = &Node{Type: "STRING", Value: "VALUE_ERROR: " + head + " of empty list\n", Line: node.Line, Column: node.Column, Filename: node.Filename}
	}
	for _, tmp := range spec.temps {
		args[tmp] = symNode(compactTemp("cl"+tmp), node)
	}
	node.Children = buildTemplateFresh(spec.tpl, args, nil, node).Children
	return true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
