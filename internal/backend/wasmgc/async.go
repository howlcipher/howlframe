package wasmgc

import (
	"fmt"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/capability"
)

// Asynchronous operations: resumable frames.
//
// Browser storage, file reading, image decoding and encoding, sleep and the
// network complete later. HowlFrame source calls them like any function (the
// JavaScript backend awaits them), and so does this backend: a function that
// reaches one, directly or through the functions it calls, is asynchronous,
// and is compiled so that it can stop and resume.
//
// The operation's import starts it and returns at once. The calling code
// then unwinds: each asynchronous function on the way out saves its locals
// and the number of the suspension it is stopped at in a frame, pushes the
// frame on the fiber's frame stack and returns, until the entry (main or a
// handler) returns to the host. The host waits for the operation and
// resumes the fiber: it calls the entry again with the result, and each
// function rewinds, restoring its locals from its frame and steering its
// structured control flow (every statement, branch and loop is entered only
// if it holds the suspension being resumed) back to where it stopped, where
// the result is taken and the code goes on. Only asynchronous functions pay
// for this, and only on the way out and back in.
//
// So that nothing waits on the operand stack across a suspension, an
// asynchronous function is first rewritten (anf) so that each asynchronous
// call is a statement, or the whole value of a let, try_let, set or return,
// with arguments that are free of asynchronous calls.

// asyncHostOps are the asynchronous browser host operations this backend
// imports (ZIP is ordinary code here), and sleep.
var asyncHostOps = func() map[string]bool {
	ops := map[string]bool{"sleep": true}
	for _, op := range capability.HostOperations() {
		if _, std := stdOps[op.Name]; op.Async && !std {
			ops[op.Name] = true
		}
	}
	return ops
}()

// asyncInfo knows which defuns are asynchronous.
type asyncInfo struct {
	defuns map[string]bool
}

// newAsyncInfo finds the asynchronous defuns: those that reach an
// asynchronous operation or call an asynchronous defun (a fixed point). A
// lambda's body is its own function and does not count.
func newAsyncInfo(defuns []*ast.Node) *asyncInfo {
	a := &asyncInfo{defuns: map[string]bool{}}
	for changed := true; changed; {
		changed = false
		for _, d := range defuns {
			name := d.Children[1].Value
			if a.defuns[name] {
				continue
			}
			for _, k := range ast.DefunBody(d) {
				if a.contains(k) {
					a.defuns[name], changed = true, true
					break
				}
			}
		}
	}
	return a
}

// call reports whether n is an asynchronous call.
func (a *asyncInfo) call(n *ast.Node) bool {
	h := head(n)
	if asyncHostOps[h] {
		return true
	}
	return h == "call" && len(n.Children) > 1 && a.defuns[n.Children[1].Value]
}

// contains reports whether n makes an asynchronous call, outside lambdas.
func (a *asyncInfo) contains(n *ast.Node) bool {
	if n == nil || head(n) == "lambda" {
		return false
	}
	if a.call(n) {
		return true
	}
	for _, k := range n.Children {
		if a.contains(k) {
			return true
		}
	}
	return false
}

// anf rewrites an asynchronous function's statements so that every
// asynchronous call is a statement or the value of a let, try_let, set or
// return, with arguments free of asynchronous calls. Arguments evaluated
// before an asynchronous one move into temporaries ahead of it, keeping
// their order. A literal or a variable stays where it is.
type anf struct {
	a *asyncInfo
	n int
}

type anfBinding struct{ name, value *ast.Node }

func symbolNode(name string, at *ast.Node) *ast.Node {
	return &ast.Node{Type: "SYMBOL", Value: name, Line: at.Line, Column: at.Column, Filename: at.Filename}
}

func listNode(at *ast.Node, kids ...*ast.Node) *ast.Node {
	return &ast.Node{Type: "List", Children: kids, Line: at.Line, Column: at.Column, Filename: at.Filename}
}

func (x *anf) temp(at *ast.Node) *ast.Node {
	x.n++
	return symbolNode(fmt.Sprintf("__hf_async_%d", x.n), at)
}

// clone copies a subtree, so that a form evaluated in two places has a node
// (and a suspension number) for each.
func clone(n *ast.Node) *ast.Node {
	if n == nil {
		return nil
	}
	c := *n
	c.Children = make([]*ast.Node, len(n.Children))
	for i, k := range n.Children {
		c.Children[i] = clone(k)
	}
	return &c
}

