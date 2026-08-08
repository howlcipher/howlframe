package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"zero/internal/ast"
	"zero/internal/backend/gogen"
	"zero/internal/backend/javascript"
	"zero/internal/backend/wasm"
	"zero/internal/bytecode"
	"zero/internal/capability"
	"zero/internal/checker"
	"zero/internal/ir"
	"zero/internal/lexer"
	"zero/internal/masking"
	"zero/internal/optimization"
	"zero/internal/parser"
	"zero/internal/vm"
	"zero/internal/zir"
)

func init() {
	gob.Register(float64(0))
	gob.Register("")
}

func main() {
	outDir := flag.String("o", "", "output directory, or exact artifact file after input with -compile-bc or -compile-wasm")
	runMode := flag.Bool("run", false, "interpret and execute a cli_app script directly (Phase 1 of improvement #49: no Go/JS text generated, no go build/go run invoked)")
	compileBc := flag.Bool("compile-bc", false, "compile AST to bytecode JSON")
	compileWasm := flag.Bool("compile-wasm", false, "compile typed SSA/CFG to WebAssembly Text")
	runBc := flag.Bool("run-bc", false, "run bytecode from JSON file")
	allowCaps := flag.String("allow-caps", "", "comma-separated capabilities to allow when running bytecode with -run-bc (network,filesystem,process,environment,database); instructions requiring an unlisted capability are denied")
	validateMode := flag.Bool("validate", false, "run lexer, parser, and semantic checker without transpiling")
	maskPlan := flag.Bool("mask-plan", false, "print the deterministic constrained-decoding mask plan and exit")
	optimizationPlan := flag.Bool("optimization-plan", false, "print the deterministic compile-time optimization plan and exit")
	flag.Parse()

	if flag.NArg() < 1 {
		ast.ReportError("Missing file argument", 0, 0)
	}
	inputFile := flag.Arg(0)
	outputDir := outputDirectory(os.Args[1:], inputFile, *outDir)

	content, err := os.ReadFile(inputFile)
	if err != nil {
		ast.ReportError(fmt.Sprintf("Cannot read file: %v", err), 0, 0)
	}

	if *runBc {
		var prog bytecode.BCProgram
		buf := bytes.NewReader(content)
		dec := gob.NewDecoder(buf)
		if err := dec.Decode(&prog); err != nil {
			ast.ReportError(fmt.Sprintf("Cannot parse bytecode: %v", err), 0, 0)
		}
		os.Exit(vm.RunBytecode(&prog, flag.Args()[1:], parseAllowedCaps(*allowCaps), os.Stdin, os.Stdout, os.Stderr))
	}

	lx := lexer.NewLexer(string(content))
	p := parser.NewParser(lx, filepath.Base(inputFile))
	root := p.ParseExpression()

	if p.Cur.Type != lexer.TokenEOF {
		ast.ReportError("Unexpected tokens after EOF", p.Cur.Line, p.Cur.Column)
	}

	parser.ExpandIncludes(root, filepath.Dir(inputFile), 0)
	ast.ResolveModules(root)

	ast.ApplyPatches(root)
	root = ast.ApplyWithContext(root, nil)
	root = ast.ApplyWithContext(root, nil)
	analysis := checker.Check(root)
	zirModule := filepath.Base(inputFile)

	if *maskPlan {
		plan, err := json.Marshal(masking.CompileAnalysis(analysis))
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to encode mask plan: %v", err), 0, 0)
		}
		fmt.Println(string(plan))
		return
	}

	if *optimizationPlan {
		plan, err := json.Marshal(optimization.CompileAnalysis(analysis))
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to encode optimization plan: %v", err), 0, 0)
		}
		fmt.Println(string(plan))
		return
	}
	if *validateMode {
		runZirGate(root, zirModule, zirTargetNone)
		return
	}

	if *compileBc {
		runZirGate(root, zirModule, zirTargetBytecode)
		prog := bytecode.CompileToBytecode(root)
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(prog); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to encode bytecode: %v", err), 0, 0)
		}
		outFile := inputFile + ".bc.bin"
		if outputFile := outputFlagAfterInput(os.Args[1:], inputFile); outputFile != "" {
			outFile = outputFile
		} else if *outDir != "" {
			outFile = filepath.Join(*outDir, filepath.Base(inputFile)+".bc.bin")
		}
		if err = writeArtifact(outFile, buf.Bytes()); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", outFile, err), 0, 0)
		}
		os.Exit(0)
	}

	if *compileWasm {
		runZirGate(root, zirModule, zirTargetWasm)
		functionSources, expression, err := ssaWasmProgram(root, analysis)
		if err != nil {
			line, column := 0, 0
			if root != nil {
				line, column = root.Line, root.Column
			}
			ast.ReportError(err.Error(), line, column)
		}
		var wasmFunctions []wasm.Function
		for _, source := range functionSources {
			functionGraph, err := ir.LowerSSAFunction(source.params, source.body)
			if err != nil {
				ast.ReportError(fmt.Sprintf("Failed to lower SSA graph for function %q: %v", source.name, err), source.body.Line, source.body.Column)
			}
			wasmFunctions = append(wasmFunctions, wasm.Function{
				Name:   source.name,
				Params: source.params,
				Return: source.ret,
				Graph:  functionGraph,
			})
		}
		graph, err := ir.LowerSSA(expression)
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to lower SSA graph: %v", err), expression.Line, expression.Column)
		}
		wasmCode, err := wasm.SerializeSSAProgram(wasmFunctions, graph)
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to serialize SSA graph: %v", err), expression.Line, expression.Column)
		}
		outFile := inputFile + ".ssa.wat"
		if outputFile := outputFlagAfterInput(os.Args[1:], inputFile); outputFile != "" {
			outFile = outputFile
		} else if *outDir != "" {
			outFile = filepath.Join(*outDir, filepath.Base(inputFile)+".ssa.wat")
		}
		if err = writeArtifact(outFile, []byte(wasmCode)); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", outFile, err), 0, 0)
		}
		os.Exit(0)
	}

	if *runMode {
		runZirGate(root, zirModule, zirTargetInterpreter)
		os.Exit(vm.Interpret(root, flag.Args()[1:], os.Stdin, os.Stdout, os.Stderr))
	}

	if root != nil && root.Type == "List" && len(root.Children) > 0 && root.Children[0].Type == "SYMBOL" && root.Children[0].Value == "wasm_app" {
		runZirGate(root, zirModule, zirTargetWasm)
		wasmCode := wasm.GenerateWasmCode(root)
		wasmFile := filepath.Join(outputDir, "app.wat")
		if err = writeArtifact(wasmFile, []byte(wasmCode)); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", wasmFile, err), 0, 0)
		}
	} else if root != nil && root.Type == "List" && len(root.Children) > 0 && root.Children[0].Type == "SYMBOL" && root.Children[0].Value == "web_app" {
		runZirGate(root, zirModule, zirTargetJavaScript)
		jsCode, testCode := javascript.GenerateJSCode(root)

		appFile := filepath.Join(outputDir, "app.js")
		appTestFile := filepath.Join(outputDir, "app.test.js")

		err = writeArtifact(appFile, []byte(jsCode))
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", appFile, err), 0, 0)
		}
		if testCode != "" {
			err = writeArtifact(appTestFile, []byte(testCode))
			if err != nil {
				ast.ReportError(fmt.Sprintf("Failed to write %s: %v", appTestFile, err), 0, 0)
			}
		} else {
			os.Remove(appTestFile)
		}
	} else {
		runZirGate(root, zirModule, zirTargetGo)
		goCode, testCode := gogen.GenerateCode(root)

		serverFile := filepath.Join(outputDir, "server.go")
		serverTestFile := filepath.Join(outputDir, "server_test.go")

		err = writeArtifact(serverFile, []byte(goCode))
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", serverFile, err), 0, 0)
		}

		if testCode != "" {
			err = writeArtifact(serverTestFile, []byte(testCode))
			if err != nil {
				ast.ReportError(fmt.Sprintf("Failed to write %s: %v", serverTestFile, err), 0, 0)
			}
		} else {
			os.Remove(serverTestFile)
		}
	}
}

