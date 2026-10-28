package ast

import (
	"strings"
	"sync"
)

// Control-flow and core built-ins (heldout-v2 follow-up). Like the
// heldout-v1 core library, every form expands into existing, already lowered
// constructs (let, if, while, for, set, append, list_get, list_len, str_split,
// str_join, arithmetic, comparisons), so it adds no opcode, capability or
// effect and runs identically on the interpreter, AST bytecode and HFIR
// bytecode. Go/JS targets reject every form with UNSUPPORTED_ON_TARGET.
//
//   (% a b)              integer remainder, floored like Python: the result
//                        has the sign of b. (% 7 3)=1, (% -7 3)=2, (% 7 -3)=-2.
//   (// a b)             floored integer division: (// -7 2) = -4.
//                        Int operands use exact int64 arithmetic over the
//                        whole range (see cfIntCore); MinInt64 // -1 is the
//                        existing RUNTIME_ERROR "integer overflow". Float
//                        (or mixed) operands follow Python exactly (#151):
//                        % is fmod floored to the divisor's sign, // is
//                        CPython's float floor division returned as an int.
//                        Inf/NaN operands and // results outside int64 are
//                        "VALUE_ERROR: ..." (exit 1). b = 0 is the existing
//                        RUNTIME_ERROR "division by zero".
//                        Instruction cost (-run-bc): a 20k-iteration loop with
//                        one % and one // is 4.30M instructions (1.55M before
//                        #149, 3.17M after); to_base of MinInt64 in base 2 is
//                        about 16.2k instructions;
//                        the exact float path loops about twice the binary
//                        exponent gap (1e308 % 5e-324 is the worst case).
//   (if c1 a c2 b ... [else])  n-ary if: elif chains without cond clauses.
//   (break)              leaves the innermost for/while. Statement position
//                        only (directly or via do/if/when/unless/let bodies
//                        inside the loop); anywhere else is a compile error.
//   (range n) / (range a b)  list of ints a..b-1 (empty when b <= a).
//   (max_by x lst key) / (min_by x lst key)  element with the largest /
//                        smallest key (first on ties). Empty list:
//                        "VALUE_ERROR: max_by of empty list", exit 1.
//   (sort_by x lst key)  stable ascending sort by key (numbers or strings).
//                        Keys are computed once per element. O(n^2).
//   (pad_left s n [ch]) / (pad_right s n [ch])  pad (str s) with ch
//                        (default " ") until it is at least n bytes.
//   (str_repeat s n)     s repeated n times ("" when n <= 0).
//   (rtrim s) / (ltrim s)  strip " ", "\t", "\n", "\r" from one end.
//   (to_base n b)        n in base b (2..36), lowercase digits, "-" prefix
//                        for negatives. Other bases: "VALUE_ERROR: to_base
//                        base must be 2..36", exit 1.
//   (let ((a b) lst) body)  destructuring: a = (list_get lst 0), b = ... .
//                        A list of another length fails with
//                        "VALUE_ERROR: destructuring expects N elements", exit 1.

// cfFloaty(x) is true when x is a float (#151). Below 2^52 in magnitude,
// x + 2^62 is exact for an int but a float >= 2^52 for a float, and such
// floats always print with an exponent; this also catches integral floats
// that print like ints (189944.0), which the int core would otherwise run in
// float arithmetic. Larger floats print with "e", "Inf" or "NaN" (NaN fails
// both range tests), none of which an int64 ever prints.
func cfFloaty(x string) string {
	return "(if (and (< " + x + " $LIM) (> " + x + " $NLIM)) (str_contains (str (+ " + x + " $P62)) $EXP) (or (str_contains (str " + x + ") $EXP) (str_contains (str " + x + ") $INF) (str_contains (str " + x + ") $NAN)))"
}

var cfIsFloatX = "(or " + cfFloaty("$A") + " " + cfFloaty("$B") + ")"

