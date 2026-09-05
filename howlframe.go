package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/backend/gogen"
	"github.com/howlcipher/howlframe/internal/backend/javascript"
	"github.com/howlcipher/howlframe/internal/backend/wasm"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/hfir"
	"github.com/howlcipher/howlframe/internal/ir"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/masking"
	"github.com/howlcipher/howlframe/internal/optimization"
	"github.com/howlcipher/howlframe/internal/parser"
	"github.com/howlcipher/howlframe/internal/vm"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const Version = "0.1.0"
const HFBCFormatVersion = "1"

func init() {
	gob.Register(float64(0))
	gob.Register("")
}

func main() {
	if len(os.Args) > 1 {
		cmd := os.Args[1]
		if !strings.HasPrefix(cmd, "-") && (cmd == "check" || cmd == "build" || cmd == "run" || cmd == "version" || cmd == "help") {
			runSubcommand()
			return
		}
		if cmd == "--version" {
			fmt.Printf("HowlFrame %s\nHFBC format: %s\n", Version, HFBCFormatVersion)
			return
		}
	}

	outDir := flag.String("o", "", "output directory, or exact artifact file after input with -compile-bc or -compile-wasm")
	runMode := flag.Bool("run", false, "interpret and execute a cli_app script directly (Phase 1 of improvement #49: no Go/JS text generated, no go build/go run invoked)")
	compileBc := flag.Bool("compile-bc", false, "compile AST to bytecode JSON")
	compileHfirBc := flag.Bool("compile-hfir-bc", false, "EXPERIMENTAL: compile directly from semantic HFIR to bytecode JSON")
	compileWasm := flag.Bool("compile-wasm", false, "compile typed SSA/CFG to WebAssembly Text")
	runBc := flag.Bool("run-bc", false, "run bytecode from JSON file")
	allowCaps := flag.String("allow-caps", "", "comma-separated capabilities to allow when running bytecode with -run-bc (network,filesystem,process,environment,database); instructions requiring an unlisted capability are denied")
	maxInstructions := flag.Int("max-instructions", vm.DefaultLimits.MaxInstructions, "positive finite instruction ceiling for -run-bc (default 100000; zero and negative values are invalid)")
	validateMode := flag.Bool("validate", false, "run lexer, parser, and semantic checker without transpiling")
	maskPlan := flag.Bool("mask-plan", false, "print the deterministic constrained-decoding mask plan and exit")
	optimizationPlan := flag.Bool("optimization-plan", false, "print the deterministic compile-time optimization plan and exit")
	flag.Parse()
	if *runBc && *maxInstructions <= 0 {
		ast.ReportError("-max-instructions must be greater than zero; unlimited execution is not supported", 0, 0)
	}
	if *outDir != "" && strings.HasPrefix(*outDir, "-") {
		ast.ReportError("flag needs an argument: -o", 0, 0)
	}
	if *allowCaps != "" && strings.HasPrefix(*allowCaps, "-") {
		ast.ReportError("flag needs an argument: -allow-caps", 0, 0)
	}

	if flag.NArg() < 1 {
		ast.ReportError("Missing file argument", 0, 0)
	}
	inputFile := flag.Arg(0)
	trailing := flag.Args()[1:]

	// Everything after the input file belongs to the target program under the
	// argv-owning modes and to this CLI under every other mode (see
	// argvOwningModes). *runBc/*runMode is the same condition the dispatch
	// below uses, so the tokens handed to vm.RunBytecodeWithPolicy and
	// vm.Interpret are exactly flag.Args()[1:], unchanged.
	var programArgs []string
	var setAfterInput map[string]bool
	if *runBc || *runMode {
		programArgs = trailing
	} else {
		afterInput, leftover, err := resumeFlagsAfterInput(flag.CommandLine, trailing)
		if errors.Is(err, flag.ErrHelp) {
			// -h and -help are answered by the first parse with usage and a
			// zero exit; the resumed parse must answer them the same way
			// rather than reporting a diagnostic, or -h would be the one flag
			// still position-dependent.
			flag.Usage()
			os.Exit(0)
		}
		if err != nil {
			ast.ReportError(fmt.Sprintf("Cannot parse arguments after input file %q: %v", inputFile, err), 0, 0)
		}
		setAfterInput = afterInput
		if *outDir != "" && strings.HasPrefix(*outDir, "-") {
			ast.ReportError(fmt.Sprintf("Cannot parse arguments after input file %q: flag needs an argument: -o", inputFile), 0, 0)
		}
		if *allowCaps != "" && strings.HasPrefix(*allowCaps, "-") {
			ast.ReportError(fmt.Sprintf("Cannot parse arguments after input file %q: flag needs an argument: -allow-caps", inputFile), 0, 0)
		}
		for _, mode := range argvOwningModes {
			if setAfterInput[mode] {
				ast.ReportError(fmt.Sprintf("-%s must be given before the input file, because every token after the input file is passed to the program being run as its arguments", mode), 0, 0)
			}
		}
		if len(leftover) > 0 {
			ast.ReportError(fmt.Sprintf("Unexpected argument %q after input file %q", leftover[0], inputFile), 0, 0)
		}
	}
	outputDir := *outDir

	content, err := os.ReadFile(inputFile)
	if err != nil {
		ast.ReportError(fmt.Sprintf("Cannot read file: %v", err), 0, 0)
	}

	if *runBc {
		prog, err := bytecode.ReadArtifact(bytes.NewReader(content))
		if err != nil {
			ast.ReportError(fmt.Sprintf("Cannot parse bytecode: %v", err), 0, 0)
		}
		executionPolicy := vm.DefaultExecutionPolicy()
		executionPolicy.Limits.MaxInstructions = *maxInstructions
		os.Exit(vm.RunBytecodeWithPolicy(prog, programArgs, executionPolicy, parseAllowedCaps(*allowCaps), os.Stdin, os.Stdout, os.Stderr))
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
	hfirModule := filepath.Base(inputFile)

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
		runHFIRGate(root, hfirModule, hfirTargetNone)
		return
	}

	if *compileBc {
		runHFIRGate(root, hfirModule, hfirTargetBytecode)
		prog := bytecode.CompileToBytecode(root)
		var buf bytes.Buffer
		if err := bytecode.WriteArtifact(&buf, prog); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to encode bytecode: %v", err), 0, 0)
		}
		outFile := artifactOutputPath(inputFile, *outDir, setAfterInput["o"], ".bc.bin")
		if err = writeArtifact(outFile, buf.Bytes()); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", outFile, err), 0, 0)
		}
		os.Exit(0)
	}

	if *compileHfirBc {
		graph := runHFIRGate(root, hfirModule, hfirTargetBytecode)
		prog, diags := hfir.LowerToBytecode(graph)
		if len(diags) > 0 {
			ast.ReportError(fmt.Sprintf("HFIR direct lowering failed: %s", diags[0].Message), diags[0].Location.Line, diags[0].Location.Column)
		}
		var buf bytes.Buffer
		if err := bytecode.WriteArtifact(&buf, prog); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to encode bytecode: %v", err), 0, 0)
		}
		outFile := artifactOutputPath(inputFile, *outDir, setAfterInput["o"], ".bc.bin")
		if err = writeArtifact(outFile, buf.Bytes()); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", outFile, err), 0, 0)
		}
		os.Exit(0)
	}

	if *compileWasm {
		runHFIRGate(root, hfirModule, hfirTargetWasm)
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
		outFile := artifactOutputPath(inputFile, *outDir, setAfterInput["o"], ".ssa.wat")
		if err = writeArtifact(outFile, []byte(wasmCode)); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", outFile, err), 0, 0)
		}
		os.Exit(0)
	}

	if *runMode {
		runHFIRGate(root, hfirModule, hfirTargetInterpreter)
		os.Exit(vm.Interpret(root, programArgs, os.Stdin, os.Stdout, os.Stderr))
	}

	if root != nil && root.Type == "List" && len(root.Children) > 0 && root.Children[0].Type == "SYMBOL" && root.Children[0].Value == "wasm_app" {
		runHFIRGate(root, hfirModule, hfirTargetWasm)
		wasmCode := wasm.GenerateWasmCode(root)
		wasmFile := filepath.Join(outputDir, "app.wat")
		if err = writeArtifact(wasmFile, []byte(wasmCode)); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", wasmFile, err), 0, 0)
		}
	} else if root != nil && root.Type == "List" && len(root.Children) > 0 && root.Children[0].Type == "SYMBOL" && root.Children[0].Value == "web_app" {
		runHFIRGate(root, hfirModule, hfirTargetJavaScript)
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
		runHFIRGate(root, hfirModule, hfirTargetGo)
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

// argvOwningModes names the mode flags whose contract hands every token after
// the positional input file to the target program instead of to this CLI: main
// passes them verbatim to vm.RunBytecodeWithPolicy (-run-bc) and vm.Interpret
// (-run) as the program's argv, so `-run-bc app.bc -allow-caps environment`
// must deliver ["-allow-caps", "environment"] to the program rather than
// setting this CLI's -allow-caps. Those modes therefore opt out of
// resumeFlagsAfterInput entirely and must be written before the input file;
// main rejects them after it rather than silently reinterpreting the program's
// arguments (bugs.md #52).
var argvOwningModes = []string{"run", "run-bc"}

// resumeFlagsAfterInput continues parsing fs's flags over the tokens that
// follow the positional input file. Go's flag package stops at the first
// non-flag argument, so without this every flag written after the filename is
// dropped and - because no mode flag was ever set - the CLI silently falls
// through to its default Go backend (bugs.md #52).
//
// The resumed parse is a second FlagSet that re-registers every flag fs
// already declares, sharing the identical flag.Value. It therefore writes into
// the same variables the first parse did and accepts the same syntax for them
// (including bare booleans and -name=value), while knowing nothing about any
// individual flag. That is what replaces bug #39's outputFlagAfterInput, which
// re-scanned raw argv for the "-o" and "-o=" spellings alone and so left every
// other flag broken after the input.
//
// It returns the names of the flags this resumed parse set - callers whose
// flag has a position-dependent meaning, such as -o, need that distinction -
// along with any tokens the resumed parse could not consume, which the caller
// must reject rather than ignore. A trailing -h or -help returns flag.ErrHelp,
// which callers answer with usage and a zero exit exactly as their first parse
// does, so help is not the one flag left position-dependent.
func resumeFlagsAfterInput(fs *flag.FlagSet, trailing []string) (map[string]bool, []string, error) {
	resumed := flag.NewFlagSet(fs.Name(), flag.ContinueOnError)
	resumed.SetOutput(io.Discard)
	fs.VisitAll(func(f *flag.Flag) { resumed.Var(f.Value, f.Name, f.Usage) })
	if err := resumed.Parse(trailing); err != nil {
		return nil, nil, err
	}
	setAfterInput := make(map[string]bool)
	resumed.Visit(func(f *flag.Flag) { setAfterInput[f.Name] = true })
	return setAfterInput, resumed.Args(), nil
}

// artifactOutputPath resolves where -compile-bc, -compile-hfir-bc and
// -compile-wasm write their emitted artifact. Their -o names an exact file
// when it is written after the input file and an output directory when it is
// written before it, the contract bug #39 established; outputAfterInput says
// which position the user used.
func artifactOutputPath(inputFile, output string, outputAfterInput bool, suffix string) string {
	switch {
	case output == "":
		return inputFile + suffix
	case outputAfterInput:
		return output
	default:
		return filepath.Join(output, filepath.Base(inputFile)+suffix)
	}
}

// hfirTarget identifies which internal/hfir verifier target identity a given
// source-based compiler path is checked against. Only "wasm" has real
// feasibility rules in internal/hfir/verifier.go's isFeasible today; every
// other identity below is accepted permissively by that function until
// per-target rules exist for it (see improvements.md #87 Phase 2 notes).
// Declaring these once here, instead of scattering literal strings at each
// dispatch call site, keeps the CLI-mode -> target-identity mapping in one
// place and gives runHFIRGate's callers a single source of truth for the
// spelling.
type hfirTarget string

const (
	hfirTargetNone        hfirTarget = ""            // -validate: target-independent
	hfirTargetBytecode    hfirTarget = "bytecode"    // -compile-bc
	hfirTargetWasm        hfirTarget = "wasm"        // -compile-wasm and legacy wasm_app
	hfirTargetInterpreter hfirTarget = "interpreter" // -run
	hfirTargetJavaScript  hfirTarget = "javascript"  // web_app
	hfirTargetGo          hfirTarget = "go"          // default cli_app/http_server backend
)

// hfirBlockingCodes are the HFIR diagnostic codes currently trusted enough to
// fail a production build closed. HFIR_UNBOUND_REF is deliberately excluded:
// verifying against the real tests/*.howl corpus (see
// TestHFIRGateAcceptsAllExistingFixtures in howlframe_test.go) showed it produces
// false positives on any variable bound via try_let/catch or bound to the
// result of a dynamically-typed primitive (llm_generate, generics, HTTP
// lambda parameter field access, etc.) - internal/checker's Analysis.infer
// (internal/checker/types.go) legitimately types these as ast.Unknown
// without raising a checker diagnostic (this is intentional dynamic-typing
// support, not a bug), but the checked AST's TypeInfo does not preserve the
// difference between "resolved, dynamically typed" and "never resolved" by
// the time HFIR lowers it - and a genuinely unresolved reference already
// fails checker.Check before HFIR ever runs, so on real programs
// HFIR_UNBOUND_REF currently has no reachable true positive, only false
// positives. Tracked as bugs.md #42. Verify() still computes and returns
// this diagnostic (internal/hfir/verifier.go is unchanged) - this exclusion
// lives only in the production gate's failure policy, not in the verifier
// itself, so nothing here silently skips verification.
var hfirBlockingCodes = map[string]bool{
	"HFIR_INVALID_REF":       true,
	"HFIR_TARGET_INFEASIBLE": true,
}

// runHFIRGate lowers root to a HFIR graph and verifies it against target
// (hfirTargetNone for target-independent validation, used only by
// -validate). Any lowering error, or any diagnostic whose code is in
// hfirBlockingCodes, is reported as one deterministic JSON array via
// reportHFIRDiagnostics before any writeArtifact/backend call runs, so the
// process exits before producing partial output - the same fail-closed
// contract checker.Check already enforces via ast.ReportError. This
// intentionally covers only the subset of verification internal/hfir
// currently implements safely for production (dangling data/control-edge
// reference and wasm-target feasibility; capability-effect inference always
// runs as a Verify() side effect but never itself produces a blocking
// diagnostic) - real control-flow/cycle verification and non-wasm target
// feasibility are not implemented in internal/hfir yet (ControlEdges is
// declared but never populated by LowerAST, and isFeasible only has rules
// for "wasm").
func runHFIRGate(root *ast.Node, module string, target hfirTarget) *hfir.Graph {
	graph, err := hfir.LowerAST(root, module)
	if err != nil {
		line, column := 0, 0
		if root != nil {
			line, column = root.Line, root.Column
		}
		ast.ReportError(fmt.Sprintf("HFIR lowering failed: %v", err), line, column)
	}

	diags := hfir.NewVerifier(graph, string(target)).Verify()

	// Construct-support verification runs over the AST rather than the
	// lowered graph, because LowerAST names a node's Kind after the head of
	// every list - including let bindings and sub-forms their parent
	// destructures itself - so a rule matching on Kind would reject correct
	// programs (bugs.md #45). Its diagnostics use HFIR_TARGET_INFEASIBLE,
	// already in hfirBlockingCodes, so the gate below fails closed before
	// bytecode.CompileToBytecode and writeArtifact with no change to this
	// gate's failure policy.
	diags = append(diags, hfir.VerifyConstructs(root, string(target))...)

	var blocking []hfir.Diagnostic
	for _, d := range diags {
		if d.Severity == hfir.SeverityError && hfirBlockingCodes[d.Code] {
			blocking = append(blocking, d)
		}
	}
	if len(blocking) > 0 {
		reportHFIRDiagnostics(blocking)
	}
	return graph
}

// reportHFIRDiagnostics prints every ERROR diagnostic as one deterministic
// JSON array line (preserving Verify()'s node-order-derived ordering) and
// exits nonzero, mirroring ast.ReportError's one-line JSON contract without
// truncating to a single diagnostic - internal/hfir/verifier.go's Diagnostics
// is a slice for a reason, and there is no existing consumer of HFIR
// diagnostics to stay wire-compatible with yet.
func reportHFIRDiagnostics(diags []hfir.Diagnostic) {
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

func runSubcommand() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}
	cmd := os.Args[1]

	switch cmd {
	case "help":
		if len(os.Args) > 2 {
			sub := os.Args[2]
			switch sub {
			case "check", "build", "run", "version", "help":
				// Could implement specific help for subcommands, but generic help is requested
				printHelp()
			default:
				printHelp()
			}
		} else {
			printHelp()
		}
	case "version":
		fmt.Printf("HowlFrame %s\nHFBC format: %s\n", Version, HFBCFormatVersion)
	case "check":
		checkSource()
	case "build":
		buildSource()
	case "run":
		runArtifact()
	default:
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`HowlFrame v0.1 CLI

Usage:
  howlframe <command> [arguments]

Commands:
  check     Parse, type-check, and verify a .howl file without emitting an artifact.
  build     Compile a .howl file to a standalone .hfbc bytecode artifact.
  run       Execute a .hfbc bytecode artifact.
  version   Print version information.
  help      Print this help message.

Capabilities are denied by default and must be granted by the runner with --allow-caps.
Example: howlframe run --allow-caps filesystem,network app.hfbc arg1 arg2`)
}

