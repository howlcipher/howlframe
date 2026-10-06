package vm

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// This is a source conformance check, not a general Go effect/type analysis.
// Follow same-file functions and methods transitively, including closures and
// goroutines. Recursive dispatch (run) is a boundary: its cases are independently
// audited here and its central gate is checked below. Calls into other files or
// packages are outside this scan; file:// stores retain their dedicated tests.
func TestEffectGateSourceConformance(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "vm.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	render := func(n ast.Node) string {
		var b bytes.Buffer
		if err := format.Node(&b, fset, n); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	packages := map[string]bool{}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		packages[name] = true
	}
	funcs := map[string][]*ast.BlockStmt{}
	var dispatch *ast.SwitchStmt
	var dispatchBlock *ast.BlockStmt
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		funcs[fn.Name.Name] = append(funcs[fn.Name.Name], fn.Body)
		if fn.Name.Name == "run" {
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				block, ok := n.(*ast.BlockStmt)
				if !ok {
					return true
				}
				for _, stmt := range block.List {
					sw, ok := stmt.(*ast.SwitchStmt)
					if ok && render(sw.Tag) == "inst.Op" {
						dispatch = sw
						dispatchBlock = block
					}
				}
				return true
			})
		}
	}
	if dispatch == nil {
		t.Fatal("missing bytecode dispatch switch")
	}
	gated := false
	registryLookup := false
	for _, stmt := range dispatchBlock.List {
		if stmt == dispatch {
			break
		}
		if render(stmt) == "spec := bytecode.Registry[inst.Op]" {
			registryLookup = true
		}
		cond, ok := stmt.(*ast.IfStmt)
		if !ok {
			continue
		}
		if render(cond.Cond) == "spec.Capability != capability.None" && render(cond.Body) == "{\n\tvm.requireCapability(spec.Capability, inst.Op)\n}" {
			gated = true
		}
	}
	if !gated || !registryLookup {
		t.Error("missing central capability gate before dispatch")
	}
	// Only entries actually seen by the call scan belong here. Ambient stream,
	// clock and exit instructions currently make no targeted package calls.
	// CALL is conditional authority: lazy synthesis checks network in its case.
	allowed := map[string]string{"OpCall": "gated inside case: lazy_synthesize"}
	seen := map[string]bool{}
	for _, stmt := range dispatch.Body.List {
		clause := stmt.(*ast.CaseClause)
		effects := map[string]bool{}
		visited := map[string]bool{"run": true}
		var scan func(ast.Node)
		scan = func(node ast.Node) {
			ast.Inspect(node, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name := ""
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					name = fun.Name
				case *ast.SelectorExpr:
					name = fun.Sel.Name
					// Inspect the receiver tree too: http.DefaultClient.Do and
					// (&http.Client{}).Do have nested selectors, not an Ident receiver.
					ast.Inspect(fun.X, func(x ast.Node) bool {
						id, ok := x.(*ast.Ident)
						if !ok {
							return true
						}
						switch id.Name {
						case "http", "os", "exec", "sql", "net":
							effects[render(fun)] = true
						}
						return true
					})
					// Package calls are not same-file method calls.
					if id, ok := fun.X.(*ast.Ident); ok && packages[id.Name] {
						name = ""
					}
				}
				if name != "" && !visited[name] {
					visited[name] = true
					for _, body := range funcs[name] {
						scan(body)
					}
				}
				return true
			})
		}
		for _, body := range clause.Body {
			scan(body)
		}
		for _, expr := range clause.List {
			sel, ok := expr.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name != "bytecode" {
				continue
			}
			opName := sel.Sel.Name
			var op bytecode.Opcode
			found := false
			// The Go identifier and registry name need not use the same casing.
			for candidate, spec := range bytecode.Registry {
				if strings.EqualFold(strings.ReplaceAll(spec.Name, "_", ""), strings.TrimPrefix(opName, "Op")) {
					op = candidate
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("dispatch opcode %s missing from Registry", opName)
			}
			if len(effects) == 0 {
				continue
			}
			if _, ok := allowed[opName]; ok {
				seen[opName] = true
				gate := false
				for _, body := range clause.Body {
					ast.Inspect(body, func(n ast.Node) bool {
						if call, ok := n.(*ast.CallExpr); ok && render(call.Fun) == "vm.requireCapability" && len(call.Args) == 2 && render(call.Args[0]) == "capability.Network" && render(call.Args[1]) == "inst.Op" {
							gate = true
						}
						return true
					})
				}
				if !gate {
					t.Errorf("%s: missing in-case network gate", opName)
				}
			} else if bytecode.Registry[op].Capability == capability.None {
				t.Errorf("%s reaches %v without a registry capability", opName, effects)
			}
		}
	}
	for name := range allowed {
		if !seen[name] {
			t.Errorf("stale effect allow-list entry %s", name)
		}
	}
}