// cfSmall is true when both operands are strictly inside (-2^52, 2^52). There
// the float formula is exact: a correctly rounded a/b cannot cross an integer
// boundary, and // divides a - r with |a - r| < 2^53 (with the old 2^53 bound
// (// (2^53-1) -3) was off by one). % and // keep the cheaper pre-#149
// expansion there, so the instruction cost of ordinary loops is unchanged.
//
// Since #151 the cheap path also requires both operands to be whole numbers
// (ints, or integral floats such as 6.0); any other float takes the exact
// float path below. A zero remainder takes the sign of the divisor as in
// Python ((% 4.0 -2.0) is -0.0, (% -0.0 5.0) is 0.0); for ints it is 0.
const cfSmall = "(and (< $A $LIM) (> $A $NLIM) (< $B $LIM) (> $B $NLIM) (= $A (to_int $A)) (= $B (to_int $B)))"

// cfCheapMod is the pre-#149 remainder formula, exact inside cfSmall.
const cfCheapMod = "(let ($R (- $A (* $B (to_int (/ $A $B))))) (if (and (!= $R 0) (!= (< $R 0) (< $B 0))) (+ $R $B) (if (= $R 0) (if (< $B 0) (* (- 0 1) (* $R $R)) (* $R $R)) $R)))"

// cfFloatCore (#151) computes R = fmod(A, B) exactly, with the sign of A,
// for float operands (or a mix of int and float, converted to float first
// like Python). Inf or NaN operands fail with "VALUE_ERROR: % and // need
// finite operands"; B = 0 is the existing "division by zero". It uses only
// existing constructs: |B| is doubled into a list of |B|*2^k (exact) while it
// fits under |A|, then subtracted back from the largest down. Each
// subtraction is exact (Sterbenz), so the result is the correctly rounded
// fmod for any magnitudes (1e308 % 5e-324 included). The loop runs about
// twice the binary-exponent gap between A and B. The doubling stops while
// 2Q <= R, tested as Q <= R - Q, so it never overflows: NUMERIC_CONTRACT
// section 7 makes float overflow an error, and R near the float64 maximum
// used to double Q to +Inf to end the loop. R - Q is exact once Q > R/2
// (Sterbenz) and cannot round below Q while 2Q <= R. extra runs after the guard;
// tail sees R (fmod), N (float A), DN (float B), AD (|B|). The caller closes
// four parens.
func cfFloatCore(extra string) string {
	return "(let ($S (if (= $B 0) (/ $A $B) (/ 0 1)) $ZF (+ (* 0 $A) (* 0 $B))) (let ($N (+ $A $ZF) $DN (+ $B $ZF)) (let ($R (if (< $N 0) (- 0 $N) $N) $AD (if (< $DN 0) (- 0 $DN) $DN) $P (list) $Q $ZF $I 0) (do " +
		"(when (or (str_contains (str $A) $INFS) (str_contains (str $A) $NANS) (str_contains (str $B) $INFS) (str_contains (str $B) $NANS)) (do (stderr $FMSG) (exit 1))) " + extra +
		" (set $Q $AD) (when (<= $Q $R) (do (append $P $Q) (while (<= $Q (- $R $Q)) (do (set $Q (* $Q 2)) (append $P $Q))))) (set $I (- (list_len $P) 1)) (while (>= $I 0) (do (when (>= $R (list_get $P $I)) (set $R (- $R (list_get $P $I)))) (set $I (- $I 1)))) (when (< $N 0) (set $R (- 0 $R))) "
}

// cfFloatMod floors the exact fmod like Python: a non-zero remainder whose
// sign differs from B gets B added; a zero remainder takes B's sign.
var cfFloatMod = cfFloatCore("") + "(if (= $R 0) (if (< $DN 0) (* (- 0 1) (* $R $R)) (* $R $R)) (if (!= (< $R 0) (< $DN 0)) (+ $R $DN) $R))))))"

// cfFloatDiv follows CPython's float floor division ((A - fmod)/B, minus one
// when the remainder's sign differs from B, then floor with a round-to-nearest
// fix-up) and returns it as an int like the rest of //. A quotient outside
// int64 fails with "VALUE_ERROR: // result outside int64" (checked before the
// loop when |A| > |B|*2^63, so huge quotients stay cheap). The check is
// written |A|/2^63 > |B| so a large |B| cannot overflow; where |A|/2^63 is
// subnormal and rounds, the check after the loop still catches the quotient.
var cfFloatDiv = cfFloatCore("(when (> (/ $R $TWO63) $AD) (do (stderr $RMSG) (exit 1)))") +
	"(let ($T (/ (- $N $R) $DN)) (do (when (and (!= $R 0) (!= (< $DN 0) (< $R 0))) (set $T (- $T 1))) (when (or (>= $T $TWO63) (< $T $NTWO63)) (do (stderr $RMSG) (exit 1))) (let ($I (to_int $T)) (do (when (> $I $T) (set $I (- $I 1))) (when (> (- $T $I) $HALF) (set $I (+ $I 1))) $I))))))))"

