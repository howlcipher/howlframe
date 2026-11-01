package wasmgc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runWasm instantiates a module in Node (V8 with WasmGC) and calls its
// exported main, printing what main returns; it also validates the module
// with wasm-tools when that is installed. Returns Node's output.
func runWasm(t *testing.T, wasm []byte, imports string) string {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "m.wasm")
	if err := os.WriteFile(path, wasm, 0o644); err != nil {
		t.Fatal(err)
	}
	if tools, err := exec.LookPath("wasm-tools"); err == nil {
		if out, err := exec.Command(tools, "validate", "--features", "all", path).CombinedOutput(); err != nil {
			t.Fatalf("wasm-tools rejects the module: %v\n%s", err, out)
		}
	}
	script := `const fs = require("fs");
const imports = ` + imports + `;
WebAssembly.instantiate(fs.readFileSync(process.argv[2]), imports).then(({ instance }) => {
  if (instance.exports.main) console.log(String(instance.exports.main()));
}, (e) => { console.log("ERROR " + e.message); process.exit(1); });`
	js := filepath.Join(dir, "run.js")
	os.WriteFile(js, []byte(script), 0o644)
	out, err := exec.Command("node", js, path).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestEncoderBuildsAValidModuleWithArithmetic(t *testing.T) {
	m := NewModule()
	fn := m.AddType(TypeDef{IsFunc: true, Results: []ValType{I64}})
	body := NewCode().I64Const(40).I64Const(2).I64Add()
	main := m.AddFunc(Func{Name: "main", Type: fn, Body: body})
	m.Exports = append(m.Exports, Export{"main", main})
	wasm, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if got := runWasm(t, wasm, "{}"); got != "42" {
		t.Fatalf("got %q", got)
	}
}

// Structs, arrays, passive data, casts and loops together: build a boxed
// integer, a byte array from a data segment, and sum the bytes in a loop.
func TestEncoderGCTypesDataAndLoops(t *testing.T) {
	m := NewModule()
	tBytes := m.AddType(TypeDef{Name: "Bytes", Array: &Field{Type: PackedI8}})
	tInt := m.AddType(TypeDef{Name: "Int", Struct: []Field{{Type: I64}}})
	fn := m.AddType(TypeDef{IsFunc: true, Results: []ValType{I64}})
	data := m.AddData([]byte{1, 2, 3, 250})
	// locals: 0 bytes, 1 i, 2 sum, 3 boxed
	body := NewCode().
		I32Const(0).I32Const(4).ArrayNewData(tBytes, data).LocalSet(0).
		I64Const(0).LocalSet(2).
		Block(NoResult).Loop(NoResult).
		LocalGet(1).LocalGet(0).ArrayLen().I32GeU().BrIf(1).
		LocalGet(2).LocalGet(0).LocalGet(1).ArrayGetU(tBytes).I64ExtendI32U().I64Add().LocalSet(2).
		LocalGet(1).I32Const(1).I32Add().LocalSet(1).
		Br(0).
		End().End().
		LocalGet(2).StructNew(tInt).LocalSet(3).
		LocalGet(3).RefCast(tInt).StructGet(tInt, 0)
	main := m.AddFunc(Func{Name: "main", Type: fn, Locals: []ValType{RefNull(tBytes), I32, I64, EqRef}, Body: body})
	m.Exports = append(m.Exports, Export{"main", main})
	wasm, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if got := runWasm(t, wasm, "{}"); got != "256" {
		t.Fatalf("got %q", got)
	}
}

func TestEncoderImportsCallTheHost(t *testing.T) {
	m := NewModule()
	fnI := m.AddType(TypeDef{IsFunc: true, Params: []ValType{I64}})
	fnV := m.AddType(TypeDef{IsFunc: true})
	log := m.AddImport("host", "log", fnI)
	run := m.AddFunc(Func{Name: "run", Type: fnV, Body: NewCode().I64Const(-7).Call(log)})
	m.Exports = append(m.Exports, Export{"main", run})
	wasm, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	got := runWasm(t, wasm, `{ host: { log: (v) => console.log("host got " + v) } }`)
	if got != "host got -7\nundefined" {
		t.Fatalf("got %q", got)
	}
}

