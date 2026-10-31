package wasmgc

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/backend/javascript"
	"github.com/howlcipher/howlframe/internal/capability"
)

// The browser host: a web_app runs in a page. Its effects on the page are
// the browser host operations of capability.HostOperations, each an import
// of the module named for the capability it needs ("hf_host" for none, else
// "hf_cap_<capability>"), so a program that is not granted a capability
// cannot be instantiated. Every import takes and returns HowlFrame values
// (eqref); the generated loader (BrowserLoader) converts them to and from
// JavaScript values and runs the JavaScript backend's runtime function for
// the operation, so both backends share one implementation and its errors.
//
// A page object (an element, an event, a subscription) is a $Host value
// holding the object as an externref. Text crosses through the exported
// linear memory as UTF-8: __hf_str_out copies a string there, __hf_str_in
// makes one from it.

// hostImportModule is the import module of a host operation.
func hostImportModule(op capability.HostOperation) string {
	if op.Capability == capability.None {
		return "hf_host"
	}
	return "hf_cap_" + string(op.Capability)
}

// browserOps returns the host operations a program calls that this backend
// imports, sorted. Operations implemented by the standard library (ZIP,
// base64) are not imports.
func browserOps(root *ast.Node) []string {
	seen := map[string]bool{}
	var walk func(n *ast.Node)
	walk = func(n *ast.Node) {
		if n == nil {
			return
		}
		if h := head(n); h != "" {
			if _, ok := capability.LookupHostOperation(h); ok {
				if _, std := stdOps[h]; !std {
					seen[h] = true
				}
			}
		}
		for _, k := range n.Children {
			walk(k)
		}
	}
	walk(root)
	ops := make([]string, 0, len(seen))
	for op := range seen {
		ops = append(ops, op)
	}
	sort.Strings(ops)
	return ops
}

// addBrowserImports declares the browser host's imports: print, stderr,
// exit_at and spawn, then one per host operation the program calls.
func (r *rt) addBrowserImports(ops []string) {
	m := r.m
	r.iPrint = m.AddImport("hf", "print", r.tEqToVoid)
	r.iStderr = m.AddImport("hf", "stderr", r.tEqToVoid)
	// exit_at(code, where) ends the program: the host throws the exit, as
	// the JavaScript backend's howlFrameExit does, naming file:line.
	r.iExitAt = m.AddImport("hf", "exit_at", r.funcType([]ValType{I32, EqRef}))
	// spawn(closure) runs a task: the host calls the closure at once, and
	// its failure is the task's alone.
	r.iSpawn = m.AddImport("hf", "spawn", r.tEqToVoid)
	// sleep(ms) waits, asynchronously.
	r.iSleep = m.AddImport("hf", "sleep", r.FuncType(1))
	// time_now() is the time in Unix seconds, as the JavaScript backend's.
	r.iTimeNow = m.AddImport("hf", "time_now", r.FuncType(0))
	r.iHost = map[string]int{}
	for _, name := range ops {
		op, _ := capability.LookupHostOperation(name)
		// An operation with optional parameters also receives how many
		// arguments the call gave.
		arity := len(op.Params)
		if op.Optional > 0 {
			arity++
		}
		r.iHost[name] = m.AddImport(hostImportModule(op), name, r.FuncType(arity))
	}
	m.MemoryPages, m.MemoryExport = 1, "memory"
}