// cfIntCore computes, in exact int64 arithmetic, Q = |trunc(A/B)| and
// R = -(|A| mod |B|) (so DN < R <= 0) for |B| >= 2, B = 0 failing with the
// existing "division by zero". It works on non-positive magnitudes
// N = -|A|, DN = -|B| so MinInt64 is representable. One float division gives
// an estimate within ~1536 of the true quotient (|Q| <= 2^62); it is biased
// 2048 toward zero so DN*Q never passes N (no overflow), a second float
// division of the small remainder is biased by 1, and a final loop of at
// most a few steps makes DN < R <= 0 exact. The caller closes four parens.
const cfIntCore = "(let ($N (if (< $A 0) $A (- 0 $A)) $DN (if (< $B 0) $B (- 0 $B)) $Q (- (to_int (/ $N $DN)) 2048)) (do (when (< $Q 0) (set $Q 0)) (let ($R (- $N (* $DN $Q)) $T (- (to_int (/ $R $DN)) 1)) (do (when (< $T 0) (set $T 0)) (set $R (- $R (* $DN $T))) (set $Q (+ $Q $T)) (while (<= $R $DN) (do (set $R (- $R $DN)) (set $Q (+ $Q 1))))"

// cfW wraps an exact-path % result as (+ (+ x (* 0 A)) (* 0 B)). At run time
// the exact path only sees ints, so this adds 0; statically it gives the
// result the same type as the float formula (float when either operand is
// typed float), which the checker requires of both if branches.
func cfW(x string) string { return "(+ (+ " + x + " (* 0 $A)) (* 0 $B))" }