func TestEncoderReportsUnbalancedBlocks(t *testing.T) {
	m := NewModule()
	fn := m.AddType(TypeDef{IsFunc: true})
	m.AddFunc(Func{Name: "bad", Type: fn, Body: NewCode().Block(NoResult)})
	if _, err := m.Bytes(); err == nil || !strings.Contains(err.Error(), "unbalanced") {
		t.Fatalf("want an unbalanced-block error, got %v", err)
	}
}

func TestLEB128(t *testing.T) {
	for v, want := range map[int64]string{0: "00", 63: "3f", 64: "c000", -1: "7f", -64: "40", -65: "bf7f", 624485: "e58e26"} {
		got := ""
		for _, b := range sleb(v) {
			got += hex(b)
		}
		if got != want {
			t.Errorf("sleb(%d) = %s, want %s", v, got, want)
		}
	}
	if got := uleb(624485); string(got) != string([]byte{0xE5, 0x8E, 0x26}) {
		t.Errorf("uleb(624485) = %x", got)
	}
}

func hex(b byte) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[b>>4], digits[b&15]})
}

// A WASI import's signature must be the very type the host defines, so the
// encoder writes standalone types as their own recursion groups ahead of the
// module's one group: wasmtime refuses fd_write's type inside the group.
func TestStandaloneTypesComeBeforeTheRecursionGroup(t *testing.T) {
	m := NewModule()
	m.AddType(TypeDef{Name: "fd_write", IsFunc: true, Standalone: true, Params: []ValType{I32, I32, I32, I32}, Results: []ValType{I32}})
	str := m.AddType(TypeDef{Name: "Bytes", Array: &Field{Type: PackedI8, Mutable: true}})
	m.AddType(TypeDef{Name: "Str", Struct: []Field{{Type: Ref(str)}}})
	wasm, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	// Header, then section 1: size, two entries (the standalone function
	// type, then a recursion group of the other two).
	types := wasm[8:]
	if types[0] != 1 {
		t.Fatalf("first section is %d, want the type section", types[0])
	}
	body := types[2:]
	if body[0] != 2 || body[1] != 0x60 {
		t.Fatalf("type section starts % x, want two entries, the first a plain func type", body[:2])
	}
	if !strings.Contains(string(body), "\x4e\x02") {
		t.Fatalf("no recursion group of two types in % x", body)
	}

	bad := NewModule()
	bad.AddType(TypeDef{Name: "Bytes", Array: &Field{Type: PackedI8, Mutable: true}})
	bad.AddType(TypeDef{Name: "late", IsFunc: true, Standalone: true})
	if _, err := bad.Bytes(); err == nil || !strings.Contains(err.Error(), "standalone types must come first") {
		t.Fatalf("a standalone type after the group: got %v", err)
	}
}

// Functions nothing reaches are left out and the rest renumbered: calls,
// ref.func, exports and the element segment all name the right functions.
// Locals of one type share an entry.
func TestEncoderLeavesOutUnreachedFunctions(t *testing.T) {
	m := NewModule()
	fn := m.AddType(TypeDef{IsFunc: true, Results: []ValType{I64}})
	m.AddImport("env", "unused", fn)
	dead := m.AddFunc(Func{Name: "dead", Type: fn, Body: NewCode().I64Const(1)})
	m.AddFunc(Func{Name: "calls dead", Type: fn, Body: NewCode().Call(dead)})
	add := m.AddFunc(Func{Name: "add", Type: fn, Locals: []ValType{I64, I64, I32}, Body: NewCode().
		I64Const(40).LocalSet(0).I64Const(2).LocalSet(1).I32Const(0).LocalSet(2).LocalGet(0).LocalGet(1).I64Add()})
	viaRef := m.AddFunc(Func{Name: "via ref", Type: fn, Body: NewCode().Call(add)})
	main := m.AddFunc(Func{Name: "main", Type: fn, Body: NewCode().RefFunc(viaRef).CallRef(fn)})
	m.Exports = append(m.Exports, Export{"main", main})
	live, err := m.live()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range live.Funcs {
		names = append(names, f.Name)
	}
	if got := strings.Join(names, ", "); got != "add, via ref, main" {
		t.Fatalf("live functions: %s", got)
	}
	if len(m.Funcs) != 5 {
		t.Fatal("live changed the module")
	}
	wasm, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if got := runWasm(t, wasm, `{ env: { unused: () => 0n } }`); got != "42" {
		t.Fatalf("got %q", got)
	}
}