// wasmFunctionSource is one defun collected from a cli_app root, ready to be
// lowered into its own ir.Graph via ir.LowerSSAFunction.
type wasmFunctionSource struct {
	name   string
	params []ir.Param
	ret    ast.TypeInfo
	body   *ast.Node
}

// ssaWasmProgram splits a cli_app root into its top-level defun declarations
// (if any) and its final entry expression for -compile-wasm. Each defun's
// parameter/return types come from the checker's already-computed signature
// (analysis.Functions), not re-derived here.
func ssaWasmProgram(root *ast.Node, analysis *checker.Analysis) ([]wasmFunctionSource, *ast.Node, error) {
	if root == nil || root.Type != "List" || len(root.Children) == 0 ||
		root.Children[0].Type != "SYMBOL" || root.Children[0].Value != "cli_app" {
		return nil, nil, fmt.Errorf("-compile-wasm requires a cli_app root")
	}
	if len(root.Children) < 2 {
		return nil, nil, fmt.Errorf("-compile-wasm requires cli_app to contain at least one expression")
	}

	var functions []wasmFunctionSource
	for _, child := range root.Children[1 : len(root.Children)-1] {
		head := ""
		if child.Type == "List" && len(child.Children) > 0 && child.Children[0].Type == "SYMBOL" {
			head = child.Children[0].Value
		}
		if head != "defun" {
			if head == "" {
				head = child.Type
			}
			return nil, nil, fmt.Errorf("-compile-wasm only supports defun declarations before the final entry expression, found %q", head)
		}
		if len(child.Children) < 4 {
			return nil, nil, fmt.Errorf("-compile-wasm: defun is malformed, expected (defun name (args) body)")
		}
		name := child.Children[1].Value
		info, ok := analysis.Functions[name]
		if !ok {
			return nil, nil, fmt.Errorf("-compile-wasm: no checker signature found for function %q", name)
		}
		argsNode := child.Children[2]
		params := make([]ir.Param, len(argsNode.Children))
		for index, arg := range argsNode.Children {
			paramName := arg.Value
			if arg.Type == "List" && len(arg.Children) > 0 {
				paramName = arg.Children[0].Value
			}
			params[index] = ir.Param{Name: paramName, Type: info.Params[index]}
		}
		body := child.Children[len(child.Children)-1]
		functions = append(functions, wasmFunctionSource{name: name, params: params, ret: info.Return, body: body})
	}

	entry := root.Children[len(root.Children)-1]
	if entry.Type == "List" && len(entry.Children) > 0 && entry.Children[0].Type == "SYMBOL" && entry.Children[0].Value == "defun" {
		return nil, nil, fmt.Errorf("-compile-wasm requires cli_app's final child to be an entry expression, not a defun")
	}
	return functions, entry, nil
}

