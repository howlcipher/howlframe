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
	"zero/internal/checker"
	"zero/internal/ir"
	"zero/internal/lexer"
	"zero/internal/masking"
	"zero/internal/optimization"
	"zero/internal/parser"
	"zero/internal/vm"
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
		os.Exit(vm.RunBytecode(&prog, flag.Args()[1:]))
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
		return
	}

	if *compileBc {
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
		os.Exit(vm.Interpret(root, flag.Args()[1:]))
	}

	if root != nil && root.Type == "List" && len(root.Children) > 0 && root.Children[0].Type == "SYMBOL" && root.Children[0].Value == "wasm_app" {
		wasmCode := wasm.GenerateWasmCode(root)
		wasmFile := filepath.Join(outputDir, "app.wat")
		if err = writeArtifact(wasmFile, []byte(wasmCode)); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", wasmFile, err), 0, 0)
		}
	} else if root != nil && root.Type == "List" && len(root.Children) > 0 && root.Children[0].Type == "SYMBOL" && root.Children[0].Value == "web_app" {
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

// writeArtifact creates the destination directory before writing an emitted
// artifact, so every backend follows the same output contract.
func writeArtifact(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}
