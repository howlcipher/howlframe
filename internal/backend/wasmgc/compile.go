package wasmgc

import (
	"fmt"
	"strconv"

	"github.com/howlcipher/howlframe/internal/ast"
)

// UnsupportedError reports a construct this backend cannot compile yet. The
// build fails instead of emitting a module that would behave differently.
type UnsupportedError struct {
	Construct    string
	Line, Column int
}

func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("WASM_UNSUPPORTED: %q is not supported by the WebAssembly backend yet (line %d, column %d)", e.Construct, e.Line, e.Column)
}

// Compile turns a checked, desugared cli_app into a WebAssembly module whose
// exported main runs the program. The module imports print and float
// formatting from the host (see Loader).
func Compile(root *ast.Node) ([]byte, error) {
	if root == nil || root.Type != "List" || len(root.Children) == 0 || root.Children[0].Value != "cli_app" {
		return nil, fmt.Errorf("WASM_UNSUPPORTED: the WebAssembly backend compiles cli_app programs so far")
	}
	m := NewModule()
	c := &compiler{m: m, rt: newRuntime(m), funcs: map[string]int{}, arity: map[string]int{}}
	body := root.Children[1:]
	// Declare every defun first so calls can come before definitions.
	var defuns []*ast.Node
	var stmts []*ast.Node
	for _, n := range body {
		if head(n) == "defun" {
			defuns = append(defuns, n)
		} else {
			stmts = append(stmts, n)
		}
	}
	for _, d := range defuns {
		if len(d.Children) < 3 || d.Children[1].Type != "SYMBOL" || d.Children[2].Type != "List" {
			return nil, &UnsupportedError{"defun", d.Line, d.Column}
		}
		name := d.Children[1].Value
		params := d.Children[2].Children
		idx := m.AddFunc(Func{Name: name, Type: c.rt.FuncType(len(params))})
		c.funcs[name] = idx
		c.arity[name] = len(params)
	}
	for _, d := range defuns {
		var names []string
		for _, p := range d.Children[2].Children {
			names = append(names, p.Value)
		}
		code, locals, err := c.function(names, ast.DefunBody(d), false)
		if err != nil {
			return nil, err
		}
		f := &m.Funcs[c.funcs[d.Children[1].Value]-len(m.Imports)]
		f.Body, f.Locals = code, locals
	}
	code, locals, err := c.function(nil, stmts, true)
	if err != nil {
		return nil, err
	}
	main := m.AddFunc(Func{Name: "main", Type: c.rt.FuncType(0), Locals: locals, Body: code})
	m.Exports = append(m.Exports, Export{"main", main})
	return m.Bytes()
}

type compiler struct {
	m     *Module
	rt    *rt
	funcs map[string]int
	arity map[string]int
}

// fn is the state of one function being compiled.
type fn struct {
	c      *compiler
	code   *Code
	nparam int
	locals []ValType
	scopes []map[string]int
	// labels counts open blocks, so a break can compute its depth.
	labels int
	// handlers holds the label depth of each enclosing try_let handler.
	handlers []int
	// scratch locals: one eqref temporary per nesting level.
	scratch []int
}

func (c *compiler) function(params []string, body []*ast.Node, isMain bool) (*Code, []ValType, error) {
	f := &fn{c: c, code: NewCode(), nparam: len(params), scopes: []map[string]int{{}}}
	for i, p := range params {
		f.scopes[0][p] = i
	}
	for i, n := range body {
		last := i == len(body)-1
		if last && !isMain {
			// A function's last form is its value when it is an expression.
			if err := f.tail(n); err != nil {
				return nil, nil, err
			}
			continue
		}
		if err := f.stmt(n); err != nil {
			return nil, nil, err
		}
	}
	if isMain || len(body) == 0 {
		f.code.RefNull(HeapNone)
	}
	return f.code, f.locals, nil
}

// tail compiles a function's final form: return statements and
// expressions leave the function's value; other statements yield nil.
func (f *fn) tail(n *ast.Node) error {
	switch head(n) {
	case "return":
		if err := f.stmt(n); err != nil {
			return err
		}
		f.code.RefNull(HeapNone)
		return nil
	case "print", "set", "while", "for", "let", "do", "if", "stderr", "exit", "append", "map_set", "map_delete", "try_let":
		if err := f.stmt(n); err != nil {
			return err
		}
		f.code.RefNull(HeapNone)
		return nil
	}
	return f.expr(n)
}

