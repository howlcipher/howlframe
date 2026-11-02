package wasmgc

import (
	"embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/moduleapi"
	"github.com/howlcipher/howlframe/internal/parser"
)

// The standard library: HowlFrame modules compiled into a program that
// reaches one of their operations. Their functions are named
// __std_<module>_<name>, and only they may use the byte intrinsics.
//
//go:embed std/*.howl
var stdFiles embed.FS

// stdOps maps an operation to the standard-library module and exported
// function that implement it.
var stdOps = map[string]struct{ module, fn string }{
	"encode_json": {"json", "encode"},
	"regex_match": {"regex", "match"},
	"parse_json":  {"json", "parse"},
	"utf8_valid":  {"utf8", "valid"},
	// Browser host operations of the JavaScript backend with no VM form,
	// on base64 data.
	"base64_text":      {"base64", "text"},
	"sniff_media_type": {"base64", "sniff"},
	"zip_build":        {"zip", "build"},
	"zip_read":         {"zip", "read"},
}

const stdPrefix = "__std_"

// stdDefuns parses, links and desugars a standard-library module and
// returns its definitions, named __std_<module>_<name>.
func stdDefuns(module string) ([]*ast.Node, error) {
	src, err := stdFiles.ReadFile("std/" + module + ".howl")
	if err != nil {
		return nil, fmt.Errorf("WASM_UNSUPPORTED: standard library module %q is missing", module)
	}
	return stdModule(module, src)
}

// stdModule parses, links and desugars a standard-library module's
// source. Text after the module form is an error: a stray parenthesis
// would otherwise end the module early and drop what follows.
func stdModule(module string, src []byte) ([]*ast.Node, error) {
	p := parser.NewParser(lexer.NewLexer(string(src)), "std/"+module+".howl")
	mod := p.ParseExpression()
	if p.Cur.Type != lexer.TokenEOF {
		return nil, fmt.Errorf("WASM_UNSUPPORTED: standard library module %q has text after its module form (line %d)", module, p.Cur.Line)
	}
	if mod.Type != "List" || len(mod.Children) == 0 || mod.Children[0].Value != "module" {
		return nil, fmt.Errorf("WASM_UNSUPPORTED: standard library module %q is not a module", module)
	}
	// Link it as the program's linker would a (use ... as __std_<module>).
	alias := &ast.Node{Type: "SYMBOL", Value: strings.TrimSuffix(stdPrefix, "_") + "_" + module}
	linked := &ast.Node{Type: "List", Children: append([]*ast.Node{mod.Children[0], alias}, mod.Children[1:]...)}
	root := &ast.Node{Type: "List", Children: []*ast.Node{{Type: "SYMBOL", Value: "cli_app"}, linked}}
	ast.ResolveModules(root)
	ast.Desugar(root)
	return root.Children[1:], nil
}

// usesHead reports whether any form in n has one of the heads.
func usesHead(n *ast.Node, heads ...string) bool {
	if n == nil {
		return false
	}
	for _, h := range heads {
		if head(n) == h {
			return true
		}
	}
	for _, k := range n.Children {
		if usesHead(k, heads...) {
			return true
		}
	}
	return false
}

// makesFloats reports whether a program can produce a float: a float
// literal, "/" (always a float division), to_float, parse_json, or a
// browser host operation whose result can hold a number from the page.
func makesFloats(n *ast.Node) bool {
	if n == nil {
		return false
	}
	if n.Type == "FLOAT" {
		return true
	}
	switch head(n) {
	case "/", "to_float", "parse_json":
		return true
	}
	if op, ok := capability.LookupHostOperation(head(n)); ok {
		switch op.Result {
		case capability.HostNumber, capability.HostDict, capability.HostList, capability.HostValue:
			if _, std := stdOps[op.Name]; !std {
				return true
			}
		}
	}
	for _, k := range n.Children {
		if makesFloats(k) {
			return true
		}
	}
	return false
}

// stdCalls adds the standard-library modules that a std definition calls.
func stdCalls(n *ast.Node, seen map[string]bool, out *[]string) {
	if n == nil {
		return
	}
	if n.Type == "SYMBOL" && strings.HasPrefix(n.Value, stdPrefix) {
		mod, _, _ := strings.Cut(strings.TrimPrefix(n.Value, stdPrefix), "_")
		if !seen[mod] {
			seen[mod] = true
			*out = append(*out, mod)
		}
	}
	for _, k := range n.Children {
		stdCalls(k, seen, out)
	}
}