// emitBrowser writes the exports the loader converts values with.
func (r *rt) emitBrowser() {
	m := r.m
	export := func(name string, params []ValType, results []ValType, locals []ValType, body func(c *Code)) {
		idx := r.def(name, r.funcType(params, results...), locals, body)
		m.Exports = append(m.Exports, Export{name, idx})
	}
	eq := []ValType{EqRef}
	// __hf_kind(v): 0 nil, 1 bool, 2 int, 3 float, 4 string, 5 list,
	// 6 dict, 7 host object, 8 anything else.
	export("__hf_kind", eq, []ValType{I32}, nil, func(c *Code) {
		c.LocalGet(0).RefIsNull().If(NoResult).I32Const(0).Return().End()
		for i, t := range []int{r.tBool, r.tInt, r.tFloat, r.tStr, r.tList, r.tDict, r.tHost} {
			c.LocalGet(0).RefTest(t).If(NoResult).I32Const(int32(i + 1)).Return().End()
		}
		c.I32Const(8)
	})
	export("__hf_bool_of", eq, []ValType{I32}, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tBool).StructGet(r.tBool, 0)
	})
	export("__hf_int_of", eq, []ValType{F64}, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tInt).StructGet(r.tInt, 0).F64ConvertI64S()
	})
	export("__hf_float_of", eq, []ValType{F64}, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tFloat).StructGet(r.tFloat, 0)
	})
	export("__hf_bool", []ValType{I32}, eq, nil, func(c *Code) { c.LocalGet(0).Call(r.fBoxBool) })
	// __hf_int(n): n is a safe integer (the loader checks).
	export("__hf_int", []ValType{F64}, eq, nil, func(c *Code) { c.LocalGet(0).I64TruncSatF64S().Call(r.fBoxInt) })
	export("__hf_float", []ValType{F64}, eq, nil, func(c *Code) { c.LocalGet(0).Call(r.fBoxFloat) })
	// __hf_str_out(s): copies s's bytes to memory from 0, growing it as
	// needed, and returns their count.
	// locals: 0 s, 1 bytes, 2 n, 3 i
	export("__hf_str_out", eq, []ValType{I32}, []ValType{RefNull(r.tBytes), I32, I32}, func(c *Code) {
		c.LocalGet(0).RefCast(r.tStr).StructGet(r.tStr, 0).LocalTee(1).ArrayLen().LocalSet(2)
		r.reserve(c, 2)
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(3).LocalGet(2).I32GeU().BrIf(1)
		c.LocalGet(3).LocalGet(1).LocalGet(3).ArrayGetU(r.tBytes).I32Store8(0)
		c.LocalGet(3).I32Const(1).I32Add().LocalSet(3).Br(0)
		c.End().End()
		c.LocalGet(2)
	})
	// __hf_str_in(n): a string of the n bytes at 0 (the loader has made
	// room for them).
	// locals: 0 n, 1 bytes, 2 i
	export("__hf_str_in", []ValType{I32}, eq, []ValType{RefNull(r.tBytes), I32}, func(c *Code) {
		c.LocalGet(0).ArrayNewDefault(r.tBytes).LocalSet(1)
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(2).LocalGet(0).I32GeU().BrIf(1)
		c.LocalGet(1).LocalGet(2).LocalGet(2).I32Load8U(0).ArraySet(r.tBytes)
		c.LocalGet(2).I32Const(1).I32Add().LocalSet(2).Br(0)
		c.End().End()
		c.LocalGet(1).RefAsNonNull().StructNew(r.tStr)
	})
	// __hf_bytes_in(n): the n bytes at 0 as a list of byte values.
	// locals: 0 n, 1 items, 2 i
	export("__hf_bytes_in", []ValType{I32}, eq, []ValType{RefNull(r.tItems), I32}, func(c *Code) {
		c.LocalGet(0).ArrayNewDefault(r.tItems).LocalSet(1)
		c.Block(NoResult).Loop(NoResult)
		c.LocalGet(2).LocalGet(0).I32GeU().BrIf(1)
		c.LocalGet(1).LocalGet(2).LocalGet(2).I32Load8U(0).I64ExtendI32U().Call(r.fBoxInt).ArraySet(r.tItems)
		c.LocalGet(2).I32Const(1).I32Add().LocalSet(2).Br(0)
		c.End().End()
		c.LocalGet(1).RefAsNonNull().Call(r.fListNew)
	})
	export("__hf_list_size", eq, []ValType{I32}, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tList).StructGet(r.tList, 0)
	})
	export("__hf_list_at", []ValType{EqRef, I32}, eq, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tList).StructGet(r.tList, 1).StructGet(r.tBuf, 1).LocalGet(1).ArrayGet(r.tItems)
	})
	export("__hf_list_new", nil, eq, nil, func(c *Code) {
		c.ArrayNewFixed(r.tItems, 0).Call(r.fListNew)
	})
	// __hf_list_push(l, v): l with v appended (the loader keeps the result).
	export("__hf_list_push", []ValType{EqRef, EqRef}, eq, nil, func(c *Code) {
		c.LocalGet(0).LocalGet(1).Call(r.fListAppend)
	})
	export("__hf_dict_size", eq, []ValType{I32}, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tDict).StructGet(r.tDict, 0)
	})
	export("__hf_dict_key", []ValType{EqRef, I32}, eq, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tDict).StructGet(r.tDict, 1).LocalGet(1).ArrayGet(r.tItems)
	})
	export("__hf_dict_value", []ValType{EqRef, I32}, eq, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tDict).StructGet(r.tDict, 2).LocalGet(1).ArrayGet(r.tItems)
	})
	export("__hf_dict_new", nil, eq, nil, func(c *Code) {
		c.ArrayNewFixed(r.tItems, 0).Call(r.fDictNew)
	})
	export("__hf_dict_put", []ValType{EqRef, EqRef, EqRef}, nil, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tDict).LocalGet(1).RefCast(r.tStr).LocalGet(2).Call(r.fDictPut)
	})
	export("__hf_host", []ValType{ExternRef}, eq, nil, func(c *Code) {
		c.LocalGet(0).StructNew(r.tHost)
	})
	export("__hf_host_of", eq, []ValType{ExternRef}, nil, func(c *Code) {
		c.LocalGet(0).RefCast(r.tHost).StructGet(r.tHost, 0)
	})
	// __hf_fail(message): the pending error, "CODE: message"; nil clears it.
	export("__hf_fail", eq, nil, nil, func(c *Code) { c.LocalGet(0).GlobalSet(r.gErr) })
	r.emitInvoke()
	r.emitAsync()
}