func (f *fn) local(t ValType) int {
	f.locals = append(f.locals, t)
	return f.nparam + len(f.locals) - 1
}

func (f *fn) lookup(name string) (int, bool) {
	for i := len(f.scopes) - 1; i >= 0; i-- {
		if idx, ok := f.scopes[i][name]; ok {
			return idx, true
		}
	}
	return 0, false
}

func head(n *ast.Node) string {
	if n != nil && n.Type == "List" && len(n.Children) > 0 && n.Children[0].Type == "SYMBOL" {
		return n.Children[0].Value
	}
	return ""
}

func unsupported(n *ast.Node, what string) error {
	return &UnsupportedError{what, n.Line, n.Column}
}

// check propagates a pending error: to the innermost try_let handler, or
// out of the function.
func (f *fn) check() {
	c := f.code
	c.GlobalGet(f.c.rt.gErr).RefIsNull().I32Eqz().If(NoResult)
	f.labels++
	if len(f.handlers) > 0 {
		c.Br(f.labels - f.handlers[len(f.handlers)-1] - 1)
	} else {
		c.RefNull(HeapNone).Return()
	}
	f.labels--
	c.End()
}

// stmt compiles a form for its effect.
func (f *fn) stmt(n *ast.Node) error {
	c := f.code
	switch head(n) {
	case "print":
		return f.print(n)
	case "do":
		for _, k := range n.Children[1:] {
			if err := f.stmt(k); err != nil {
				return err
			}
		}
		return nil
	case "let":
		return f.let(n, false)
	case "set":
		if len(n.Children) != 3 || n.Children[1].Type != "SYMBOL" {
			return unsupported(n, "set")
		}
		idx, ok := f.lookup(n.Children[1].Value)
		if !ok {
			return unsupported(n, "set of an unknown variable")
		}
		if err := f.expr(n.Children[2]); err != nil {
			return err
		}
		c.LocalSet(idx)
		return nil
	case "if":
		if len(n.Children) < 3 {
			return unsupported(n, "if")
		}
		if err := f.cond(n.Children[1]); err != nil {
			return err
		}
		c.If(NoResult)
		f.labels++
		if err := f.stmt(n.Children[2]); err != nil {
			return err
		}
		if len(n.Children) > 3 {
			c.Else()
			if err := f.stmt(n.Children[3]); err != nil {
				return err
			}
		}
		f.labels--
		c.End()
		return nil
	case "while":
		c.Block(NoResult).Loop(NoResult)
		f.labels += 2
		if err := f.cond(n.Children[1]); err != nil {
			return err
		}
		c.I32Eqz().BrIf(1)
		for _, k := range n.Children[2:] {
			if err := f.stmt(k); err != nil {
				return err
			}
		}
		c.Br(0)
		f.labels -= 2
		c.End().End()
		return nil
	case "return":
		if len(n.Children) == 1 {
			c.RefNull(HeapNone).Return()
			return nil
		}
		if err := f.expr(n.Children[1]); err != nil {
			return err
		}
		c.Return()
		return nil
	case "append":
		// (append xs v) rebinds xs to a list one longer; lists are values.
		if len(n.Children) != 3 || n.Children[1].Type != "SYMBOL" {
			return unsupported(n, "append")
		}
		idx, ok := f.lookup(n.Children[1].Value)
		if !ok {
			return unsupported(n, "append to an unknown variable")
		}
		c.LocalGet(idx)
		if err := f.expr(n.Children[2]); err != nil {
			return err
		}
		c.Call(f.c.rt.fListAppend)
		f.check()
		c.LocalSet(idx)
		return nil
	case "for":
		return f.forLoop(n)
	case "try_let":
		return f.tryLet(n, false)
	case "map_set", "map_delete":
		// (map_set d k v) and (map_delete d k) change the dict d names in
		// place; dicts are references.
		want := map[string]int{"map_set": 4, "map_delete": 3}[head(n)]
		if len(n.Children) != want || n.Children[1].Type != "SYMBOL" {
			return unsupported(n, head(n))
		}
		idx, ok := f.lookup(n.Children[1].Value)
		if !ok {
			return unsupported(n, head(n)+" of an unknown variable")
		}
		c.LocalGet(idx)
		for _, a := range n.Children[2:] {
			if err := f.expr(a); err != nil {
				return err
			}
		}
		if head(n) == "map_set" {
			c.Call(f.c.rt.fMapSet)
		} else {
			c.Call(f.c.rt.fMapDelete)
		}
		f.check()
		c.Drop()
		return nil
	case "stderr":
		if len(n.Children) != 2 {
			return unsupported(n, "stderr")
		}
		if err := f.expr(n.Children[1]); err != nil {
			return err
		}
		c.Call(f.c.rt.fText).Call(f.c.rt.iStderr)
		return nil
	case "exit":
		if len(n.Children) != 2 {
			return unsupported(n, "exit")
		}
		if err := f.expr(n.Children[1]); err != nil {
			return err
		}
		c.Call(f.c.rt.fToInt)
		f.check()
		c.RefCast(f.c.rt.tInt).StructGet(f.c.rt.tInt, 0).I32WrapI64().Call(f.c.rt.iExit)
		return nil
	case "type_hint", "type_hints", "type_param":
		return nil
	}
	if err := f.expr(n); err != nil {
		return err
	}
	c.Drop()
	return nil
}