var controlflowTemplates = map[string]struct {
	arity  int
	vars   []string
	temps  []string
	tpl    string
	binder int
}{
	"%": {2, []string{"a", "b"}, []string{"A", "B", "R", "N", "DN", "Q", "T", "S", "ZF", "AD", "P", "I"},
		"(let ($A $a $B $b) (if " + cfSmall + " " + cfCheapMod + " (if " + cfIsFloatX + " " + cfFloatMod + " (if (or (= $B 1) (= $B (- 0 1))) " + cfW("0") + " " + cfIntCore + " (let ($T (if (< $A 0) $R (- 0 $R))) (if (and (!= $T 0) (!= (< $T 0) (< $B 0))) " + cfW("(+ $T $B)") + " " + cfW("$T") + "))))))))))", -1},
	"//": {2, []string{"a", "b"}, []string{"A", "B", "R", "N", "DN", "Q", "T", "S", "ZF", "AD", "P", "I"},
		"(let ($A $a $B $b) (if " + cfSmall + " (to_int (/ (- $A (% $A $B)) $B)) (if " + cfIsFloatX + " " + cfFloatDiv + " (if (= $B 1) (to_int $A) (if (= $B (- 0 1)) (to_int (- 0 $A)) " + cfIntCore + " (if (= (< $A 0) (< $B 0)) $Q (if (= $R 0) (- 0 $Q) (- 0 (+ $Q 1)))))))))))))", -1},
	"range": {2, []string{"a", "b"}, []string{"A", "B", "R"},
		"(let ($A $a $B $b $R (list)) (do (while (< $A $B) (do (append $R $A) (set $A (+ $A 1)))) $R))", -1},
	"max_by": {3, []string{"x", "lst", "key"}, []string{"L", "KS", "I", "M", "MK", "K"},
		"(let ($L $lst $KS (map_list $x $L $key) $I 0 $M (list_get $L 0) $MK (list_get $KS 0)) (do (when (= (list_len $L) 0) (do (stderr $msg) (exit 1))) (for $K $KS (do (when (> $K $MK) (do (set $M (list_get $L $I)) (set $MK $K))) (set $I (+ $I 1)))) $M))", 0},
	"min_by": {3, []string{"x", "lst", "key"}, []string{"L", "KS", "I", "M", "MK", "K"},
		"(let ($L $lst $KS (map_list $x $L $key) $I 0 $M (list_get $L 0) $MK (list_get $KS 0)) (do (when (= (list_len $L) 0) (do (stderr $msg) (exit 1))) (for $K $KS (do (when (< $K $MK) (do (set $M (list_get $L $I)) (set $MK $K))) (set $I (+ $I 1)))) $M))", 0},
	"sort_by": {3, []string{"x", "lst", "key"}, []string{"L", "KS", "R", "I", "K", "N", "F", "J", "Y"},
		"(let ($L $lst $KS (map_list $x $L $key) $R (list) $I 0) (do (for $K $KS (let ($N (list) $F 0) (do (for $J $R (do (when (and (= $F 0) (< $K (list_get $KS $J))) (do (append $N $I) (set $F 1))) (append $N $J))) (when (= $F 0) (append $N $I)) (set $R $N) (set $I (+ $I 1))))) (map_list $Y $R (list_get $L $Y))))", 0},
	"pad_left": {3, []string{"s", "n", "ch"}, []string{"S", "N", "C", "P"},
		"(let ($S (str $s) $N $n $C $ch $P $E) (do (while (< (+ (str_len $P) (str_len $S)) $N) (set $P (str $P $C))) (str $P $S)))", -1},
	"pad_right": {3, []string{"s", "n", "ch"}, []string{"S", "N", "C"},
		"(let ($S (str $s) $N $n $C $ch) (do (while (< (str_len $S) $N) (set $S (str $S $C))) $S))", -1},
	"str_repeat": {2, []string{"s", "n"}, []string{"S", "N", "R"},
		"(let ($S $s $N $n $R $E) (do (while (> $N 0) (do (set $R (str $R $S)) (set $N (- $N 1)))) $R))", -1},
	"rtrim": {1, []string{"s"}, []string{"C", "N"},
		"(let ($C (str_split $s $E) $N (list_len $C)) (do (while (and (> $N 0) (list_contains $WS (list_get $C (- $N 1)))) (set $N (- $N 1))) (str_join (list_slice $C 0 $N) $E)))", -1},
	"ltrim": {1, []string{"s"}, []string{"C", "N", "I"},
		"(let ($C (str_split $s $E) $N (list_len $C) $I 0) (do (while (and (< $I $N) (list_contains $WS (list_get $C $I))) (set $I (+ $I 1))) (str_join (list_slice $C $I $N) $E)))", -1},
	"to_base": {2, []string{"n", "b"}, []string{"N", "B", "G", "R", "NEG", "M"},
		"(let ($N $n $B $b $G (str_split $D $E) $R $E $NEG (< $N 0) $M (* 0 $N)) (do (when (or (< $B 2) (> $B 36)) (do (stderr $msg) (exit 1))) (when (not $NEG) (set $N (- 0 $N))) (while (< $N 0) (do (set $M (% $N $B)) (set $N (// $N $B)) (when (!= $M 0) (do (set $M (- $B $M)) (set $N (+ $N 1)))) (set $R (str (list_get $G $M) $R)))) (when (= $R $E) (set $R $Z)) (when $NEG (set $R (str $MINUS $R))) $R))", -1},
}

// ControlflowHeads lists every head added by this file plus break, for the
// Go/JS target rejection and the reserved-call list.
var ControlflowHeads = []string{"%", "//", "range", "max_by", "min_by", "sort_by", "pad_left", "pad_right", "str_repeat", "rtrim", "ltrim", "to_base", "break"}

// IsControlflowSugar reports whether head is a template form of this file.
func IsControlflowSugar(head string) bool {
	_, ok := controlflowTemplates[head]
	return ok
}

func strNode(v string, at *Node) *Node {
	return &Node{Type: "STRING", Value: v, Line: at.Line, Column: at.Column, Filename: at.Filename}
}