// stdModulesUsed lists, in a stable order, the standard-library modules a
// program reaches.
func stdModulesUsed(n *ast.Node, seen map[string]bool, out *[]string) {
	if n == nil {
		return
	}
	if op, ok := stdOps[head(n)]; ok && !seen[op.module] {
		seen[op.module] = true
		*out = append(*out, op.module)
	}
	for _, k := range n.Children {
		stdModulesUsed(k, seen, out)
	}
}

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
// exported main runs the program in Node (see Loader).
func Compile(root *ast.Node) ([]byte, error) { return CompileFor(root, HostNode) }

// CompileFor compiles for a host. A WASI module (HostWASI) is a command:
// its exported _start runs the program, writes an uncaught error as the
// VM's JSON report on standard error and exits 1.
func CompileFor(root *ast.Node, host Host) ([]byte, error) {
	if rootHead(root) != "cli_app" {
		return nil, fmt.Errorf("WASM_UNSUPPORTED: the WebAssembly backend compiles cli_app programs so far (the test blocks of a web_app build with --tests)")
	}
	return compile(root, host, false, nil)
}

// CompileModule compiles a web_app's top-level exported functions for use by
// the generated Node ESM loader. Browser host operations are refused: a Node
// SDK module must not acquire page capabilities implicitly.
func CompileModule(root *ast.Node) ([]byte, []string, error) {
	if rootHead(root) != "web_app" {
		return nil, nil, fmt.Errorf("WASM_UNSUPPORTED: an exported module requires a web_app root")
	}
	exports := moduleapi.Exports(root)
	if len(exports) == 0 {
		return nil, nil, fmt.Errorf("--module: the program has no top-level (export (defun ...)) to export")
	}
	ops := browserOps(root)
	if len(ops) != 0 {
		return nil, nil, fmt.Errorf("WASM_UNSUPPORTED: Node module exports cannot use browser host operations: %s", strings.Join(ops, ", "))
	}
	names := make([]string, 0, len(exports))
	seen := map[string]bool{}
	for _, d := range exports {
		name := d.Children[1].Value
		publicName := moduleapi.PublicName(name)
		if name == "main" || name == "memory" || strings.HasPrefix(name, "__hf_") || seen[publicName] {
			return nil, nil, unsupported(d, "an exported function named "+name+" (reserved or duplicated)")
		}
		seen[publicName] = true
		names = append(names, name)
	}
	wasm, err := compile(root, HostBrowser, false, exports)
	return wasm, names, err
}

// CompileTests compiles a program's test blocks, whatever its root, into a
// test module: the program's functions, one exported function per test
// (test_0, test_1, ... in source order, a used module's tests where its use
// is), and a main that runs them all and reports like node:test's spec
// reporter. The program's top-level statements are not part of it, as on
// the JavaScript backend's app.test.js. A failing assert, an exit or an
// uncaught error fails its test only; the run exits 1 when any test failed.
func CompileTests(root *ast.Node, host Host) ([]byte, error) {
	if h := rootHead(root); h != "cli_app" && h != "web_app" {
		return nil, fmt.Errorf("WASM_UNSUPPORTED: test blocks compile from a cli_app or web_app, not %q", h)
	}
	return compile(root, host, true, nil)
}

func rootHead(root *ast.Node) string {
	if root == nil || root.Type != "List" || len(root.Children) == 0 {
		return ""
	}
	return root.Children[0].Value
}