// let binds (name value) pairs in a new scope for its body. As a value, the
// body's last form is the result.
func (f *fn) let(n *ast.Node, value bool) error {
	if len(n.Children) < 3 || n.Children[1].Type != "List" {
		return unsupported(n, "let")
	}
	binds := n.Children[1].Children
	if len(binds)%2 != 0 {
		return unsupported(n, "let bindings")
	}
	f.scopes = append(f.scopes, map[string]int{})
	defer func() { f.scopes = f.scopes[:len(f.scopes)-1] }()
	for i := 0; i < len(binds); i += 2 {
		if binds[i].Type != "SYMBOL" {
			return unsupported(n, "let binding name")
		}
		if err := f.expr(binds[i+1]); err != nil {
			return err
		}
		idx := f.local(EqRef)
		f.code.LocalSet(idx)
		f.scopes[len(f.scopes)-1][binds[i].Value] = idx
	}
	body := n.Children[2:]
	for i, k := range body {
		if value && i == len(body)-1 {
			return f.expr(k)
		}
		if err := f.stmt(k); err != nil {
			return err
		}
	}
	if value {
		f.code.RefNull(HeapNone)
	}
	return nil
}

// cond pushes the truthiness (i32) of an expression.
func (f *fn) cond(n *ast.Node) error {
	if err := f.expr(n); err != nil {
		return err
	}
	f.code.Call(f.c.rt.fTruthy)
	tmp := f.local(I32)
	f.code.LocalSet(tmp)
	f.check()
	f.code.LocalGet(tmp)
	return nil
}

func (f *fn) print(n *ast.Node) error {
	r := f.c.rt
	c := f.code
	// Concatenate the texts of the arguments, separated by spaces.
	acc := f.local(RefNull(r.tStr))
	r.StrLiteral(c, "").LocalSet(acc)
	for i, a := range n.Children[1:] {
		if i > 0 {
			c.LocalGet(acc).RefAsNonNull()
			r.StrLiteral(c, " ").Call(r.fConcat).LocalSet(acc)
		}
		if err := f.expr(a); err != nil {
			return err
		}
		c.Call(r.fText)
		tmp := f.local(RefNull(r.tStr))
		c.LocalSet(tmp)
		c.LocalGet(acc).RefAsNonNull().LocalGet(tmp).RefAsNonNull().Call(r.fConcat).LocalSet(acc)
	}
	c.LocalGet(acc).Call(r.iPrint)
	return nil
}