// reserve grows memory, when needed, to hold as many bytes as the i32 in
// local n.
func (r *rt) reserve(c *Code, n int) {
	c.LocalGet(n).MemorySize().I32Const(16).I32Shl().I32GtU().If(NoResult)
	c.LocalGet(n).I32Const(0xFFFF).I32Add().I32Const(16).I32ShrU().MemorySize().I32Sub().MemoryGrow().Drop()
	c.End()
}

// hostCall compiles a call of a browser host operation: its arguments as
// values, the import, and the error check.
func (f *fn) hostCall(n *ast.Node, h string, args []*ast.Node) error {
	idx, ok := f.c.rt.iHost[h]
	if !ok {
		return unsupported(n, h+" (a browser host operation: it runs in a web_app built for the browser)")
	}
	op, _ := capability.LookupHostOperation(h)
	if !op.Arity(len(args)) {
		return unsupported(n, h)
	}
	push := func() error {
		for i, a := range args {
			if op.Params[i].Kind == capability.HostLambda {
				if err := f.closure(a); err != nil {
					return err
				}
				continue
			}
			if err := f.expr(a); err != nil {
				return err
			}
		}
		if op.Optional > 0 {
			for range op.Params[len(args):] {
				f.code.RefNull(HeapNone)
			}
			f.code.I64Const(int64(len(args))).Call(f.c.rt.fBoxInt)
		}
		return nil
	}
	if op.Async {
		return f.asyncHostCall(n, f.sites[n], idx, push)
	}
	if err := push(); err != nil {
		return err
	}
	f.code.Call(idx)
	f.check()
	return nil
}

// sleep compiles (sleep ms), an asynchronous operation of the host.
func (f *fn) sleep(n *ast.Node) error {
	if f.c.rt.host != HostBrowser || len(n.Children) != 2 {
		return unsupported(n, "sleep")
	}
	return f.asyncHostCall(n, f.sites[n], f.c.rt.iSleep, func() error { return f.expr(n.Children[1]) })
}

// spawn compiles (spawn (lambda () body)): the host runs the closure as a
// task.
func (f *fn) spawn(n *ast.Node) error {
	if f.c.rt.host != HostBrowser || len(n.Children) != 2 {
		return unsupported(n, "spawn")
	}
	if err := f.closure(n.Children[1]); err != nil {
		return err
	}
	f.code.Call(f.c.rt.iSpawn)
	return nil
}

// CompileBrowser compiles a web_app for the browser: the module and the
// loader (app.js) that runs it in a page.
func CompileBrowser(root *ast.Node) ([]byte, string, error) {
	if rootHead(root) != "web_app" {
		return nil, "", fmt.Errorf("WASM_UNSUPPORTED: the browser host runs a web_app, not %q", rootHead(root))
	}
	ops := browserOps(root)
	wasm, err := compile(root, HostBrowser, false)
	if err != nil {
		return nil, "", err
	}
	loader, err := BrowserLoader(ops)
	if err != nil {
		return nil, "", err
	}
	return wasm, loader, nil
}