func compile(root *ast.Node, host Host, tests bool, moduleExports []*ast.Node) ([]byte, error) {
	m := NewModule()
	var ops []string
	if host == HostBrowser {
		ops = browserOps(root)
	}
	c := &compiler{m: m, rt: newRuntime(m, host, ops), funcs: map[string]int{}, arity: map[string]int{}, tests: tests}
	if tests {
		c.gExiting = m.AddGlobal(Global{Type: I32, Mutable: true, Init: []byte{0x41, 0x00}})
	}
	body := root.Children[1:]
	// Declare every defun first so calls can come before definitions.
	var defuns, stmts, testBlocks []*ast.Node
	for _, n := range body {
		if head(n) == "export" && len(n.Children) == 2 && head(n.Children[1]) == "defun" {
			defuns = append(defuns, n.Children[1])
		}
	}
	for _, n := range body {
		switch {
		case head(n) == "export" && len(n.Children) == 2 && head(n.Children[1]) == "defun":
			// The wrapped defun was collected above; it is an export declaration,
			// not a top-level statement.
		case head(n) == "defun":
			if strings.HasPrefix(n.Children[1].Value, stdPrefix) {
				return nil, unsupported(n, "a function named "+n.Children[1].Value+" (the prefix is reserved)")
			}
			defuns = append(defuns, n)
		case tests && head(n) == "test":
			if len(n.Children) < 3 || n.Children[1].Type != "STRING" {
				return nil, unsupported(n, "test (expects (test \"name\" body...))")
			}
			testBlocks = append(testBlocks, n)
		case tests:
			// Top-level statements are the application's, not the tests'.
		case host == HostBrowser && head(n) == "test":
			// A page leaves its tests out, as the JavaScript backend's
			// app.js does; they build with --tests.
		default:
			stmts = append(stmts, n)
		}
	}
	// Standard-library modules the program reaches, and the ones they call
	// (by their linked names, __std_<module>_<name>).
	// The float module formats and parses floats through the runtime's
	// bridges. A program without a float literal, "/", to_float or
	// parse_json can never hold a float, so it goes without the module
	// (about 19 KB) and the bridges trap if ever reached.
	var modules []string
	seen := map[string]bool{}
	if makesFloats(root) {
		modules, seen["float"] = []string{"float"}, true
	}
	// The unicode module maps case for to_lower and to_upper of text that
	// is not all ASCII.
	if usesHead(root, "to_lower", "to_upper") {
		modules, seen["unicode"] = append(modules, "unicode"), true
	}
	stdModulesUsed(root, seen, &modules)
	for i := 0; i < len(modules); i++ {
		std, err := stdDefuns(modules[i])
		if err != nil {
			return nil, err
		}
		for _, d := range std {
			stdCalls(d, seen, &modules)
			// Std code that makes floats or maps case needs those modules too.
			if !seen["float"] && makesFloats(d) {
				modules, seen["float"] = append(modules, "float"), true
			}
			if !seen["unicode"] && usesHead(d, "to_lower", "to_upper") {
				modules, seen["unicode"] = append(modules, "unicode"), true
			}
		}
		defuns = append(defuns, std...)
	}
	for _, d := range defuns {
		if len(d.Children) < 3 || d.Children[1].Type != "SYMBOL" || d.Children[2].Type != "List" {
			return nil, &UnsupportedError{"defun", d.Line, d.Column}
		}
	}
	if host == HostBrowser {
		c.async = newAsyncInfo(defuns)
	}
	for _, d := range defuns {
		name := d.Children[1].Value
		params := d.Children[2].Children
		idx := m.AddFunc(Func{Name: name, Type: c.rt.FuncType(len(params))})
		c.funcs[name] = idx
		c.arity[name] = len(params)
	}
	for _, d := range defuns {
		var names []string
		for _, p := range d.Children[2].Children {
			// A parameter is a name or a typed (name type) pair.
			if p.Type == "List" && len(p.Children) > 0 {
				p = p.Children[0]
			}
			if p.Type != "SYMBOL" {
				return nil, unsupported(d, "defun parameter")
			}
			names = append(names, p.Value)
		}
		c.std = strings.HasPrefix(d.Children[1].Value, stdPrefix)
		code, locals, err := c.function(names, ast.DefunBody(d), false)
		if err != nil {
			return nil, err
		}
		f := &m.Funcs[c.funcs[d.Children[1].Value]-len(m.Imports)]
		f.Body, f.Locals = code, locals
	}
	for _, d := range moduleExports {
		name := d.Children[1].Value
		if _, ok := c.funcs[name]; !ok {
			return nil, unsupported(d, "an exported function that is not defined at the top level")
		}
		m.Exports = append(m.Exports, Export{name, c.funcs[name]})
	}
	// The runtime's float bridges call the float module.
	if seen["float"] {
		c.m.Funcs[c.rt.fStdShortest-len(m.Imports)].Body = NewCode().LocalGet(0).Call(c.rt.fBoxFloat).Call(c.funcs[stdPrefix+"float_shortest"])
		c.m.Funcs[c.rt.fStdParse-len(m.Imports)].Body = NewCode().LocalGet(0).Call(c.funcs[stdPrefix+"float_parse"])
	} else {
		c.m.Funcs[c.rt.fStdShortest-len(m.Imports)].Body = NewCode().Unreachable()
		c.m.Funcs[c.rt.fStdParse-len(m.Imports)].Body = NewCode().Unreachable()
	}
	for _, b := range []struct {
		bridge int
		fn     string
	}{{c.rt.fStdLower, "unicode_lower"}, {c.rt.fStdUpper, "unicode_upper"}} {
		body := NewCode().Unreachable()
		if seen["unicode"] {
			body = NewCode().LocalGet(0).Call(c.funcs[stdPrefix+b.fn])
		}
		c.m.Funcs[b.bridge-len(m.Imports)].Body = body
	}
	c.std = false
	var code *Code
	var locals []ValType
	if tests {
		var err error
		if code, locals, err = c.testRunner(testBlocks); err != nil {
			return nil, err
		}
	} else {
		var err error
		if code, locals, err = c.function(nil, stmts, true); err != nil {
			return nil, err
		}
	}
	main := m.AddFunc(Func{Name: "main", Type: c.rt.FuncType(0), Locals: locals, Body: code})
	m.Exports = append(m.Exports, Export{"main", main})
	if host == HostWASI {
		r := c.rt
		start := NewCode().Call(main).Drop()
		start.GlobalGet(r.gErr).RefIsNull().I32Eqz().If(NoResult)
		start.GlobalGet(r.gErr).RefCast(r.tStr).Call(r.fCatchText).Call(r.iStderr)
		r.StrLiteral(start, "\n").Call(r.iStderr)
		start.I32Const(1).Call(r.iExit).End()
		m.Exports = append(m.Exports, Export{"_start", m.AddFunc(Func{Name: "_start", Type: r.tVoid, Body: start})})
	}
	return m.Bytes()
}