// expr compiles a form for its value: one eqref on the stack.
func (f *fn) expr(n *ast.Node) error {
	r := f.c.rt
	c := f.code
	switch n.Type {
	case "INT":
		v, err := strconv.ParseInt(n.Value, 10, 64)
		if err != nil {
			return unsupported(n, "integer literal "+n.Value)
		}
		c.I64Const(v).Call(r.fBoxInt)
		return nil
	case "FLOAT":
		v, err := strconv.ParseFloat(n.Value, 64)
		if err != nil {
			return unsupported(n, "float literal "+n.Value)
		}
		c.F64Const(v).Call(r.fBoxFloat)
		return nil
	case "STRING":
		r.StrLiteral(c, n.Value)
		return nil
	case "SYMBOL":
		switch n.Value {
		case "true":
			c.GlobalGet(r.gTrue)
			return nil
		case "false":
			c.GlobalGet(r.gFalse)
			return nil
		}
		idx, ok := f.lookup(n.Value)
		if !ok {
			return unsupported(n, "reference to "+n.Value)
		}
		c.LocalGet(idx)
		return nil
	}
	h := head(n)
	args := n.Children[1:]
	binary := map[string]int{"+": r.fAdd, "-": r.fSub, "*": r.fMul, "/": r.fDiv, "%": r.fMod}
	if fnIdx, ok := binary[h]; ok && len(args) == 2 {
		if err := f.expr(args[0]); err != nil {
			return err
		}
		if err := f.expr(args[1]); err != nil {
			return err
		}
		c.Call(fnIdx)
		f.check()
		return nil
	}
	switch h {
	case "=", "==", "!=":
		if len(args) != 2 {
			return unsupported(n, h)
		}
		if err := f.expr(args[0]); err != nil {
			return err
		}
		if err := f.expr(args[1]); err != nil {
			return err
		}
		c.Call(r.fEq)
		if h == "!=" {
			c.I32Eqz()
		}
		c.Call(r.fBoxBool)
		return nil
	case "<", ">", "<=", ">=":
		if len(args) != 2 {
			return unsupported(n, h)
		}
		var operands [2]int
		for i, a := range args {
			if err := f.expr(a); err != nil {
				return err
			}
			operands[i] = f.local(EqRef)
			c.LocalSet(operands[i])
		}
		// compare gives -1, 0 or 1; 3 for NaN (no ordering holds); 2 when the
		// operands cannot be ordered, reported here with the operator.
		c.LocalGet(operands[0]).LocalGet(operands[1]).Call(r.fCompare)
		tmp := f.local(I32)
		c.LocalTee(tmp).I32Const(2).I32Eq().If(NoResult)
		f.labels++
		r.StrLiteral(c, "TYPE_ERROR: "+h+" requires numeric operands, got ")
		c.LocalGet(operands[0]).Call(r.fGoType).Call(r.fConcat)
		r.StrLiteral(c, " and ").Call(r.fConcat)
		c.LocalGet(operands[1]).Call(r.fGoType).Call(r.fConcat).Call(r.fFail).Drop()
		f.check()
		f.labels--
		c.End()
		c.LocalGet(tmp)
		switch h {
		case "<":
			c.I32Const(0).I32LtS()
		case ">":
			c.I32Const(1).I32Eq()
		case "<=":
			c.I32Const(0).I32LeS()
		case ">=":
			c.I32Const(1).I32LeU()
		}
		c.Call(r.fBoxBool)
		return nil
	case "and", "or":
		if len(args) != 2 {
			return unsupported(n, h)
		}
		if err := f.cond(args[0]); err != nil {
			return err
		}
		c.If(Result(EqRef))
		f.labels++
		if h == "and" {
			if err := f.cond(args[1]); err != nil {
				return err
			}
			c.Call(r.fBoxBool)
			c.Else().GlobalGet(r.gFalse)
		} else {
			c.GlobalGet(r.gTrue).Else()
			if err := f.cond(args[1]); err != nil {
				return err
			}
			c.Call(r.fBoxBool)
		}
		f.labels--
		c.End()
		return nil
	case "if":
		if len(args) < 2 {
			return unsupported(n, "if")
		}
		if err := f.cond(args[0]); err != nil {
			return err
		}
		c.If(Result(EqRef))
		f.labels++
		if err := f.expr(args[1]); err != nil {
			return err
		}
		c.Else()
		if len(args) > 2 {
			if err := f.expr(args[2]); err != nil {
				return err
			}
		} else {
			c.RefNull(HeapNone)
		}
		f.labels--
		c.End()
		return nil
	case "do":
		if len(args) == 0 {
			c.RefNull(HeapNone)
			return nil
		}
		for _, k := range args[:len(args)-1] {
			if err := f.stmt(k); err != nil {
				return err
			}
		}
		return f.expr(args[len(args)-1])
	case "let":
		return f.let(n, true)
	case "try_let":
		return f.tryLet(n, true)
	case "call":
		if len(args) == 0 || args[0].Type != "SYMBOL" {
			return unsupported(n, "call")
		}
		return f.call(n, args[0].Value, args[1:])
	case "list":
		for _, a := range args {
			if err := f.expr(a); err != nil {
				return err
			}
		}
		c.ArrayNewFixed(r.tItems, len(args)).Call(r.fListNew)
		return nil
	case "dict":
		// (dict (k v) ...): the pairs are laid out key, value in one array.
		for _, p := range args {
			if p.Type != "List" || len(p.Children) != 2 {
				return unsupported(n, "dict entry")
			}
			for _, kv := range p.Children {
				if err := f.expr(kv); err != nil {
					return err
				}
			}
		}
		c.ArrayNewFixed(r.tItems, 2*len(args)).Call(r.fDictNew)
		return nil
	case "map_get", "map_keys":
		want := map[string]int{"map_get": 2, "map_keys": 1}[h]
		if len(args) != want {
			return unsupported(n, h)
		}
		for _, a := range args {
			if err := f.expr(a); err != nil {
				return err
			}
		}
		if h == "map_get" {
			c.Call(r.fMapGet)
		} else {
			c.Call(r.fMapKeys)
		}
		f.check()
		return nil
	case "list_get", "list_len":
		want := map[string]int{"list_get": 2, "list_len": 1}[h]
		if len(args) != want {
			return unsupported(n, h)
		}
		for _, a := range args {
			if err := f.expr(a); err != nil {
				return err
			}
		}
		if h == "list_get" {
			c.Call(r.fListGet)
		} else {
			c.Call(r.fListLen)
		}
		f.check()
		return nil
	case "str_split", "str_join", "to_lower", "to_upper", "str_trim", "html_escape", "attr_escape", "is_nil":
		unary := map[string]int{"to_lower": r.fLower, "to_upper": r.fUpper, "str_trim": r.fStrTrim, "html_escape": r.fEscape, "attr_escape": r.fAttrEscape, "is_nil": -1}
		binaryOps := map[string]int{"str_split": r.fStrSplit, "str_join": r.fStrJoin}
		want := 1
		if _, ok := binaryOps[h]; ok {
			want = 2
		}
		if len(args) != want {
			return unsupported(n, h)
		}
		for _, a := range args {
			if err := f.expr(a); err != nil {
				return err
			}
		}
		if h == "is_nil" {
			c.RefIsNull().Call(r.fBoxBool)
			return nil
		}
		if idx, ok := binaryOps[h]; ok {
			c.Call(idx)
		} else {
			c.Call(unary[h])
		}
		f.check()
		return nil
	case "to_int", "to_float":
		if len(args) != 1 {
			return unsupported(n, h)
		}
		if err := f.expr(args[0]); err != nil {
			return err
		}
		if h == "to_int" {
			c.Call(r.fToInt)
		} else {
			c.Call(r.fToFloat)
		}
		f.check()
		return nil
	case "to_string":
		if len(args) != 1 {
			return unsupported(n, h)
		}
		if err := f.expr(args[0]); err != nil {
			return err
		}
		c.Call(r.fText)
		return nil
	}
	if _, ok := f.c.funcs[h]; ok {
		return f.call(n, h, args)
	}
	if h == "" {
		return unsupported(n, n.Type)
	}
	return unsupported(n, h)
}

