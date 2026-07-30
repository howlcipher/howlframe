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
	maskPlan := flag.Bool("mask-plan", false, "print the deterministic constrained-decoding mask plan and exit")
	optimizationPlan := flag.Bool("optimization-plan", false, "print the deterministic compile-time optimization plan and exit")
	flag.Parse()

	if flag.NArg() < 1 {
		ast.ReportError("Missing file argument", 0, 0)
	}
	inputFile := flag.Arg(0)

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
		if err = os.WriteFile(outFile, buf.Bytes(), 0644); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", outFile, err), 0, 0)
		}
		os.Exit(0)
	}

	if *compileWasm {
		expression, err := ssaWasmExpression(root)
		if err != nil {
			line, column := 0, 0
			if root != nil {
				line, column = root.Line, root.Column
			}
			ast.ReportError(err.Error(), line, column)
		}
		graph, err := ir.LowerSSA(expression)
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to lower SSA graph: %v", err), expression.Line, expression.Column)
		}
		wasmCode, err := wasm.SerializeSSA(graph)
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to serialize SSA graph: %v", err), expression.Line, expression.Column)
		}
		outFile := inputFile + ".ssa.wat"
		if outputFile := outputFlagAfterInput(os.Args[1:], inputFile); outputFile != "" {
			outFile = outputFile
		} else if *outDir != "" {
			outFile = filepath.Join(*outDir, filepath.Base(inputFile)+".ssa.wat")
		}
		if err = os.WriteFile(outFile, []byte(wasmCode), 0644); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", outFile, err), 0, 0)
		}
		os.Exit(0)
	}

	if *runMode {
		os.Exit(vm.Interpret(root, flag.Args()[1:]))
	}

	if root != nil && root.Type == "List" && len(root.Children) > 0 && root.Children[0].Type == "SYMBOL" && root.Children[0].Value == "wasm_app" {
		wasmCode := wasm.GenerateWasmCode(root)
		wasmFile := filepath.Join(*outDir, "app.wat")
		if err = os.WriteFile(wasmFile, []byte(wasmCode), 0644); err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", wasmFile, err), 0, 0)
		}
	} else if root != nil && root.Type == "List" && len(root.Children) > 0 && root.Children[0].Type == "SYMBOL" && root.Children[0].Value == "web_app" {
		jsCode, testCode := javascript.GenerateJSCode(root)

		appFile := filepath.Join(*outDir, "app.js")
		appTestFile := filepath.Join(*outDir, "app.test.js")

		err = os.WriteFile(appFile, []byte(jsCode), 0644)
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", appFile, err), 0, 0)
		}
		if testCode != "" {
			err = os.WriteFile(appTestFile, []byte(testCode), 0644)
			if err != nil {
				ast.ReportError(fmt.Sprintf("Failed to write %s: %v", appTestFile, err), 0, 0)
			}
		} else {
			os.Remove(appTestFile)
		}
	} else {
		goCode, testCode := gogen.GenerateCode(root)

		serverFile := filepath.Join(*outDir, "server.go")
		serverTestFile := filepath.Join(*outDir, "server_test.go")

		err = os.WriteFile(serverFile, []byte(goCode), 0644)
		if err != nil {
			ast.ReportError(fmt.Sprintf("Failed to write %s: %v", serverFile, err), 0, 0)
		}

		if testCode != "" {
			err = os.WriteFile(serverTestFile, []byte(testCode), 0644)
			if err != nil {
				ast.ReportError(fmt.Sprintf("Failed to write %s: %v", serverTestFile, err), 0, 0)
			}
		} else {
			os.Remove(serverTestFile)
		}
	}
}

func ssaWasmExpression(root *ast.Node) (*ast.Node, error) {
	if root == nil || root.Type != "List" || len(root.Children) == 0 ||
		root.Children[0].Type != "SYMBOL" || root.Children[0].Value != "cli_app" {
		return nil, fmt.Errorf("-compile-wasm requires a cli_app root")
	}
	if len(root.Children) != 2 {
		return nil, fmt.Errorf("-compile-wasm requires cli_app to contain exactly one expression")
	}
	return root.Children[1], nil
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