func desugarControlflow(node *Node) bool {
	head := node.Children[0].Value
	spec := controlflowTemplates[head]
	switch {
	case head == "range" && len(node.Children) == 2:
		zero := &Node{Type: "INT", Value: "0", Line: node.Line, Column: node.Column, Filename: node.Filename}
		node.Children = []*Node{node.Children[0], zero, node.Children[1]}
	case (head == "pad_left" || head == "pad_right") && len(node.Children) == 3:
		node.Children = append(node.Children, strNode(" ", node))
	}
	if len(node.Children) != spec.arity+1 {
		ReportError(head+" expects "+itoa(spec.arity)+" arguments", node.Line, node.Column)
		return false
	}
	if spec.binder >= 0 && node.Children[spec.binder+1].Type != "SYMBOL" {
		ReportError(head+" expects a variable name as argument "+itoa(spec.binder+1), node.Line, node.Column)
		return false
	}
	args := map[string]*Node{}
	for i, v := range spec.vars {
		args[v] = node.Children[i+1]
	}
	// Literal constants get a fresh node per occurrence (the template
	// builder only clones symbols).
	consts := map[string]func() *Node{
		"E":     func() *Node { return strNode("", node) },
		"Z":     func() *Node { return strNode("0", node) },
		"MINUS": func() *Node { return strNode("-", node) },
		"DOT":   func() *Node { return strNode(".", node) },
		"LIM": func() *Node {
			return &Node{Type: "INT", Value: "4503599627370496", Line: node.Line, Column: node.Column, Filename: node.Filename}
		},
		"NLIM": func() *Node {
			return &Node{Type: "INT", Value: "-4503599627370496", Line: node.Line, Column: node.Column, Filename: node.Filename}
		},
		"EXP":  func() *Node { return strNode("e", node) },
		"INFS": func() *Node { return strNode("Inf", node) },
		"NANS": func() *Node { return strNode("NaN", node) },
		"FMSG": func() *Node {
			return strNode("VALUE_ERROR: % and // need finite operands"+atPos(node)+"\n", node)
		},
		"RMSG": func() *Node { return strNode("VALUE_ERROR: // result outside int64"+atPos(node)+"\n", node) },
		"TWO63": func() *Node {
			return &Node{Type: "FLOAT", Value: "9223372036854775808.0", Line: node.Line, Column: node.Column, Filename: node.Filename}
		},
		"P62": func() *Node {
			return &Node{Type: "INT", Value: "4611686018427387904", Line: node.Line, Column: node.Column, Filename: node.Filename}
		},
		"HALF": func() *Node {
			return &Node{Type: "FLOAT", Value: "0.5", Line: node.Line, Column: node.Column, Filename: node.Filename}
		},
		"NTWO63": func() *Node {
			return &Node{Type: "FLOAT", Value: "-9223372036854775808.0", Line: node.Line, Column: node.Column, Filename: node.Filename}
		},
		"INF": func() *Node { return strNode("n", node) },
		"NAN": func() *Node { return strNode("N", node) },
		"D":   func() *Node { return strNode("0123456789abcdefghijklmnopqrstuvwxyz", node) },
		"WS": func() *Node {
			return &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename, Children: []*Node{
				symNode("list", node), strNode(" ", node), strNode("\t", node), strNode("\n", node), strNode("\r", node)}}
		},
	}
	tpl := spec.tpl
	toks := strings.Fields(strings.NewReplacer("(", " ( ", ")", " ) ").Replace(tpl))
	for i, t := range toks {
		if mk, ok := consts[strings.TrimPrefix(t, "$")]; ok && strings.HasPrefix(t, "$") {
			key := "const" + itoa(i)
			args[key] = mk()
			toks[i] = "$" + key
		}
	}
	tpl = strings.Join(toks, " ")
	switch head {
	case "max_by", "min_by":
		args["msg"] = strNode("VALUE_ERROR: "+head+" of empty list\n", node)
	case "to_base":
		args["msg"] = strNode("VALUE_ERROR: to_base base must be 2..36\n", node)
	}
	for _, tmp := range spec.temps {
		args[tmp] = symNode(compactTemp("cl"+tmp), node)
	}
	node.Children = buildTemplateFresh(tpl, args, nil, node).Children
	if head == "%" || head == "//" {
		markNumericSugar(node, head, args["a"], args["b"])
	}
	return true
}

// atPos renders " at line L, column C" for VALUE_ERROR messages emitted by
// templates (they go to stderr as plain text, not the JSON error object).
func atPos(at *Node) string {
	if at == nil || at.Line <= 0 {
		return ""
	}
	return " at line " + itoa(at.Line) + ", column " + itoa(at.Column)
}