func (f *fn) call(n *ast.Node, name string, args []*ast.Node) error {
	idx, ok := f.c.funcs[name]
	if !ok {
		return unsupported(n, "call to "+name)
	}
	if f.c.arity[name] != len(args) {
		return unsupported(n, fmt.Sprintf("call to %s with %d arguments", name, len(args)))
	}
	for _, a := range args {
		if err := f.expr(a); err != nil {
			return err
		}
	}
	f.code.Call(idx)
	f.check()
	return nil
}

// forLoop compiles (for item list body...): the list is evaluated once and
// iterated as it was then (appends inside the body do not extend the loop).
func (f *fn) forLoop(n *ast.Node) error {
	r := f.c.rt
	c := f.code
	if len(n.Children) < 4 || n.Children[1].Type != "SYMBOL" {
		return unsupported(n, "for")
	}
	if err := f.expr(n.Children[2]); err != nil {
		return err
	}
	list := f.local(EqRef)
	c.LocalTee(list).RefTest(r.tList).I32Eqz().If(NoResult)
	f.labels++
	r.failWithType(c, "TYPE_ERROR: for requires a list, got ", list).Drop()
	f.check()
	f.labels--
	c.End()
	length := f.local(I32)
	i := f.local(I32)
	c.LocalGet(list).RefCast(r.tList).StructGet(r.tList, 0).LocalSet(length)
	c.I32Const(0).LocalSet(i)
	f.scopes = append(f.scopes, map[string]int{})
	item := f.local(EqRef)
	f.scopes[len(f.scopes)-1][n.Children[1].Value] = item
	c.Block(NoResult).Loop(NoResult)
	f.labels += 2
	c.LocalGet(i).LocalGet(length).I32GeU().BrIf(1)
	c.LocalGet(list).RefCast(r.tList).StructGet(r.tList, 1).StructGet(r.tBuf, 1).LocalGet(i).ArrayGet(r.tItems).LocalSet(item)
	for _, k := range n.Children[3:] {
		if err := f.stmt(k); err != nil {
			return err
		}
	}
	c.LocalGet(i).I32Const(1).I32Add().LocalSet(i).Br(0)
	f.labels -= 2
	c.End().End()
	f.scopes = f.scopes[:len(f.scopes)-1]
	return nil
}

