package wasmgc

import (
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

// browserUnsupported names host operations the browser host does not
// implement yet; they fail closed at build time.
var browserUnsupported = map[string]string{}

func init() {
	for _, op := range capability.HostOperations() {
		switch {
		case op.LambdaLast():
			browserUnsupported[op.Name] = "a handler (lambda)"
		case op.Async:
			if _, std := stdOps[op.Name]; !std {
				browserUnsupported[op.Name] = "an asynchronous operation"
			}
		}
	}
}

// addBrowserImports declares the browser host's imports: print, stderr and
// exit_at, then one per host operation the program calls.
func (r *rt) addBrowserImports(ops []string) {
	m := r.m
	r.iPrint = m.AddImport("hf", "print", r.tEqToVoid)
	r.iStderr = m.AddImport("hf", "stderr", r.tEqToVoid)
	// exit_at(code, where) ends the program: the host throws the exit, as
	// the JavaScript backend's howlFrameExit does, naming file:line.
	r.iExitAt = m.AddImport("hf", "exit_at", r.funcType([]ValType{I32, EqRef}))
	r.iHost = map[string]int{}
	for _, name := range ops {
		op, _ := capability.LookupHostOperation(name)
		r.iHost[name] = m.AddImport(hostImportModule(op), name, r.FuncType(len(op.Params)))
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
	// __hf_fail(message): the pending error, "CODE: message".
	export("__hf_fail", eq, nil, nil, func(c *Code) { c.LocalGet(0).GlobalSet(r.gErr) })
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
	if why, ok := browserUnsupported[h]; ok {
		return unsupported(n, h+" ("+why+")")
	}
	idx, ok := f.c.rt.iHost[h]
	if !ok {
		return unsupported(n, h+" (a browser host operation; it needs a web_app)")
	}
	op, _ := capability.LookupHostOperation(h)
	if len(args) != len(op.Params) {
		return unsupported(n, h)
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

// BrowserLoader returns the loader of a web_app that calls the given host
// operations: a classic script (app.js) that instantiates app.wasm against
// the page and runs the program. The module's bytes come from the global
// howlFrameWasm when a page (or a test) defines it, otherwise from app.wasm
// beside the script. The script's value is a promise that settles when the
// program's top level has run, rejected with its uncaught error.
func BrowserLoader(ops []string) (string, error) {
	helpers, runtime, err := javascript.HostRuntime(ops)
	if err != nil {
		return "", err
	}
	var imports strings.Builder
	modules := map[string][]string{}
	for _, name := range ops {
		op, _ := capability.LookupHostOperation(name)
		mod := hostImportModule(op)
		modules[mod] = append(modules[mod], fmt.Sprintf("    %s: hostOp(%s, %q),", name, helpers[name], string(op.Result)))
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
	return `// Generated by the HowlFrame compiler. Runs app.wasm (a web_app) in a page.
(() => {
"use strict";
` + runtime + javascript.OutputRuntime() + browserLoaderCore + imports.String() + browserLoaderRun, nil
}

// browserLoaderCore converts values between the module and JavaScript.
const browserLoaderCore = `const howlFrameSource = typeof howlFrameWasm !== "undefined" ? howlFrameWasm :
  new URL("app.wasm", typeof document !== "undefined" && document.currentScript ? document.currentScript.src : location.href);
let ex;
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
function hostOp(fn, kind) {
  return function () {
    const args = Array.prototype.map.call(arguments, toJS);
    try { return resultOf(kind, fn.apply(null, args)); } catch (e) { fail(e); return null; }
  };
}
function howlFrameGranted(name) {
  const raw = typeof HOWLFRAME_ALLOW_CAPS === "string" ? HOWLFRAME_ALLOW_CAPS : "";
  return raw.split(",").some((g) => g.trim() === name || g.trim().startsWith(name + ":"));
}
const imports = {
  hf: {
    print: (s) => { console.log(fromStr(s)); },
    stderr: (s) => { howlFrameStderr(fromStr(s)); },
    exit_at: (code, where) => { howlFrameExit(code, fromStr(where)); },
  },
};
`

// browserLoaderRun instantiates the module and runs the program.
const browserLoaderRun = `async function howlFrameBytes(source) {
  if (typeof URL !== "undefined" && source instanceof URL) return (await fetch(source)).arrayBuffer();
  return source;
}
return (async () => {
  const { instance } = await WebAssembly.instantiate(await howlFrameBytes(howlFrameSource), imports);
  ex = instance.exports;
  ex.main();
  const err = ex.__hf_error();
  if (err !== null) throw uncaught(err);
})();
})();
`