func outputFlagAfterInput(args []string, inputFile string) string {
	inputSeen := false
	for index, arg := range args {
		if arg == inputFile {
			inputSeen = true
			continue
		}
		if !inputSeen {
			continue
		}
		if arg == "-o" && index+1 < len(args) {
			return args[index+1]
		}
		if strings.HasPrefix(arg, "-o=") {
			return strings.TrimPrefix(arg, "-o=")
		}
	}
	return ""
}

// outputDirectory accepts -o before or after the positional input file for
// backends whose -o contract names a directory rather than an exact artifact.
func outputDirectory(args []string, inputFile, configured string) string {
	if output := outputFlagAfterInput(args, inputFile); output != "" {
		return output
	}
	return configured
}

// zirTarget identifies which internal/zir verifier target identity a given
// source-based compiler path is checked against. Only "wasm" has real
// feasibility rules in internal/zir/verifier.go's isFeasible today; every
// other identity below is accepted permissively by that function until
// per-target rules exist for it (see improvements.md #87 Phase 2 notes).
// Declaring these once here, instead of scattering literal strings at each
// dispatch call site, keeps the CLI-mode -> target-identity mapping in one
// place and gives runZirGate's callers a single source of truth for the
// spelling.
type zirTarget string

const (
	zirTargetNone        zirTarget = ""            // -validate: target-independent
	zirTargetBytecode    zirTarget = "bytecode"    // -compile-bc
	zirTargetWasm        zirTarget = "wasm"        // -compile-wasm and legacy wasm_app
	zirTargetInterpreter zirTarget = "interpreter" // -run
	zirTargetJavaScript  zirTarget = "javascript"  // web_app
	zirTargetGo          zirTarget = "go"          // default cli_app/http_server backend
)