// tryLet compiles (try_let (name expr) (catch err handler) body). When expr
// (or anything it calls) fails, the error is cleared, err is bound to the
// VM's JSON report of it and handler runs; otherwise name is bound to the
// value and body runs. Failures in handler or body are not caught. As a
// value, the form is the value of whichever of handler and body ran.
func (f *fn) tryLet(n *ast.Node, value bool) error {
	r := f.c.rt
	c := f.code
	if len(n.Children) != 4 {
		return unsupported(n, "try_let")
	}
	binding, catch, body := n.Children[1], n.Children[2], n.Children[3]
	if binding.Type != "List" || len(binding.Children) != 2 || binding.Children[0].Type != "SYMBOL" ||
		head(catch) != "catch" || len(catch.Children) != 3 || catch.Children[1].Type != "SYMBOL" {
		return unsupported(n, "try_let")
	}
	branch := func(k *ast.Node, name string, idx int, result int) error {
		f.scopes = append(f.scopes, map[string]int{name: idx})
		defer func() { f.scopes = f.scopes[:len(f.scopes)-1] }()
		if !value {
			return f.stmt(k)
		}
		if err := f.expr(k); err != nil {
			return err
		}
		c.LocalSet(result)
		return nil
	}
	result := -1
	if value {
		result = f.local(EqRef)
	}
	done := f.labels
	c.Block(NoResult)
	f.labels++
	handler := f.labels
	c.Block(NoResult)
	f.labels++
	f.handlers = append(f.handlers, handler)
	if err := f.expr(binding.Children[1]); err != nil {
		return err
	}
	f.handlers = f.handlers[:len(f.handlers)-1]
	v := f.local(EqRef)
	c.LocalSet(v)
	if err := branch(body, binding.Children[0].Value, v, result); err != nil {
		return err
	}
	c.Br(f.labels - done - 1)
	f.labels--
	c.End()
	// The handler: take the pending error.
	e := f.local(EqRef)
	c.GlobalGet(r.gErr).RefCast(r.tStr).Call(r.fCatchText).LocalSet(e)
	c.RefNull(HeapNone).GlobalSet(r.gErr)
	if err := branch(catch.Children[2], catch.Children[1].Value, e, result); err != nil {
		return err
	}
	f.labels--
	c.End()
	if value {
		c.LocalGet(result)
	}
	return nil
}