func checkSource() {
	checkFlags := flag.NewFlagSet("check", flag.ExitOnError)
	checkFlags.Usage = func() {
		fmt.Println("Usage: howlframe check <source.howl>")
	}
	checkFlags.Parse(os.Args[2:])

	if checkFlags.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Missing source file")
		checkFlags.Usage()
		os.Exit(1)
	}

	inputFile := checkFlags.Arg(0)
	content, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read file: %v\n", err)
		os.Exit(1)
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
	_ = checker.Check(root)

	hfirModule := filepath.Base(inputFile)
	runHFIRGate(root, hfirModule, hfirTargetNone)

	fmt.Printf("OK: %s\n", inputFile)
}

func buildSource() {
	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)
	outPath := buildFlags.String("o", "", "output artifact file path")

	buildFlags.Usage = func() {
		fmt.Println("Usage: howlframe build <source.howl> [-o <output.hfbc>]")
		buildFlags.PrintDefaults()
	}
	buildFlags.Parse(os.Args[2:])

	if buildFlags.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Missing source file")
		buildFlags.Usage()
		os.Exit(1)
	}
	if *outPath != "" && strings.HasPrefix(*outPath, "-") {
		fmt.Fprintln(os.Stderr, "flag needs an argument: -o")
		buildFlags.Usage()
		os.Exit(1)
	}

	inputFile := buildFlags.Arg(0)
	// build has no argv-owning mode, so every token after the source file is
	// this subcommand's to parse or to reject (bugs.md #52).
	_, leftover, err := resumeFlagsAfterInput(buildFlags, buildFlags.Args()[1:])
	if errors.Is(err, flag.ErrHelp) {
		buildFlags.Usage()
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse arguments after source file %q: %v\n", inputFile, err)
		buildFlags.Usage()
		os.Exit(1)
	}
	if *outPath != "" && strings.HasPrefix(*outPath, "-") {
		fmt.Fprintf(os.Stderr, "Cannot parse arguments after source file %q: flag needs an argument: -o\n", inputFile)
		buildFlags.Usage()
		os.Exit(1)
	}
	if len(leftover) > 0 {
		fmt.Fprintf(os.Stderr, "Unexpected argument %q after source file %q\n", leftover[0], inputFile)
		buildFlags.Usage()
		os.Exit(1)
	}

	outFile := *outPath
	if outFile == "" {
		outFile = strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile)) + ".hfbc"
	}

	content, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read file: %v\n", err)
		os.Exit(1)
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
	_ = checker.Check(root)

	hfirModule := filepath.Base(inputFile)
	runHFIRGate(root, hfirModule, hfirTargetBytecode)

	prog := bytecode.CompileToBytecode(root)
	var buf bytes.Buffer
	if err := bytecode.WriteArtifact(&buf, prog); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode bytecode: %v\n", err)
		os.Exit(1)
	}

	if err := writeArtifact(outFile, buf.Bytes()); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write %s: %v\n", outFile, err)
		os.Exit(1)
	}

	fmt.Printf("Built %s\n", outFile)
}