type compiler struct {
	m     *Module
	rt    *rt
	funcs map[string]int
	arity map[string]int
	// std is set while compiling a standard-library function, the only
	// code that may use the byte intrinsics.
	std bool
	// tests is set when compiling a test module. There exit ends the
	// running test, not the process: it raises "HowlFrame exit N at
	// file:line" (the JavaScript backend's text) and sets gExiting, which
	// no try_let catches.
	tests    bool
	gExiting int
	// async knows the asynchronous defuns (browser host only).
	async *asyncInfo
}

// testRunner compiles each test block into its own exported function and
// returns main's body, which runs them in order. A test passes when it
// returns with no pending error. The report follows node:test's spec
// reporter without durations: "✔ name" or "✖ name" and the error, then
// the counts. main exits 1 when a test failed.
func (c *compiler) testRunner(blocks []*ast.Node) (*Code, []ValType, error) {
	r := c.rt
	var tests []int
	for i, b := range blocks {
		code, locals, err := c.function(nil, b.Children[2:], true)
		if err != nil {
			return nil, nil, err
		}
		name := fmt.Sprintf("test_%d", i)
		idx := c.m.AddFunc(Func{Name: name, Type: r.FuncType(0), Locals: locals, Body: code})
		c.m.Exports = append(c.m.Exports, Export{name, idx})
		tests = append(tests, idx)
	}
	// locals: 0 passed, 1 failed, 2 the error
	run := NewCode()
	for i, b := range blocks {
		name := b.Children[1].Value
		run.Call(tests[i]).Drop()
		run.GlobalGet(r.gErr).RefIsNull().If(NoResult)
		run.LocalGet(0).I32Const(1).I32Add().LocalSet(0)
		r.StrLiteral(run, "✔ "+name).Call(r.iPrint)
		run.Else()
		run.LocalGet(1).I32Const(1).I32Add().LocalSet(1)
		run.GlobalGet(r.gErr).LocalSet(2)
		run.RefNull(HeapNone).GlobalSet(r.gErr)
		run.I32Const(0).GlobalSet(c.gExiting)
		r.StrLiteral(run, "✖ "+name).Call(r.iPrint)
		r.StrLiteral(run, "  Error: ")
		run.LocalGet(2).RefCast(r.tStr).Call(r.fConcat).Call(r.iPrint)
		run.End()
	}
	count := func(label string, local int) {
		r.StrLiteral(run, "ℹ "+label+" ")
		if local < 0 {
			run.I64Const(0)
		} else {
			run.LocalGet(local).I64ExtendI32U()
		}
		run.Call(r.fItoa).Call(r.fConcat).Call(r.iPrint)
	}
	r.StrLiteral(run, fmt.Sprintf("ℹ tests %d", len(blocks))).Call(r.iPrint)
	count("suites", -1)
	count("pass", 0)
	count("fail", 1)
	count("cancelled", -1)
	count("skipped", -1)
	count("todo", -1)
	run.LocalGet(1).If(NoResult).I32Const(1).Call(r.iExit).End()
	run.RefNull(HeapNone)
	return run, []ValType{I32, I32, EqRef}, nil
}