var numericSugar = struct {
	sync.Mutex
	m map[*Node]NumericSugarSite
}{m: map[*Node]NumericSugarSite{}}

// NumericSugarSite names the % or // form a generated node came from.
type NumericSugarSite struct {
	Head string
	Root *Node
}

// markNumericSugar records every node the % or // template generated (not
// the user's operand subtrees), so the checker can report a non-numeric
// operand as "% requires numeric operands" at the form instead of an error
// about an internal comparison.
func markNumericSugar(root *Node, head string, a, b *Node) {
	numericSugar.Lock()
	defer numericSugar.Unlock()
	site := NumericSugarSite{Head: head, Root: root}
	if outer, ok := numericSugar.m[root]; ok {
		site = outer // (% A B) inside the // expansion reports as //
	}
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil || n == a || n == b {
			return
		}
		numericSugar.m[n] = site
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
}

// NumericSugarOrigin reports the % or // form that generated n, if any.
func NumericSugarOrigin(n *Node) (NumericSugarSite, bool) {
	numericSugar.Lock()
	defer numericSugar.Unlock()
	s, ok := numericSugar.m[n]
	return s, ok
}

// desugarNaryIf turns (if c1 a c2 b ... [else]) with more than three
// arguments into nested binary ifs. Returns true when it rewrote the node.
func desugarNaryIf(node *Node) bool {
	if len(node.Children) <= 4 {
		return false
	}
	args := node.Children[1:]
	var build func(i int) *Node
	build = func(i int) *Node {
		rest := len(args) - i
		if rest == 1 {
			return args[i]
		}
		n := &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{symNode("if", node), args[i], args[i+1]}}
		if rest > 2 {
			n.Children = append(n.Children, build(i+2))
		}
		return n
	}
	node.Children = build(0).Children
	return true
}

// desugarDestructure rewrites list-pattern bindings in a let binding list:
// ((a b) v) becomes (T v a (list_get T 0) b (list_get T 1)).
func desugarDestructure(node *Node) {
	if len(node.Children) < 3 {
		return
	}
	binding := node.Children[1]
	if binding == nil || binding.Type != "List" || len(binding.Children)%2 != 0 {
		return
	}
	var out []*Node
	changed := false
	for i := 0; i+1 < len(binding.Children); i += 2 {
		name, val := binding.Children[i], binding.Children[i+1]
		if name == nil || name.Type != "List" || len(name.Children) == 0 || !allSymbols(name.Children) {
			out = append(out, name, val)
			continue
		}
		changed = true
		tmp := compactTemp("clDS")
		out = append(out, symNode(tmp, node), val)
		// Length check: (do (when (!= (list_len T) n) (do (stderr msg) (exit 1))) 0).
		at := name
		n := len(name.Children)
		mk := func(ch ...*Node) *Node {
			return &Node{Type: "List", Line: at.Line, Column: at.Column, Filename: at.Filename, Children: ch}
		}
		intN := func(v int) *Node {
			return &Node{Type: "INT", Value: itoa(v), Line: at.Line, Column: at.Column, Filename: at.Filename}
		}
		check := mk(symNode("do", at),
			mk(symNode("when", at), mk(symNode("!=", at), mk(symNode("list_len", at), symNode(tmp, at)), intN(n)),
				mk(symNode("do", at),
					mk(symNode("stderr", at), strNode("VALUE_ERROR: destructuring expects "+itoa(n)+" elements"+atPos(at)+"\n", at)),
					mk(symNode("exit", at), intN(1)))),
			intN(0))
		out = append(out, symNode(compactTemp("clDSN"), node), check)
		for j, sym := range name.Children {
			idx := &Node{Type: "INT", Value: itoa(j), Line: sym.Line, Column: sym.Column, Filename: sym.Filename}
			get := &Node{Type: "List", Line: sym.Line, Column: sym.Column, Filename: sym.Filename,
				Children: []*Node{symNode("list_get", sym), symNode(tmp, sym), idx}}
			out = append(out, sym, get)
		}
	}
	if changed {
		binding.Children = out
	}
}

func allSymbols(ns []*Node) bool {
	for _, n := range ns {
		if n == nil || n.Type != "SYMBOL" {
			return false
		}
	}
	return true
}

func isBreak(n *Node) bool {
	return n != nil && n.Type == "List" && len(n.Children) >= 1 && n.Children[0].Type == "SYMBOL" && n.Children[0].Value == "break"
}