func runArtifact() {
	runFlags := flag.NewFlagSet("run", flag.ExitOnError)
	allowCaps := runFlags.String("allow-caps", "", "comma-separated capabilities to allow (network,filesystem,process,environment,database)")
	maxInst := runFlags.Int("max-instructions", vm.DefaultLimits.MaxInstructions, "finite instruction limit")

	runFlags.Usage = func() {
		fmt.Println("Usage: howlframe run <artifact.hfbc> [options] [-- arguments...]")
		runFlags.PrintDefaults()
	}
	runFlags.Parse(os.Args[2:])

	if runFlags.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Missing artifact file")
		runFlags.Usage()
		os.Exit(1)
	}

	inputFile := runFlags.Arg(0)
	var programArgs []string
	if runFlags.NArg() > 1 {
		programArgs = runFlags.Args()[1:]
	}

	content, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read file: %v\n", err)
		os.Exit(1)
	}

	prog, err := bytecode.ReadArtifact(bytes.NewReader(content))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse bytecode: %v\n", err)
		os.Exit(1)
	}

	executionPolicy := vm.DefaultExecutionPolicy()
	if *maxInst <= 0 {
		fmt.Fprintln(os.Stderr, "-max-instructions must be greater than zero")
		os.Exit(1)
	}
	executionPolicy.Limits.MaxInstructions = *maxInst

	os.Exit(vm.RunBytecodeWithPolicy(prog, programArgs, executionPolicy, parseAllowedCaps(*allowCaps), os.Stdin, os.Stdout, os.Stderr))
}