// fn is the state of one function being compiled.
type fn struct {
	c      *compiler
	code   *Code
	nparam int
	locals []ValType
	scopes []map[string]binding
	// boxed names the variables that lambdas in this function capture
	// (closure.go): they live in cells.
	boxed map[string]bool
	// An asynchronous function (async.go): its suspensions by form and the
	// range of them each form holds, the locals of the suspension being
	// resumed and of the popped frame, and the statement being entered.
	async        *asyncInfo
	sites        map[*ast.Node]int
	ranges       map[*ast.Node][2]int
	nsites       int
	state, frame int
	guarded      *ast.Node
	// labels counts open blocks, so a break can compute its depth.
	labels int
	// handlers holds the label depth of each enclosing try_let handler.
	handlers []int
	// scratch locals: one eqref temporary per nesting level.
	scratch []int
}

func (c *compiler) function(params []string, body []*ast.Node, isMain bool) (*Code, []ValType, error) {
	f := &fn{c: c, code: NewCode(), nparam: len(params), scopes: []map[string]binding{{}}, boxed: map[string]bool{}}
	body, err := f.beginAsync(body)
	if err != nil {
		return nil, nil, err
	}
	for _, n := range body {
		lambdaSymbols(n, false, f.boxed)
	}
	for i, p := range params {
		f.param(p, i)
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
	code, err := f.endAsync()
	return code, f.locals, err
}

// tail compiles a function's final form: return statements and
// expressions leave the function's value; other statements yield nil.
func (f *fn) tail(n *ast.Node) error {
	if _, ok := f.ranges[n]; ok && f.async != nil && !statementHeads[head(n)] {
		return f.stmt(listNode(n, symbolNode("return", n), n))
	}
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

func (f *fn) lookup(name string) (binding, bool) {
	for i := len(f.scopes) - 1; i >= 0; i-- {
		if b, ok := f.scopes[i][name]; ok {
			return b, true
		}
	}
	return binding{}, false
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
	f.code.GlobalGet(f.c.rt.gErr).RefIsNull().I32Eqz()
	f.propagateIf()
}

// propagateIf passes the pending error on, to the innermost try_let
// handler or out of the function, when the i32 on the stack is set.
func (f *fn) propagateIf() {
	c := f.code
	c.If(NoResult)
	f.labels++
	if len(f.handlers) > 0 {
		c.Br(f.labels - f.handlers[len(f.handlers)-1] - 1)
	} else {
		c.RefNull(HeapNone).Return()
	}
	f.labels--
	c.End()
}

// stmt compiles a form for its effect. In an asynchronous function each
// statement runs only if the function is not resuming or the statement
// holds the suspension being resumed.
func (f *fn) stmt(n *ast.Node) error {
	if f.async != nil && f.guarded != n {
		f.enter(n)
		f.code.If(NoResult)
		f.labels++
		f.guarded = n
		err := f.stmt(n)
		f.labels--
		f.code.End()
		return err
	}
	f.guarded = nil
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
		b, ok := f.lookup(n.Children[1].Value)
		if !ok {
			return unsupported(n, "set of an unknown variable")
		}
		if err := f.expr(n.Children[2]); err != nil {
			return err
		}
		f.store(b)
		return nil
	case "if":
		if len(n.Children) < 3 {
			return unsupported(n, "if")
		}
		if err := f.steer(n.Children[2], func() error { return f.cond(n.Children[1]) }); err != nil {
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
		// Resuming goes straight into the body, which holds the suspension.
		if err := f.steer(n, func() error { return f.cond(n.Children[1]) }); err != nil {
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
		b, ok := f.lookup(n.Children[1].Value)
		if !ok {
			return unsupported(n, "append to an unknown variable")
		}
		f.load(b)
		if err := f.expr(n.Children[2]); err != nil {
			return err
		}
		c.Call(f.c.rt.fListAppend)
		f.check()
		f.store(b)
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
		b, ok := f.lookup(n.Children[1].Value)
		if !ok {
			return unsupported(n, head(n)+" of an unknown variable")
		}
		f.load(b)
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
		if f.c.tests {
			// In a test, exit fails the test: an error no try_let catches.
			r := f.c.rt
			code := f.local(EqRef)
			c.LocalSet(code)
			r.StrLiteral(c, "HowlFrame exit ")
			c.LocalGet(code).Call(r.fText).Call(r.fConcat)
			r.StrLiteral(c, fmt.Sprintf(" at %s:%d", n.Filename, n.Line)).Call(r.fConcat)
			c.GlobalSet(r.gErr)
			c.I32Const(1).GlobalSet(f.c.gExiting)
			f.check()
			return nil
		}
		c.RefCast(f.c.rt.tInt).StructGet(f.c.rt.tInt, 0).I32WrapI64()
		if f.c.rt.host == HostBrowser {
			f.c.rt.StrLiteral(c, fmt.Sprintf("%s:%d", n.Filename, n.Line)).Call(f.c.rt.iExitAt)
			return nil
		}
		c.Call(f.c.rt.iExit)
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
	f.scopes = append(f.scopes, map[string]binding{})
	defer func() { f.scopes = f.scopes[:len(f.scopes)-1] }()
	for i := 0; i < len(binds); i += 2 {
		if binds[i].Type != "SYMBOL" {
			return unsupported(n, "let binding name")
		}
		// The value is compiled before the name is in scope: it sees the
		// variable the binding shadows.
		name := binds[i].Value
		b := binding{local: f.local(EqRef), cell: f.boxed[name], env: -1}
		if err := f.entered(binds[i+1], func() error {
			if err := f.expr(binds[i+1]); err != nil {
				return err
			}
			f.init(b)
			return nil
		}); err != nil {
			return err
		}
		f.scopes[len(f.scopes)-1][name] = b
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
		b, ok := f.lookup(n.Value)
		if !ok {
			return unsupported(n, "reference to "+n.Value)
		}
		f.load(b)
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
	case "bytes_to_string":
		if len(args) != 1 {
			return unsupported(n, h)
		}
		if err := f.expr(args[0]); err != nil {
			return err
		}
		c.Call(r.fBytesToString)
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
	if _, ok := capability.LookupHostOperation(h); ok {
		if _, std := stdOps[h]; !std {
			return f.hostCall(n, h, args)
		}
	}
	if h == "sleep" {
		return f.sleep(n)
	}
	if h == "time_now" && f.c.rt.host == HostBrowser && len(args) == 0 {
		c.Call(f.c.rt.iTimeNow)
		return nil
	}
	if h == "spawn" {
		if err := f.spawn(n); err != nil {
			return err
		}
		c.RefNull(HeapNone)
		return nil
	}
	if op, ok := stdOps[h]; ok {
		if h == "parse_json" {
			// (parse_json Type text): the type names what the program expects;
			// the VM reads the text the same way whatever it is.
			if len(args) != 2 {
				return unsupported(n, h)
			}
			args = args[1:]
		}
		return f.call(n, stdPrefix+op.module+"_"+op.fn, args)
	}
	if strings.HasPrefix(h, "__hf_") {
		if !f.c.std {
			return unsupported(n, h+" (an intrinsic of the standard library)")
		}
		return f.intrinsic(n, h, args)
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
	push := func() error {
		for _, a := range args {
			if err := f.expr(a); err != nil {
				return err
			}
		}
		return nil
	}
	if k := f.sites[n]; k > 0 {
		return f.asyncCall(k, idx, len(args), push)
	}
	if err := push(); err != nil {
		return err
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
	list := f.local(EqRef)
	length := f.local(I32)
	i := f.local(I32)
	// Resuming, the list, its length and the position are restored.
	if err := f.unlessResuming(func() error {
		if err := f.expr(n.Children[2]); err != nil {
			return err
		}
		c.LocalTee(list).RefTest(r.tList).I32Eqz().If(NoResult)
		f.labels++
		r.failWithType(c, "TYPE_ERROR: for requires a list, got ", list).Drop()
		f.check()
		f.labels--
		c.End()
		c.LocalGet(list).RefCast(r.tList).StructGet(r.tList, 0).LocalSet(length)
		c.I32Const(0).LocalSet(i)
		return nil
	}); err != nil {
		return err
	}
	f.scopes = append(f.scopes, map[string]binding{})
	item := f.declare(n.Children[1].Value)
	c.Block(NoResult).Loop(NoResult)
	f.labels += 2
	exit := f.labels - 2 // the block around the loop
	if err := f.unlessResuming(func() error {
		c.LocalGet(i).LocalGet(length).I32GeU().BrIf(f.labels - exit - 1)
		c.LocalGet(list).RefCast(r.tList).StructGet(r.tList, 1).StructGet(r.tBuf, 1).LocalGet(i).ArrayGet(r.tItems)
		f.init(item)
		return nil
	}); err != nil {
		return err
	}
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
	bound, catch, body := n.Children[1], n.Children[2], n.Children[3]
	if bound.Type != "List" || len(bound.Children) != 2 || bound.Children[0].Type != "SYMBOL" ||
		head(catch) != "catch" || len(catch.Children) != 3 || catch.Children[1].Type != "SYMBOL" {
		return unsupported(n, "try_let")
	}
	branch := func(k *ast.Node, name string, idx int, result int) error {
		f.scopes = append(f.scopes, map[string]binding{})
		defer func() { f.scopes = f.scopes[:len(f.scopes)-1] }()
		b := f.declare(name)
		if err := f.unlessResuming(func() error { c.LocalGet(idx); f.init(b); return nil }); err != nil {
			return err
		}
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
	if f.async != nil {
		// Resuming in the handler: go there.
		c.GlobalGet(r.gRewinding).If(Result(I32))
		f.resuming(catch.Children[2])
		c.Else().I32Const(0).End().BrIf(0)
	}
	f.handlers = append(f.handlers, handler)
	v := f.local(EqRef)
	if err := f.entered(bound.Children[1], func() error {
		if err := f.expr(bound.Children[1]); err != nil {
			return err
		}
		c.LocalSet(v)
		return nil
	}); err != nil {
		return err
	}
	f.handlers = f.handlers[:len(f.handlers)-1]
	if err := branch(body, bound.Children[0].Value, v, result); err != nil {
		return err
	}
	c.Br(f.labels - done - 1)
	f.labels--
	c.End()
	// The handler: take the pending error, unless it is a test's exit,
	// which passes every handler.
	if f.c.tests {
		c.GlobalGet(f.c.gExiting)
		f.propagateIf()
	}
	e := f.local(EqRef)
	if err := f.unlessResuming(func() error {
		c.GlobalGet(r.gErr).RefCast(r.tStr).Call(r.fCatchText).LocalSet(e)
		c.RefNull(HeapNone).GlobalSet(r.gErr)
		return nil
	}); err != nil {
		return err
	}
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

// intrinsic compiles a byte-level operation of the standard library:
// (__hf_type v) the VM's type name of v (as %T), (__hf_byte_len s),
// (__hf_byte_at s i), (__hf_byte_slice s from to); on 64-bit integers
// (__hf_idiv a b), (__hf_imod a b), (__hf_shl a k), (__hf_shr a k)
// (unsigned), (__hf_and a b), (__hf_or a b), (__hf_xor a b), and (__hf_f64_bits f),
// (__hf_f64_from_bits n) between a float and its IEEE 754 bits.
// (__hf_char cp) is the UTF-8 text of a code point and (__hf_nil) nil.
// (__hf_array n), (__hf_aget a i), (__hf_aset a i v) and (__hf_alen a) are
// a mutable array of n values (nil at first) for std algorithms; arrays
// never reach programs. (__hf_fail "CODE: message") raises an error.
// Standard-library code passes them the types they need.
func (f *fn) intrinsic(n *ast.Node, h string, args []*ast.Node) error {
	r := f.c.rt
	c := f.code
	want := map[string]int{"__hf_type": 1, "__hf_byte_len": 1, "__hf_byte_at": 2, "__hf_byte_slice": 3,
		"__hf_idiv": 2, "__hf_imod": 2, "__hf_shl": 2, "__hf_shr": 2, "__hf_and": 2, "__hf_or": 2, "__hf_xor": 2, "__hf_f64_bits": 1, "__hf_f64_from_bits": 1,
		"__hf_array": 1, "__hf_aget": 2, "__hf_aset": 3, "__hf_alen": 1, "__hf_fail": 1, "__hf_char": 1, "__hf_nil": 0}
	if k, ok := want[h]; !ok || k != len(args) {
		return unsupported(n, h)
	}
	for _, a := range args {
		if err := f.expr(a); err != nil {
			return err
		}
	}
	unboxInt := func() { c.RefCast(r.tInt).StructGet(r.tInt, 0).I32WrapI64() }
	switch h {
	case "__hf_type":
		c.Call(r.fGoType)
	case "__hf_byte_len":
		c.RefCast(r.tStr).StructGet(r.tStr, 0).ArrayLen().I64ExtendI32U().Call(r.fBoxInt)
	case "__hf_byte_at":
		i := f.local(EqRef)
		c.LocalSet(i).RefCast(r.tStr).StructGet(r.tStr, 0).LocalGet(i)
		unboxInt()
		c.ArrayGetU(r.tBytes).I64ExtendI32U().Call(r.fBoxInt)
	case "__hf_byte_slice":
		c.Call(r.fByteSlice)
	case "__hf_idiv", "__hf_imod", "__hf_shl", "__hf_shr", "__hf_and", "__hf_or", "__hf_xor":
		b := f.local(EqRef)
		c.LocalSet(b).RefCast(r.tInt).StructGet(r.tInt, 0)
		c.LocalGet(b).RefCast(r.tInt).StructGet(r.tInt, 0)
		switch h {
		case "__hf_idiv":
			c.I64DivS()
		case "__hf_imod":
			c.I64RemS()
		case "__hf_shl":
			c.I64Shl()
		case "__hf_shr":
			c.I64ShrU()
		case "__hf_and":
			c.I64And()
		case "__hf_or":
			c.I64Or()
		case "__hf_xor":
			c.I64Xor()
		}
		c.Call(r.fBoxInt)
	case "__hf_f64_bits":
		c.RefCast(r.tFloat).StructGet(r.tFloat, 0).I64ReinterpretF64().Call(r.fBoxInt)
	case "__hf_f64_from_bits":
		c.RefCast(r.tInt).StructGet(r.tInt, 0).F64ReinterpretI64().Call(r.fBoxFloat)
	case "__hf_array":
		unboxInt()
		c.ArrayNewDefault(r.tItems)
	case "__hf_aget", "__hf_aset":
		v := f.local(EqRef)
		if h == "__hf_aset" {
			c.LocalSet(v)
		}
		i := f.local(EqRef)
		c.LocalSet(i).RefCast(r.tItems).LocalGet(i)
		unboxInt()
		if h == "__hf_aget" {
			c.ArrayGet(r.tItems)
		} else {
			c.LocalGet(v).ArraySet(r.tItems).RefNull(HeapNone)
		}
	case "__hf_alen":
		c.RefCast(r.tItems).ArrayLen().I64ExtendI32U().Call(r.fBoxInt)
	case "__hf_fail":
		c.RefCast(r.tStr).Call(r.fFail)
		f.check()
	case "__hf_char":
		c.RefCast(r.tInt).StructGet(r.tInt, 0).I32WrapI64().Call(r.fChar)
	case "__hf_nil":
		c.RefNull(HeapNone)
	}
	return nil
}