// zirBlockingCodes are the ZIR diagnostic codes currently trusted enough to
// fail a production build closed. ZIR_UNBOUND_REF is deliberately excluded:
// verifying against the real tests/*.zero corpus (see
// TestZirGateAcceptsAllExistingFixtures in zero_test.go) showed it produces
// false positives on any variable bound via try_let/catch or bound to the
// result of a dynamically-typed primitive (llm_generate, generics, HTTP
// lambda parameter field access, etc.) - internal/checker's Analysis.infer
// (internal/checker/types.go) legitimately types these as ast.Unknown
// without raising a checker diagnostic (this is intentional dynamic-typing
// support, not a bug), but the checked AST's TypeInfo does not preserve the
// difference between "resolved, dynamically typed" and "never resolved" by
// the time ZIR lowers it - and a genuinely unresolved reference already
// fails checker.Check before ZIR ever runs, so on real programs
// ZIR_UNBOUND_REF currently has no reachable true positive, only false
// positives. Tracked as bugs.md #42. Verify() still computes and returns
// this diagnostic (internal/zir/verifier.go is unchanged) - this exclusion
// lives only in the production gate's failure policy, not in the verifier
// itself, so nothing here silently skips verification.
var zirBlockingCodes = map[string]bool{
	"ZIR_INVALID_REF":       true,
	"ZIR_TARGET_INFEASIBLE": true,
}

// runZirGate lowers root to a ZIR graph and verifies it against target
// (zirTargetNone for target-independent validation, used only by
// -validate). Any lowering error, or any diagnostic whose code is in
// zirBlockingCodes, is reported as one deterministic JSON array via
// reportZirDiagnostics before any writeArtifact/backend call runs, so the
// process exits before producing partial output - the same fail-closed
// contract checker.Check already enforces via ast.ReportError. This
// intentionally covers only the subset of verification internal/zir
// currently implements safely for production (dangling data/control-edge
// reference and wasm-target feasibility; capability-effect inference always
// runs as a Verify() side effect but never itself produces a blocking
// diagnostic) - real control-flow/cycle verification and non-wasm target
// feasibility are not implemented in internal/zir yet (ControlEdges is
// declared but never populated by LowerAST, and isFeasible only has rules
// for "wasm").
func runZirGate(root *ast.Node, module string, target zirTarget) {
	graph, err := zir.LowerAST(root, module)
	if err != nil {
		line, column := 0, 0
		if root != nil {
			line, column = root.Line, root.Column
		}
		ast.ReportError(fmt.Sprintf("ZIR lowering failed: %v", err), line, column)
	}

	diags := zir.NewVerifier(graph, string(target)).Verify()
	var errDiags []zir.Diagnostic
	for _, d := range diags {
		if d.Severity == zir.SeverityError && zirBlockingCodes[d.Code] {
			errDiags = append(errDiags, d)
		}
	}
	if len(errDiags) > 0 {
		reportZirDiagnostics(errDiags)
	}
}

// reportZirDiagnostics prints every ERROR diagnostic as one deterministic
// JSON array line (preserving Verify()'s node-order-derived ordering) and
// exits nonzero, mirroring ast.ReportError's one-line JSON contract without
// truncating to a single diagnostic - internal/zir/verifier.go's Diagnostics
// is a slice for a reason, and there is no existing consumer of ZIR
// diagnostics to stay wire-compatible with yet.
func reportZirDiagnostics(diags []zir.Diagnostic) {
	b, _ := json.Marshal(diags)
	fmt.Println(string(b))
	os.Exit(1)
}

var knownCapabilities = map[capability.Capability]bool{
	capability.Network:     true,
	capability.Filesystem:  true,
	capability.Process:     true,
	capability.Environment: true,
	capability.Database:    true,
}

// parseAllowedCaps turns -allow-caps into a capability allow-list. An empty
// or unset flag denies every capability-gated instruction (fail-closed
// default); RunBytecode always permits CapNone regardless of this list.
func parseAllowedCaps(raw string) []capability.Capability {
	if raw == "" {
		return nil
	}
	var caps []capability.Capability
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		cap := capability.Capability(part)
		if !knownCapabilities[cap] {
			ast.ReportError(fmt.Sprintf("unknown capability in -allow-caps: %q", part), 0, 0)
		}
		caps = append(caps, cap)
	}
	return caps
}

// writeArtifact creates the destination directory before writing an emitted
// artifact, so every backend follows the same output contract.
func writeArtifact(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
