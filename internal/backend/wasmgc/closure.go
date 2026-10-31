package wasmgc

import (
	"sort"

	"github.com/howlcipher/howlframe/internal/ast"
)

// Closures. A lambda (the handler of on_event, a subscription or spawn) is
// lifted to a function of the module taking its environment and two
// arguments: (env, a0, a1). A closure is a $Closure: a reference to that
// function and its environment, an array of the cells of the variables it
// captures. As in JavaScript, a lambda shares the variables it captures
// with the code around it: a variable that any lambda in a function names
// lives in a one-slot $Cell, made afresh each time the variable is bound
// (so a handler made in a loop keeps that iteration's value), and the
// closure holds the cell.

// lambdaArgs is how many arguments a lifted lambda receives; a lambda with
// fewer parameters ignores the rest, one with more sees nil.
const lambdaArgs = 2

// binding is where a variable lives: a local holding its value, a local
// holding its cell, or (in a lambda) a cell in the environment.
type binding struct {
	local int
	cell  bool
	env   int // slot in the environment, or -1
}

// lambdaSymbols adds the symbols named inside the lambdas within n.
func lambdaSymbols(n *ast.Node, inLambda bool, out map[string]bool) {
	if n == nil {
		return
	}
	if inLambda && n.Type == "SYMBOL" {
		out[n.Value] = true
	}
	if head(n) == "lambda" {
		inLambda = true
	}
	for _, k := range n.Children {
		lambdaSymbols(k, inLambda, out)
	}
}

// symbols adds every symbol named within n.
func symbols(n *ast.Node, out map[string]bool) {
	if n == nil {
		return
	}
	if n.Type == "SYMBOL" {
		out[n.Value] = true
	}
	for _, k := range n.Children {
		symbols(k, out)
	}
}

// declare adds a variable to the innermost scope and returns its binding;
// init then gives it the value on the stack.
func (f *fn) declare(name string) binding {
	b := binding{local: f.local(EqRef), cell: f.boxed[name], env: -1}
	f.scopes[len(f.scopes)-1][name] = b
	return b
}

// init binds a declared variable to the value on the stack, in a new cell
// when lambdas capture it.
func (f *fn) init(b binding) {
	if b.cell {
		f.code.StructNew(f.c.rt.tCell)
	}
	f.code.LocalSet(b.local)
}

// load pushes a variable's value.
func (f *fn) load(b binding) {
	r := f.c.rt
	c := f.code
	switch {
	case b.env >= 0:
		c.LocalGet(0).RefCast(r.tItems).I32Const(int32(b.env)).ArrayGet(r.tItems).RefCast(r.tCell).StructGet(r.tCell, 0)
	case b.cell:
		c.LocalGet(b.local).RefCast(r.tCell).StructGet(r.tCell, 0)
	default:
		c.LocalGet(b.local)
	}
}

// loadCell pushes a captured variable's cell.
func (f *fn) loadCell(b binding) {
	r := f.c.rt
	if b.env >= 0 {
		f.code.LocalGet(0).RefCast(r.tItems).I32Const(int32(b.env)).ArrayGet(r.tItems)
		return
	}
	f.code.LocalGet(b.local)
}

// store gives a variable the value on the stack.
func (f *fn) store(b binding) {
	r := f.c.rt
	c := f.code
	if b.env < 0 && !b.cell {
		c.LocalSet(b.local)
		return
	}
	v := f.local(EqRef)
	c.LocalSet(v)
	f.loadCell(b)
	c.RefCast(r.tCell).LocalGet(v).StructSet(r.tCell, 0)
}

// closure compiles a lambda form where it is made: the lifted function,
// then a $Closure of it and the cells of the variables it captures.
func (f *fn) closure(n *ast.Node) error {
	r := f.c.rt
	if head(n) != "lambda" || len(n.Children) != 3 || n.Children[1].Type != "List" {
		return unsupported(n, "lambda")
	}
	var params []string
	for _, p := range n.Children[1].Children {
		if p.Type != "SYMBOL" {
			return unsupported(n, "lambda parameter")
		}
		params = append(params, p.Value)
	}
	if len(params) > lambdaArgs {
		return unsupported(n, "a lambda of more than two parameters")
	}
	body := n.Children[2]
	named := map[string]bool{}
	symbols(body, named)
	for _, p := range params {
		delete(named, p)
	}
	var captured []string
	for name := range named {
		if _, ok := f.lookup(name); ok {
			captured = append(captured, name)
		}
	}
	sort.Strings(captured)
	g := &fn{c: f.c, code: NewCode(), nparam: 1 + lambdaArgs, scopes: []map[string]binding{{}}, boxed: map[string]bool{}}
	lambdaSymbols(body, false, g.boxed)
	for i, name := range captured {
		g.scopes[0][name] = binding{env: i}
	}
	for i, p := range params {
		g.param(p, 1+i)
	}
	if err := g.stmt(body); err != nil {
		return err
	}
	g.code.RefNull(HeapNone)
	idx := f.c.m.AddFunc(Func{Name: "lambda", Type: r.FuncType(1 + lambdaArgs), Locals: g.locals, Body: g.code})
	f.code.RefFunc(idx)
	for _, name := range captured {
		b, _ := f.lookup(name)
		if b.env < 0 && !b.cell {
			return unsupported(n, "capture of "+name)
		}
		f.loadCell(b)
	}
	f.code.ArrayNewFixed(r.tItems, len(captured)).StructNew(r.tClosure)
	return nil
}

// param binds parameter name, in local i, boxing it when a lambda captures
// it.
func (f *fn) param(name string, i int) {
	if !f.boxed[name] {
		f.scopes[0][name] = binding{local: i, env: -1}
		return
	}
	f.code.LocalGet(i)
	f.init(f.declare(name))
}

// emitInvoke writes __hf_invoke(closure, a0, a1), which the host calls to
// run a handler.
func (r *rt) emitInvoke() {
	fnType := r.FuncType(1 + lambdaArgs)
	idx := r.def("__hf_invoke", fnType, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tClosure).StructGet(r.tClosure, 1)
		c.LocalGet(1).LocalGet(2)
		c.LocalGet(0).RefCast(r.tClosure).StructGet(r.tClosure, 0).CallRef(fnType)
	})
	r.m.Exports = append(r.m.Exports, Export{"__hf_invoke", idx})
}