// CompileBrowserInline compiles a web_app for the browser into one classic
// script: the loader with the module's bytes inside it (base64), so a page
// loads nothing else. A page's policy needs 'wasm-unsafe-eval' in
// script-src, and no connect-src (the plan's "Site delivery").
func CompileBrowserInline(root *ast.Node) (string, error) {
	if rootHead(root) != "web_app" {
		return "", fmt.Errorf("WASM_UNSUPPORTED: the browser host runs a web_app, not %q", rootHead(root))
	}
	ops := browserOps(root)
	wasm, err := compile(root, HostBrowser, false)
	if err != nil {
		return "", err
	}
	return browserLoader(ops, inlineSource(wasm), "the module inside it")
}

// BrowserLoader returns the loader of a web_app that calls the given host
// operations: a classic script (app.js) that instantiates app.wasm against
// the page and runs the program. The module's bytes come from the global
// howlFrameWasm when a page (or a test) defines it, otherwise from app.wasm
// beside the script. The script's value is a promise that settles when the
// program's top level has run, rejected with its uncaught error.
func BrowserLoader(ops []string) (string, error) {
	return browserLoader(ops, browserFileSource, "app.wasm")
}

// browserFileSource is where a loader finds a module built beside it.
const browserFileSource = `const howlFrameSource = typeof howlFrameWasm !== "undefined" ? howlFrameWasm :
  new URL("app.wasm", typeof document !== "undefined" && document.currentScript ? document.currentScript.src : location.href);
async function howlFrameBytes(source) {
  if (typeof URL === "function" && source instanceof URL) return (await fetch(source)).arrayBuffer();
  return source;
}
`

// inlineSource holds a module's bytes in the loader, as base64 decoded once
// at load. Base64 has no character that could end an inline script element.
func inlineSource(wasm []byte) string {
	return `const howlFrameSource = (() => {
  const s = atob("` + base64.StdEncoding.EncodeToString(wasm) + `"), b = new Uint8Array(s.length);
  for (let i = 0; i < s.length; i++) b[i] = s.charCodeAt(i);
  return b;
})();
const howlFrameBytes = (source) => source;
`
}

// browserLoader is a loader that takes the module's bytes from source: the
// definitions of howlFrameSource (a URL to fetch, or the bytes) and of
// howlFrameBytes, which gives the bytes of a source.
func browserLoader(ops []string, source, module string) (string, error) {
	helpers, runtime, err := javascript.HostRuntime(ops)
	if err != nil {
		return "", err
	}
	var imports strings.Builder
	modules := map[string][]string{}
	for _, name := range ops {
		op, _ := capability.LookupHostOperation(name)
		mod := hostImportModule(op)
		var lambdas []string
		for _, p := range op.Params {
			if p.Kind != capability.HostLambda {
				lambdas = append(lambdas, "null")
				continue
			}
			kinds := make([]string, len(p.Lambda))
			for i, q := range p.Lambda {
				kinds[i] = fmt.Sprintf("%q", string(q.Kind))
			}
			lambdas = append(lambdas, "["+strings.Join(kinds, ", ")+"]")
		}
		wrapper := "hostOp"
		if op.Async {
			wrapper = "asyncOp"
		}
		if op.Optional > 0 {
			lambdas = append(lambdas, "\"count\"")
		}
		modules[mod] = append(modules[mod], fmt.Sprintf("    %s: %s(%s, %q, [%s]),", name, wrapper, helpers[name], string(op.Result), strings.Join(lambdas, ", ")))
	}
	mods := make([]string, 0, len(modules))
	for mod := range modules {
		mods = append(mods, mod)
	}
	sort.Strings(mods)
	for _, mod := range mods {
		cap := strings.TrimPrefix(mod, "hf_cap_")
		if mod == "hf_host" {
			fmt.Fprintf(&imports, "  imports.%s = {\n%s\n  };\n", mod, strings.Join(modules[mod], "\n"))
		} else {
			// Granted capabilities only: without the grant the module does
			// not link.
			fmt.Fprintf(&imports, "  if (howlFrameGranted(%q)) imports.%s = {\n%s\n  };\n", cap, mod, strings.Join(modules[mod], "\n"))
		}
	}
	return `// Generated by the HowlFrame compiler. Runs a web_app (` + module + `) in a page.
(() => {
"use strict";
` + runtime + javascript.OutputRuntime() + source + browserLoaderCore + imports.String() + browserLoaderRun, nil
}