func isLoop(n *Node) bool {
	return n != nil && n.Type == "List" && len(n.Children) > 0 && n.Children[0].Type == "SYMBOL" &&
		(n.Children[0].Value == "for" || n.Children[0].Value == "while")
}

// rewriteBreaks replaces statement-position (break) forms in a loop body by
// (set F 1) and guards every later statement of an enclosing do with
// (if (= F 0) ...). It does not enter nested loops (their breaks are already
// rewritten bottom-up) and reports whether it rewrote anything.
func rewriteBreaks(n *Node, flag string) bool {
	if n == nil || n.Type != "List" || len(n.Children) == 0 || isLoop(n) {
		return false
	}
	if isBreak(n) {
		if len(n.Children) != 1 {
			ReportError("break takes no arguments", n.Line, n.Column)
		}
		n.Children = []*Node{symNode("set", n), symNode(flag, n), {Type: "INT", Value: "1", Line: n.Line, Column: n.Column, Filename: n.Filename}}
		return true
	}
	if n.Children[0].Type != "SYMBOL" {
		return false
	}
	switch n.Children[0].Value {
	case "do", "cli_app":
		for i := 1; i < len(n.Children); i++ {
			if !rewriteBreaks(n.Children[i], flag) {
				continue
			}
			if i+1 < len(n.Children) {
				rest := &Node{Type: "List", Line: n.Line, Column: n.Column, Filename: n.Filename,
					Children: append([]*Node{symNode("do", n)}, n.Children[i+1:]...)}
				rewriteBreaks(rest, flag)
				guard := &Node{Type: "List", Line: n.Line, Column: n.Column, Filename: n.Filename,
					Children: []*Node{symNode("if", n), flagIsZero(flag, n), rest}}
				n.Children = append(n.Children[:i+1], guard)
			}
			return true
		}
		return false
	case "if":
		hit := false
		for i := 2; i < len(n.Children); i++ {
			if rewriteBreaks(n.Children[i], flag) {
				hit = true
			}
		}
		return hit
	case "let":
		if len(n.Children) > 2 {
			return rewriteBreaks(n.Children[2], flag)
		}
	}
	return false
}

func flagIsZero(flag string, at *Node) *Node {
	return &Node{Type: "List", Line: at.Line, Column: at.Column, Filename: at.Filename, Children: []*Node{
		symNode("=", at), symNode(flag, at), {Type: "INT", Value: "0", Line: at.Line, Column: at.Column, Filename: at.Filename}}}
}

// desugarLoopBreak rewrites a for/while whose body breaks into
// (let (F 0) (while (and (= F 0) c) body')) or
// (let (F 0) (for x lst (if (= F 0) body'))). A for keeps iterating its
// (already evaluated) list but does nothing after the break.
func desugarLoopBreak(node *Node) {
	if len(node.Children) < 3 {
		return
	}
	bodyIdx := len(node.Children) - 1
	flag := compactTemp("clBRK")
	if !rewriteBreaks(node.Children[bodyIdx], flag) {
		return
	}
	loop := &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename, Children: node.Children}
	if node.Children[0].Value == "while" {
		loop.Children[1] = &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{symNode("and", node), flagIsZero(flag, node), loop.Children[1]}}
	} else {
		loop.Children[bodyIdx] = &Node{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename,
			Children: []*Node{symNode("if", node), flagIsZero(flag, node), loop.Children[bodyIdx]}}
	}
	node.Children = []*Node{symNode("let", node),
		{Type: "List", Line: node.Line, Column: node.Column, Filename: node.Filename, Children: []*Node{
			symNode(flag, node), {Type: "INT", Value: "0", Line: node.Line, Column: node.Column, Filename: node.Filename}}},
		loop}
}

// reportStrayBreaks fails closed on any (break) left after desugaring: one
// outside a loop, or in a value position the loop rewrite does not reach.
func reportStrayBreaks(n *Node) {
	if n == nil || n.Type != "List" {
		return
	}
	if isBreak(n) {
		ReportError("break must be a statement inside for/while (directly or in do/if/when/unless/let bodies)", n.Line, n.Column)
		return
	}
	for _, c := range n.Children {
		reportStrayBreaks(c)
	}
}