// wrap nests body in lets of the bindings, in order.
func wrap(pre []anfBinding, body *ast.Node) *ast.Node {
	for i := len(pre) - 1; i >= 0; i-- {
		body = listNode(body, symbolNode("let", body), listNode(body, pre[i].name, pre[i].value), body)
	}
	return body
}

// lazyHeads evaluate their operands conditionally or as statements.
var lazyHeads = map[string]bool{"and": true, "or": true, "if": true, "let": true, "try_let": true, "do": true,
	"while": true, "for": true, "lambda": true, "match": true, "catch": true}

func (x *anf) stmts(ns []*ast.Node) ([]*ast.Node, error) {
	out := make([]*ast.Node, len(ns))
	for i, n := range ns {
		var err error
		if out[i], err = x.stmt(n); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// stmt rewrites a statement.
func (x *anf) stmt(n *ast.Node) (*ast.Node, error) {
	if !x.a.contains(n) {
		return n, nil
	}
	kids := n.Children
	switch head(n) {
	case "do":
		body, err := x.stmts(kids[1:])
		if err != nil {
			return nil, err
		}
		return listNode(n, append([]*ast.Node{kids[0]}, body...)...), nil
	case "if":
		if len(kids) < 3 {
			return nil, unsupported(n, "if")
		}
		branches, err := x.stmts(kids[2:])
		if err != nil {
			return nil, err
		}
		pre, cond, err := x.value(kids[1])
		if err != nil {
			return nil, err
		}
		return wrap(pre, listNode(n, append([]*ast.Node{kids[0], cond}, branches...)...)), nil
	case "while":
		body, err := x.stmts(kids[2:])
		if err != nil {
			return nil, err
		}
		if !x.a.contains(kids[1]) {
			return listNode(n, append([]*ast.Node{kids[0], kids[1]}, body...)...), nil
		}
		// (while c body) is (let (t c) (while t body (set t c))), with c
		// evaluated as often and in the same order.
		t := x.temp(n)
		again, err := x.stmt(listNode(n, symbolNode("set", n), t, clone(kids[1])))
		if err != nil {
			return nil, err
		}
		loop := listNode(n, append(append([]*ast.Node{kids[0], t}, body...), again)...)
		return x.let1(n, t, kids[1], []*ast.Node{loop})
	case "for":
		if len(kids) < 4 {
			return nil, unsupported(n, "for")
		}
		body, err := x.stmts(kids[3:])
		if err != nil {
			return nil, err
		}
		if !x.a.contains(kids[2]) {
			return listNode(n, append([]*ast.Node{kids[0], kids[1], kids[2]}, body...)...), nil
		}
		t := x.temp(n)
		loop := listNode(n, append([]*ast.Node{kids[0], kids[1], t}, body...)...)
		return x.let1(n, t, kids[2], []*ast.Node{loop})
	case "let":
		if len(kids) < 3 || kids[1].Type != "List" || len(kids[1].Children)%2 != 0 {
			return nil, unsupported(n, "let")
		}
		binds := kids[1].Children
		if len(binds) == 0 {
			body, err := x.stmts(kids[2:])
			if err != nil {
				return nil, err
			}
			return listNode(n, append([]*ast.Node{kids[0], kids[1]}, body...)...), nil
		}
		var body []*ast.Node
		if len(binds) > 2 {
			// (let (a x b y) body) is (let (a x) (let (b y) body)).
			inner, err := x.stmt(listNode(n, append([]*ast.Node{kids[0], listNode(kids[1], binds[2:]...)}, kids[2:]...)...))
			if err != nil {
				return nil, err
			}
			body = []*ast.Node{inner}
		} else {
			var err error
			if body, err = x.stmts(kids[2:]); err != nil {
				return nil, err
			}
		}
		return x.let1(n, binds[0], binds[1], body)
	case "try_let":
		if len(kids) != 4 || len(kids[1].Children) != 2 || len(kids[2].Children) != 3 {
			return nil, unsupported(n, "try_let")
		}
		v := kids[1].Children[1]
		if x.a.contains(v) && !(x.a.call(v) && !x.argsContain(v)) {
			return nil, unsupported(v, "an asynchronous call inside the value of a try_let (bind it with its own try_let)")
		}
		handler, err := x.stmt(kids[2].Children[2])
		if err != nil {
			return nil, err
		}
		body, err := x.stmt(kids[3])
		if err != nil {
			return nil, err
		}
		catch := listNode(kids[2], kids[2].Children[0], kids[2].Children[1], handler)
		return listNode(n, kids[0], kids[1], catch, body), nil
	case "set", "return":
		if len(kids) != 2+boolInt(head(n) == "set") {
			return nil, unsupported(n, head(n))
		}
		v := kids[len(kids)-1]
		pre, v2, err := x.direct(v)
		if err != nil {
			return nil, err
		}
		return wrap(pre, listNode(n, append(append([]*ast.Node{}, kids[:len(kids)-1]...), v2)...)), nil
	}
	if lazyHeads[head(n)] {
		return nil, unsupported(n, "an asynchronous call inside "+head(n))
	}
	pre, n2, err := x.args(n)
	if err != nil {
		return nil, err
	}
	return wrap(pre, n2), nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// let1 is (let (name v) body...), with v's inner asynchronous calls moved
// ahead of it; v itself may be an asynchronous call.
func (x *anf) let1(at, name, v *ast.Node, body []*ast.Node) (*ast.Node, error) {
	pre, v2, err := x.direct(v)
	if err != nil {
		return nil, err
	}
	return wrap(pre, listNode(at, append([]*ast.Node{symbolNode("let", at), listNode(at, name, v2)}, body...)...)), nil
}

// direct returns bindings and a form for v that is an asynchronous call
// with plain arguments, or free of asynchronous calls.
func (x *anf) direct(v *ast.Node) ([]anfBinding, *ast.Node, error) {
	if x.a.call(v) {
		return x.args(v)
	}
	return x.value(v)
}

// value returns bindings and a form free of asynchronous calls with v's
// value.
func (x *anf) value(v *ast.Node) ([]anfBinding, *ast.Node, error) {
	if !x.a.contains(v) {
		return nil, v, nil
	}
	if x.a.call(v) {
		pre, v2, err := x.args(v)
		if err != nil {
			return nil, nil, err
		}
		t := x.temp(v)
		return append(pre, anfBinding{t, v2}), t, nil
	}
	switch head(v) {
	case "let":
		// A let's bindings are evaluated first and then its body, so they
		// move ahead, its variable renamed to a temporary (the variable's
		// scope grows to the whole statement).
		kids := v.Children
		if len(kids) < 3 || kids[1].Type != "List" || len(kids[1].Children)%2 != 0 || len(kids[1].Children) == 0 {
			return nil, nil, unsupported(v, "let")
		}
		binds := kids[1].Children
		body := kids[2:]
		if len(binds) > 2 {
			body = []*ast.Node{listNode(v, append([]*ast.Node{kids[0], listNode(kids[1], binds[2:]...)}, body...)...)}
		}
		if len(body) != 1 {
			if x.a.contains(listNode(v, body...)) {
				return nil, nil, unsupported(v, "an asynchronous call in a let of several forms used as a value")
			}
			body = []*ast.Node{listNode(v, append([]*ast.Node{symbolNode("do", v)}, body...)...)}
		}
		pre, val, err := x.direct(binds[1])
		if err != nil {
			return nil, nil, err
		}
		t := x.temp(v)
		pre = append(pre, anfBinding{t, val})
		rest, out, err := x.value(rename(body[0], binds[0].Value, t.Value))
		if err != nil {
			return nil, nil, err
		}
		return append(pre, rest...), out, nil
	case "do":
		if len(v.Children) == 2 {
			return x.value(v.Children[1])
		}
	case "if", "and", "or":
		// Only the first operand is always evaluated, so only it can move.
		for _, k := range v.Children[2:] {
			if x.a.contains(k) {
				return nil, nil, unsupported(k, "an asynchronous call in a branch of "+head(v)+" (bind it with let first)")
			}
		}
		pre, c, err := x.value(v.Children[1])
		if err != nil {
			return nil, nil, err
		}
		return pre, listNode(v, append([]*ast.Node{v.Children[0], c}, v.Children[2:]...)...), nil
	}
	if lazyHeads[head(v)] {
		return nil, nil, unsupported(v, "an asynchronous call inside "+head(v)+" used as a value")
	}
	return x.args(v)
}

// argStart is the first child of n that is an evaluated argument.
func argStart(n *ast.Node) int {
	switch head(n) {
	case "call", "set", "append", "map_set", "map_delete", "parse_json":
		return 2
	}
	return 1
}

func (x *anf) argsContain(n *ast.Node) bool {
	for _, k := range n.Children[argStart(n):] {
		if x.a.contains(k) {
			return true
		}
	}
	return false
}

// args moves n's asynchronous arguments, and the arguments before the last
// of them, into temporaries ahead of n.
func (x *anf) args(n *ast.Node) ([]anfBinding, *ast.Node, error) {
	if head(n) == "dict" && x.a.contains(n) {
		return nil, nil, unsupported(n, "an asynchronous call inside a dict literal (bind it with let first)")
	}
	start := argStart(n)
	last := -1
	for i := start; i < len(n.Children); i++ {
		if x.a.contains(n.Children[i]) {
			last = i
		}
	}
	if last < 0 {
		return nil, n, nil
	}
	kids := append([]*ast.Node(nil), n.Children...)
	var pre []anfBinding
	for i := start; i <= last; i++ {
		k := kids[i]
		if x.a.contains(k) {
			p, k2, err := x.value(k)
			if err != nil {
				return nil, nil, err
			}
			pre = append(pre, p...)
			kids[i] = k2
			continue
		}
		if k.Type == "List" {
			t := x.temp(k)
			pre = append(pre, anfBinding{t, k})
			kids[i] = t
		}
	}
	return pre, listNode(n, kids...), nil
}

// numberSites numbers the asynchronous calls of a rewritten function, from
// 1 in source order, and records for every form the range of numbers it
// holds.
func (f *fn) numberSites(n *ast.Node) {
	if n == nil || head(n) == "lambda" {
		return
	}
	before := f.nsites
	if f.async.call(n) {
		f.nsites++
		f.sites[n] = f.nsites
	}
	for _, k := range n.Children {
		f.numberSites(k)
	}
	if f.nsites > before {
		f.ranges[n] = [2]int{before + 1, f.nsites}
	}
}

// resuming pushes whether the function is resuming at a suspension in n.
func (f *fn) resuming(n *ast.Node) {
	c := f.code
	rg, ok := f.ranges[n]
	if !ok {
		c.I32Const(0)
		return
	}
	c.LocalGet(f.state).I32Const(int32(rg[0])).I32GeU()
	c.LocalGet(f.state).I32Const(int32(rg[1])).I32LeU().I32And()
}

// enter pushes whether to run n: always, unless the function is resuming,
// and then only if n holds the suspension being resumed.
func (f *fn) enter(n *ast.Node) {
	c := f.code
	c.GlobalGet(f.c.rt.gRewinding).If(Result(I32))
	f.resuming(n)
	c.Else().I32Const(1).End()
}

// unlessResuming runs body only when the function is not resuming.
func (f *fn) unlessResuming(body func() error) error {
	if f.async == nil {
		return body()
	}
	c := f.code
	c.GlobalGet(f.c.rt.gRewinding).I32Eqz().If(NoResult)
	f.labels++
	err := body()
	f.labels--
	c.End()
	return err
}

// suspend saves the function's locals so far and suspension k in a new
// frame on the frame stack, and returns.
func (f *fn) suspend(k int) error {
	r := f.c.rt
	c := f.code
	c.I32Const(int32(k))
	n := f.nparam + len(f.locals)
	for i := 0; i < n; i++ {
		c.LocalGet(i)
		if i >= f.nparam {
			switch t := f.locals[i-f.nparam]; {
			case sameTypes([]ValType{t}, []ValType{I32}):
				c.I64ExtendI32S().Call(r.fBoxInt)
			case sameTypes([]ValType{t}, []ValType{EqRef}), refNullIndex(t) >= 0:
			default:
				return fmt.Errorf("wasmgc: an asynchronous function holds a local of type %x", t)
			}
		}
	}
	c.ArrayNewFixed(r.tItems, n).GlobalGet(r.gStack).StructNew(r.tFrame).GlobalSet(r.gStack)
	c.RefNull(HeapNone).Return()
	return nil
}

// asyncHostCall compiles suspension k at an asynchronous host operation:
// the import starts it (a failure to start is an ordinary error) and the
// function suspends; on resuming, the host's result is the value and its
// failure the pending error.
func (f *fn) asyncHostCall(n *ast.Node, k int, idx int, args func() error) error {
	r := f.c.rt
	c := f.code
	if k == 0 {
		return unsupported(n, head(n)+" (an asynchronous operation here)")
	}
	c.GlobalGet(r.gRewinding).If(Result(EqRef))
	f.labels++
	c.I32Const(0).GlobalSet(r.gRewinding).GlobalGet(r.gResume)
	c.RefNull(HeapNone).GlobalSet(r.gResume)
	c.Else()
	if err := args(); err != nil {
		return err
	}
	c.Call(idx).Drop()
	f.check()
	c.I32Const(1).GlobalSet(r.gUnwinding)
	if err := f.suspend(k); err != nil {
		return err
	}
	f.labels--
	c.End()
	f.check()
	return nil
}

// asyncCall compiles suspension k at a call of an asynchronous defun:
// resuming calls it again (it restores its own frame) with nil arguments.
func (f *fn) asyncCall(k, idx, nargs int, args func() error) error {
	r := f.c.rt
	c := f.code
	c.GlobalGet(r.gRewinding).If(Result(EqRef))
	for i := 0; i < nargs; i++ {
		c.RefNull(HeapNone)
	}
	c.Call(idx).Else()
	f.labels++
	if err := args(); err != nil {
		return err
	}
	f.labels--
	c.Call(idx).End()
	v := f.local(EqRef)
	c.LocalSet(v)
	c.GlobalGet(r.gUnwinding).If(NoResult)
	f.labels++
	if err := f.suspend(k); err != nil {
		return err
	}
	f.labels--
	c.End()
	f.check()
	c.LocalGet(v)
	return nil
}

// prologue returns the start of an asynchronous function: when resuming,
// it pops the function's frame and restores its locals and suspension.
func (f *fn) prologue() (*Code, error) {
	r := f.c.rt
	c := NewCode()
	c.GlobalGet(r.gRewinding).If(NoResult)
	c.GlobalGet(r.gStack).RefCast(r.tFrame).LocalSet(f.frame)
	c.LocalGet(f.frame).StructGet(r.tFrame, 2).GlobalSet(r.gStack)
	c.LocalGet(f.frame).StructGet(r.tFrame, 0).LocalSet(f.state)
	n := f.nparam + len(f.locals)
	for i := 0; i < n; i++ {
		if i == f.frame || i == f.state {
			continue
		}
		c.LocalGet(f.frame).RefCast(r.tFrame).StructGet(r.tFrame, 1).ArrayLen().I32Const(int32(i)).I32GtU().If(NoResult)
		c.LocalGet(f.frame).RefCast(r.tFrame).StructGet(r.tFrame, 1).I32Const(int32(i)).ArrayGet(r.tItems)
		if i >= f.nparam {
			switch t := f.locals[i-f.nparam]; {
			case sameTypes([]ValType{t}, []ValType{I32}):
				c.RefCast(r.tInt).StructGet(r.tInt, 0).I32WrapI64()
			case sameTypes([]ValType{t}, []ValType{EqRef}):
			case refNullIndex(t) >= 0:
				c.RefCastNull(refNullIndex(t))
			default:
				return nil, fmt.Errorf("wasmgc: an asynchronous function holds a local of type %x", t)
			}
		}
		c.LocalSet(i).End()
	}
	c.End()
	return c, nil
}

// emitAsync writes the exports the host drives fibers with.
func (r *rt) emitAsync() {
	m := r.m
	export := func(name string, params, results []ValType, body func(c *Code)) {
		idx := r.def(name, r.funcType(params, results...), nil, body)
		m.Exports = append(m.Exports, Export{name, idx})
	}
	// __hf_unwinding(): whether the call that just returned suspended.
	export("__hf_unwinding", nil, []ValType{I32}, func(c *Code) { c.GlobalGet(r.gUnwinding) })
	// __hf_take_fiber(): the suspended fiber's frames, leaving none.
	export("__hf_take_fiber", nil, []ValType{EqRef}, func(c *Code) {
		c.I32Const(0).GlobalSet(r.gUnwinding)
		c.GlobalGet(r.gStack).RefNull(HeapNone).GlobalSet(r.gStack)
	})
	// __hf_resume(fiber, value): the next call of the fiber's entry resumes
	// it, with value as the operation's result.
	export("__hf_resume", []ValType{EqRef, EqRef}, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tFrame).GlobalSet(r.gStack)
		c.LocalGet(1).GlobalSet(r.gResume)
		c.I32Const(1).GlobalSet(r.gRewinding)
	})
}

// statementHeads are the forms a function's last form is a statement for.
var statementHeads = map[string]bool{"return": true, "print": true, "set": true, "while": true, "for": true, "let": true,
	"do": true, "if": true, "stderr": true, "exit": true, "append": true, "map_set": true, "map_delete": true, "try_let": true}

// steer pushes a branch condition: resuming, whether target holds the
// suspension; otherwise cond.
func (f *fn) steer(target *ast.Node, cond func() error) error {
	if f.async == nil {
		return cond()
	}
	c := f.code
	c.GlobalGet(f.c.rt.gRewinding).If(Result(I32))
	f.labels++
	f.resuming(target)
	c.Else()
	err := cond()
	f.labels--
	c.End()
	return err
}

// entered runs body when n is to be run (see enter).
func (f *fn) entered(n *ast.Node, body func() error) error {
	if f.async == nil {
		return body()
	}
	c := f.code
	f.enter(n)
	c.If(NoResult)
	f.labels++
	err := body()
	f.labels--
	c.End()
	return err
}

// beginAsync makes f asynchronous when its body makes an asynchronous call:
// the body rewritten (anf), its suspensions numbered, and the locals for
// resuming.
func (f *fn) beginAsync(body []*ast.Node) ([]*ast.Node, error) {
	a := f.c.async
	if a == nil {
		return body, nil
	}
	async := false
	for _, n := range body {
		async = async || a.contains(n)
	}
	if !async {
		return body, nil
	}
	x := &anf{a: a}
	body, err := x.stmts(body)
	if err != nil {
		return nil, err
	}
	f.async, f.sites, f.ranges = a, map[*ast.Node]int{}, map[*ast.Node][2]int{}
	for _, n := range body {
		f.numberSites(n)
	}
	f.state = f.local(I32)
	f.frame = f.local(RefNull(f.c.rt.tFrame))
	return body, nil
}

// endAsync returns the function's code: an asynchronous function's starts
// with its prologue.
func (f *fn) endAsync() (*Code, error) {
	if f.async == nil {
		return f.code, nil
	}
	pro, err := f.prologue()
	if err != nil {
		return nil, err
	}
	pro.Append(f.code)
	return pro, nil
}

// refNullIndex returns T for a local of type (ref null T), or -1.
func refNullIndex(t ValType) int {
	if len(t) < 2 || t[0] != 0x63 {
		return -1
	}
	v, shift := 0, 0
	for _, b := range t[1:] {
		v |= int(b&0x7F) << shift
		shift += 7
		if b&0x80 == 0 {
			if b&0x40 != 0 {
				return -1 // an abstract heap type
			}
			return v
		}
	}
	return -1
}

// rename returns n with the variable from renamed to to, except where a
// nested form binds from again.
func rename(n *ast.Node, from, to string) *ast.Node {
	if n == nil {
		return nil
	}
	if n.Type == "SYMBOL" {
		if n.Value == from {
			c := *n
			c.Value = to
			return &c
		}
		return n
	}
	kids := n.Children
	switch head(n) {
	case "let":
		// (let (a x b y) body): each value sees the names bound before it.
		if len(kids) >= 2 && kids[1].Type == "List" {
			binds := append([]*ast.Node(nil), kids[1].Children...)
			for i := 0; i+1 < len(binds); i += 2 {
				binds[i+1] = rename(binds[i+1], from, to)
				if binds[i].Value == from {
					out := append([]*ast.Node{kids[0], listNode(kids[1], binds...)}, kids[2:]...)
					return listNode(n, out...)
				}
			}
			out := []*ast.Node{kids[0], listNode(kids[1], binds...)}
			for _, k := range kids[2:] {
				out = append(out, rename(k, from, to))
			}
			return listNode(n, out...)
		}
	case "for":
		if len(kids) >= 3 && kids[1].Value == from {
			return listNode(n, append([]*ast.Node{kids[0], kids[1], rename(kids[2], from, to)}, kids[3:]...)...)
		}
	case "lambda":
		if len(kids) >= 2 {
			for _, p := range kids[1].Children {
				if p.Value == from {
					return n
				}
			}
		}
	case "try_let":
		if len(kids) == 4 && len(kids[1].Children) == 2 && len(kids[2].Children) == 3 {
			bound := listNode(kids[1], kids[1].Children[0], rename(kids[1].Children[1], from, to))
			catch := kids[2]
			if catch.Children[1].Value != from {
				catch = listNode(catch, catch.Children[0], catch.Children[1], rename(catch.Children[2], from, to))
			}
			body := kids[3]
			if kids[1].Children[0].Value != from {
				body = rename(body, from, to)
			}
			return listNode(n, kids[0], bound, catch, body)
		}
	}
	out := make([]*ast.Node, len(kids))
	for i, k := range kids {
		out[i] = rename(k, from, to)
	}
	c := *n
	c.Children = out
	return &c
}