func TestEffectGateEmptyGrantConformance(t *testing.T) {
	probe := startModelHostProbe(t)
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer server.Close()
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	quote := strconv.Quote
	t.Setenv("HOWL_EFFECT_SECRET", "must-not-print")
	sources := map[bytecode.Opcode]string{
		bytecode.OpFetch:            `(fetch ` + quote(server.URL) + ` "GET")`,
		bytecode.OpReadFile:         `(read_file ` + quote(marker) + `)`,
		bytecode.OpWriteFile:        `(write_file ` + quote(marker) + ` "changed")`,
		bytecode.OpMkdir:            `(mkdir ` + quote(marker) + `)`,
		bytecode.OpExec:             `(exec "touch" ` + quote(marker) + `)`,
		bytecode.OpSpawn:            `(spawn (fn () (exec "touch" ` + quote(marker) + `)))`,
		bytecode.OpSpawnAgent:       `(spawn_agent "worker" (task "write" (write_file ` + quote(marker) + ` "changed")))`,
		bytecode.OpEnv:              `(print (env "HOWL_EFFECT_SECRET"))`,
		bytecode.OpConfidence:       `(confidence "test")`,
		bytecode.OpNeuralCircuit:    `(neural_circuit ("x") "test")`,
		bytecode.OpEphemeralCircuit: `(ephemeral_circuit ("x") "test")`,
		bytecode.OpLlmGenerate:      `(llm_generate "test" "test")`,
		bytecode.OpAchieve:          `(achieve "test" "test")`,
		bytecode.OpStoreOpen:        `(store_open kv "memory://conformance")`,
	}
	// Context-dependent HTTP operations and store/SQL operations whose setup
	// already requires authority are deliberately bare. Even malformed operands
	// and an empty stack must be denied before the case performs any work.
	bare := []bytecode.Opcode{
		bytecode.OpDbConnect, bytecode.OpSqlQuery,
		bytecode.OpRes, bytecode.OpResJson, bytecode.OpHttpServerStart,
		bytecode.OpHttpRoute, bytecode.OpHttpServerServe, bytecode.OpHttpReqMethod,
		bytecode.OpHttpResHeader, bytecode.OpStorePut, bytecode.OpStoreGet,
		bytecode.OpStoreDelete, bytecode.OpStoreKeys,
	}
	programs := map[bytecode.Opcode]*bytecode.BCProgram{}
	for op, source := range sources {
		_, program := parseAndCompile(t, `(cli_app `+source+`)`)
		present := false
		for _, inst := range program.Main {
			if inst.Op == op {
				present = true
			}
		}
		if !present {
			t.Fatalf("source does not exercise %s", bytecode.Registry[op].Name)
		}
		programs[op] = program
	}
	for _, op := range bare {
		programs[op] = &bytecode.BCProgram{Main: []bytecode.BCInstruction{{Op: op}}}
	}
	_, lazy := parseAndCompile(t, `(cli_app (lazy_synthesize f () "test") (call f))`)
	programs[bytecode.OpCall] = lazy
	for op, spec := range bytecode.Registry {
		if spec.Capability != capability.None && programs[op] == nil {
			t.Errorf("%s gained a capability without a dynamic case", spec.Name)
		}
	}
	for op, program := range programs {
		t.Run(bytecode.Registry[op].Name, func(t *testing.T) {
			cap := bytecode.Registry[op].Capability
			if op == bytecode.OpCall {
				cap = capability.Network
			}
			if cap == capability.None {
				t.Fatal("stale dynamic capability case")
			}
			outcome := runBytecodeOutcome(program, "", nil)
			if outcome.vmError == nil || outcome.vmError.Code != "CAPABILITY_DENIED" || !strings.Contains(outcome.vmError.Message, string(cap)) || outcome.vmError.Opcode != bytecode.Registry[op].Name {
				t.Fatalf("expected %s denial at %s: %+v", cap, bytecode.Registry[op].Name, outcome)
			}
			if outcome.stdout != "" {
				t.Fatalf("denied run printed %q", outcome.stdout)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("denied effect created marker: %v", err)
			}
			if hits.Load() != 0 {
				t.Fatalf("denied fetch sent %d requests", hits.Load())
			}
			probe.assertNoRequests(t, "bytecode")
		})
	}
	// Interpreter constructs with an existing executable harness. Bytecode-only
	// SQL, HTTP context and native stores are covered above, not interpreted.
	for _, op := range []bytecode.Opcode{bytecode.OpFetch, bytecode.OpReadFile, bytecode.OpWriteFile, bytecode.OpMkdir, bytecode.OpExec, bytecode.OpSpawn, bytecode.OpSpawnAgent, bytecode.OpEnv, bytecode.OpConfidence, bytecode.OpNeuralCircuit, bytecode.OpEphemeralCircuit, bytecode.OpLlmGenerate, bytecode.OpAchieve, bytecode.OpCall} {
		t.Run("interpret/"+bytecode.Registry[op].Name, func(t *testing.T) {
			source := sources[op]
			cap := bytecode.Registry[op].Capability
			if op == bytecode.OpCall {
				source = `(lazy_synthesize f () "test") (call f)`
				cap = capability.Network
			}
			node, _ := parseAndCompile(t, `(cli_app `+source+`)`)
			var out, stderr bytes.Buffer
			code := Interpret(node, nil, nil, strings.NewReader(""), &out, &stderr)
			if code == 0 || !strings.Contains(stderr.String(), "capability denied: "+string(cap)) || out.Len() != 0 {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, out.String(), stderr.String())
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("interpreter created marker: %v", err)
			}
			if hits.Load() != 0 {
				t.Fatal("interpreter sent HTTP request")
			}
			probe.assertNoRequests(t, "interpreter")
		})
	}
}