// browserLoaderCore converts values between the module and JavaScript.
const browserLoaderCore = `let ex;
const encoder = new TextEncoder();
const decoder = new TextDecoder();
function fromStr(s) {
  const n = ex.__hf_str_out(s);
  return decoder.decode(new Uint8Array(ex.memory.buffer, 0, n));
}
function toStr(s) {
  const bytes = encoder.encode(s);
  const need = bytes.length - ex.memory.buffer.byteLength;
  if (need > 0) ex.memory.grow(Math.ceil(need / 65536));
  new Uint8Array(ex.memory.buffer, 0, bytes.length).set(bytes);
  return ex.__hf_str_in(bytes.length);
}
// A HowlFrame value as JavaScript: lists are arrays, dicts plain objects,
// page objects themselves.
function toJS(v) {
  switch (ex.__hf_kind(v)) {
    case 0: return null;
    case 1: return ex.__hf_bool_of(v) === 1;
    case 2: return ex.__hf_int_of(v);
    case 3: return ex.__hf_float_of(v);
    case 4: return fromStr(v);
    case 5: { const n = ex.__hf_list_size(v), a = new Array(n); for (let i = 0; i < n; i++) a[i] = toJS(ex.__hf_list_at(v, i)); return a; }
    case 6: { const n = ex.__hf_dict_size(v), o = {}; for (let i = 0; i < n; i++) o[fromStr(ex.__hf_dict_key(v, i))] = toJS(ex.__hf_dict_value(v, i)); return o; }
    case 7: return ex.__hf_host_of(v);
  }
  return v;
}
// A JavaScript value as HowlFrame: whole safe numbers are ints, other
// numbers floats; arrays lists, plain objects dicts, anything else a page
// object.
function toWasm(x) {
  if (x === null || x === undefined) return null;
  switch (typeof x) {
    case "boolean": return ex.__hf_bool(x ? 1 : 0);
    case "number": return Number.isSafeInteger(x) ? ex.__hf_int(x) : ex.__hf_float(x);
    case "string": return toStr(x);
  }
  if (Array.isArray(x)) { let l = ex.__hf_list_new(); for (const e of x) l = ex.__hf_list_push(l, toWasm(e)); return l; }
  const proto = Object.getPrototypeOf(x);
  if (proto === Object.prototype || proto === null) {
    const d = ex.__hf_dict_new();
    for (const k of Object.keys(x)) ex.__hf_dict_put(d, toStr(k), toWasm(x[k]));
    return d;
  }
  return ex.__hf_host(x);
}
// The result of a host operation, by its kind in the operation table: a
// page object, an element list, or any value.
function resultOf(kind, x) {
  switch (kind) {
    case "": return null;
    case "element?": case "event": case "subscription": return x === null || x === undefined ? null : ex.__hf_host(x);
    case "elements": { let l = ex.__hf_list_new(); for (const e of x) l = ex.__hf_list_push(l, ex.__hf_host(e)); return l; }
    case "bytes": {
      // The VM's bytes: a list of byte values. Text is its UTF-8.
      const bytes = typeof x === "string" ? encoder.encode(x) : new Uint8Array(x);
      const need = bytes.length - ex.memory.buffer.byteLength;
      if (need > 0) ex.memory.grow(Math.ceil(need / 65536));
      new Uint8Array(ex.memory.buffer, 0, bytes.length).set(bytes);
      return ex.__hf_bytes_in(bytes.length);
    }
  }
  return toWasm(x);
}
// A thrown error becomes the module's pending error, "CODE: message"; an
// exit passes through. The last one is kept, so that if no try_let catches
// it the program fails with the host's own error, as on the JavaScript
// backend.
let hostError = null;
function fail(e) {
  if (e && e.howlFrameExit) throw e;
  let m = e && e.message !== undefined ? String(e.message) : String(e);
  if (!/^[A-Z][A-Z0-9_]*: /.test(m)) m = "RUNTIME_ERROR: " + (e && e.name ? e.name + ": " : "") + m;
  hostError = { text: m, error: e };
  ex.__hf_fail(toStr(m));
}
// The uncaught error a call of the module left, as a JavaScript error.
function uncaught(err) {
  const t = fromStr(err);
  return hostError !== null && hostError.text === t ? hostError.error : new Error(t);
}
// A HowlFrame value passed to the page, by its kind: a page object, or any
// value.
function argOf(kind, v) {
  if (kind === "event" || kind === "element") return v === null || v === undefined ? null : ex.__hf_host(v);
  return toWasm(v);
}
// The asynchronous operation the running fiber is suspended on.
let pending = null;
// task runs an entry of the module (main, or a handler) as a task, as the
// JavaScript backend runs an async function: each time it suspends on an
// asynchronous operation, task waits for the operation and calls the entry
// again to resume it. Its promise settles when the entry finishes, rejected
// with its uncaught error or exit; meanwhile other tasks run.
async function task(enter) {
  for (;;) {
    enter();
    if (!ex.__hf_unwinding()) {
      const err = ex.__hf_error();
      if (err === null) return;
      ex.__hf_fail(null);
      throw uncaught(err);
    }
    const fiber = ex.__hf_take_fiber();
    const op = pending;
    pending = null;
    let value = null;
    try { value = resultOf(op.kind, await op.promise); } catch (e) { fail(e); }
    ex.__hf_resume(fiber, value);
  }
}
// run calls a closure with JavaScript arguments as a task of its own, like
// a JavaScript backend handler (an async function): its promise rejects
// with the task's uncaught error or exit, and the caller carries on.
function run(closure, args) {
  const a0 = args.length > 0 ? args[0] : null, a1 = args.length > 1 ? args[1] : null;
  return task(() => { ex.__hf_invoke(closure, a0, a1); });
}
// handler makes a closure a JavaScript function for the page to call.
function handler(closure, kinds) {
  return function () {
    const args = [];
    for (let i = 0; i < kinds.length; i++) args.push(argOf(kinds[i], arguments[i]));
    return run(closure, args);
  };
}
// argsOf converts an import's arguments: a lambda to a handler, and for an
// operation with optional parameters the last argument, how many the call
// gave, to the length of the argument list.
function argsOf(raw, lambdas) {
  let args = [];
  for (let i = 0; i < raw.length; i++) {
    if (lambdas[i] === "count") { args = args.slice(0, toJS(raw[i])); break; }
    args.push(lambdas[i] ? handler(raw[i], lambdas[i]) : toJS(raw[i]));
  }
  return args;
}
// asyncOp is the import of an asynchronous host operation: it starts the
// operation (a failure to start is the module's pending error) and leaves
// it for task to wait on while the fiber suspends.
function asyncOp(fn, kind, lambdas) {
  return function () {
    const args = argsOf(arguments, lambdas);
    let p;
    try { p = fn.apply(null, args); } catch (e) { fail(e); return null; }
    pending = { promise: Promise.resolve(p), kind };
    return null;
  };
}
// hostOp is the import of a host operation: its arguments as JavaScript
// (a lambda as a handler), the JavaScript backend's runtime function, and
// its result or error back as HowlFrame.
function hostOp(fn, kind, lambdas) {
  return function () {
    const args = argsOf(arguments, lambdas);
    try { return resultOf(kind, fn.apply(null, args)); } catch (e) { fail(e); return null; }
  };
}
// howlFrameGranted reports whether the runner granted a capability, read as
// the JavaScript backend reads grants: HOWLFRAME_ALLOW_CAPS from the
// environment in Node, otherwise the page's global. A narrower grant
// (network:fetch=host) grants the capability; the operation checks the rest.
function howlFrameGranted(name) {
  let raw = "";
  if (typeof process !== "undefined" && process.env) raw = process.env.HOWLFRAME_ALLOW_CAPS || "";
  else if (typeof globalThis !== "undefined") raw = globalThis.HOWLFRAME_ALLOW_CAPS;
  if (typeof raw !== "string") return false;
  return raw.split(",").some((g) => g.trim() === name || g.trim().startsWith(name + ":"));
}
const imports = {
  hf: {
    print: (s) => { console.log(fromStr(s)); },
    stderr: (s) => { howlFrameStderr(fromStr(s)); },
    exit_at: (code, where) => { howlFrameExit(code, fromStr(where)); },
    spawn: (closure) => { run(closure, []); },
    sleep: asyncOp((ms) => new Promise((r) => setTimeout(r, ms)), "", [null]),
    time_now: () => toWasm(Math.floor(Date.now() / 1000)),
  },
};
`

// browserLoaderRun instantiates the module and runs the program.
const browserLoaderRun = `return (async () => {
  const module = await WebAssembly.compile(await howlFrameBytes(howlFrameSource));
  // A program that needs a capability the runner did not grant does not
  // link; it fails as a denied operation does on the JavaScript backend.
  for (const i of WebAssembly.Module.imports(module)) {
    if (i.module.startsWith("hf_cap_") && !imports[i.module]) throw new Error("CAPABILITY_DENIED: capability denied: " + i.module.slice(7));
  }
  const instance = await WebAssembly.instantiate(module, imports);
  ex = instance.exports;
  await task(() => { ex.main(); });
})();
})();
`
