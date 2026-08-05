package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeepLetChainTranspilesBeyondLegacyDepthLimit(t *testing.T) {
	const chainLength = 2000

	var source strings.Builder
	source.WriteString("(cli_app ")
	for i := 0; i < chainLength; i++ {
		if i == 0 {
			fmt.Fprint(&source, "(let (value0 0) ")
		} else {
			fmt.Fprintf(&source, "(let (value%d value%d) ", i, i-1)
		}
	}
	fmt.Fprintf(&source, "(print value%d)%s)", chainLength-1, strings.Repeat(")", chainLength))

	zeroBinary := filepath.Join(t.TempDir(), "zero")
	if output, err := exec.Command("go", "build", "-o", zeroBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build zero binary: %v\n%s", err, output)
	}

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "deep-let.zero")
	if err := os.WriteFile(inputFile, []byte(source.String()), 0o644); err != nil {
		t.Fatalf("failed to write deep let chain: %v", err)
	}
	if output, err := exec.Command(zeroBinary, "-o", outDir, inputFile).CombinedOutput(); err != nil {
		t.Fatalf("deep let chain failed to transpile: %v\n%s", err, output)
	}

	generated, err := os.ReadFile(filepath.Join(outDir, "server.go"))
	if err != nil {
		t.Fatalf("failed to read generated server.go: %v", err)
	}
	lastBinding := fmt.Sprintf("value%d := value%d", chainLength-1, chainLength-2)
	if !strings.Contains(string(generated), lastBinding) {
		t.Fatalf("generated Go omitted the final let binding %q", lastBinding)
	}
	if output, err := exec.Command("go", "build", "-o", filepath.Join(outDir, "deep-let"), filepath.Join(outDir, "server.go")).CombinedOutput(); err != nil {
		t.Fatalf("generated Go for deep let chain did not build: %v\n%s", err, output)
	}
}

func TestOutputDirectoryFlag(t *testing.T) {
	// Build the zero binary
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	// Create a temporary directory for output
	outDir, err := os.MkdirTemp("", "zero-test-out-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	// Create a dummy .zero file
	inputFile := filepath.Join(outDir, "dummy.zero")
	if err := os.WriteFile(inputFile, []byte("(cli_app (print \"Hello\"))"), 0644); err != nil {
		t.Fatalf("Failed to write dummy file: %v", err)
	}

	// Run the zero binary with -o flag
	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run zero binary: %v", err)
	}

	// Check if server.go was created in the output directory
	serverFile := filepath.Join(outDir, "server.go")
	if _, err := os.Stat(serverFile); os.IsNotExist(err) {
		t.Errorf("Expected server.go to be created in %s, but it was not", outDir)
	}
}

func TestOutputDirectoryFlagAfterInputCreatesDirectoriesForEverySourceBackend(t *testing.T) {
	zeroBinary := filepath.Join(t.TempDir(), "zero")
	if output, err := exec.Command("go", "build", "-o", zeroBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build zero binary: %v\n%s", err, output)
	}

	tests := []struct {
		name     string
		source   string
		artifact string
	}{
		{
			name:     "go",
			source:   `(cli_app (print "Hello"))`,
			artifact: "server.go",
		},
		{
			name:     "javascript",
			source:   `(web_app (set_text (dom_query "#label") "Hello"))`,
			artifact: "app.js",
		},
		{
			name:     "wasm",
			source:   `(wasm_app (+ 1 2))`,
			artifact: "app.wat",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workDir := t.TempDir()
			inputFile := filepath.Join(workDir, test.name+".zero")
			outputDir := filepath.Join(workDir, "nested", "artifacts")
			if err := os.WriteFile(inputFile, []byte(test.source), 0o644); err != nil {
				t.Fatalf("failed to write input: %v", err)
			}

			command := exec.Command(zeroBinary, inputFile, "-o", outputDir)
			command.Dir = workDir
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("transpilation failed: %v\n%s", err, output)
			}
			if _, err := os.Stat(filepath.Join(outputDir, test.artifact)); err != nil {
				t.Fatalf("expected %s in requested output directory: %v", test.artifact, err)
			}
			if _, err := os.Stat(filepath.Join(workDir, test.artifact)); !os.IsNotExist(err) {
				t.Fatalf("unexpected artifact in working directory: %v", err)
			}
		})
	}
}

func TestBytecodeOutputFileFlagAfterInput(t *testing.T) {
	zeroBinary := filepath.Join(t.TempDir(), "zero")
	if output, err := exec.Command("go", "build", "-o", zeroBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build zero binary: %v\n%s", err, output)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "store.zero")
	outputFile := filepath.Join(tempDir, "nested", "store.zbc")
	if err := os.WriteFile(inputFile, []byte(`(cli_app (print "ok"))`), 0o644); err != nil {
		t.Fatalf("failed to write bytecode input: %v", err)
	}

	cmd := exec.Command(zeroBinary, "-compile-bc", inputFile, "-o", outputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile bytecode: %v\n%s", err, output)
	}
	if _, err := os.Stat(outputFile); err != nil {
		t.Fatalf("bytecode output was not written to %s: %v", outputFile, err)
	}
}

func TestRunBytecodeAllowCapsFlagGatesCapabilities(t *testing.T) {
	zeroBinary := filepath.Join(t.TempDir(), "zero")
	if output, err := exec.Command("go", "build", "-o", zeroBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build zero binary: %v\n%s", err, output)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "env_read.zero")
	source := `(cli_app (print "value:" (env "ZERO_TEST_ALLOW_CAPS_VAR")))`
	if err := os.WriteFile(inputFile, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write bytecode input: %v", err)
	}

	bytecodeFile := filepath.Join(tempDir, "env_read.zbc")
	if output, err := exec.Command(zeroBinary, "-compile-bc", inputFile, "-o", bytecodeFile).CombinedOutput(); err != nil {
		t.Fatalf("failed to compile bytecode: %v\n%s", err, output)
	}

	runWithEnv := func(env []string, args ...string) ([]byte, error) {
		cmd := exec.Command(zeroBinary, args...)
		cmd.Env = env
		return cmd.CombinedOutput()
	}

	if output, err := runWithEnv(os.Environ(), "-run-bc", bytecodeFile); err == nil {
		t.Fatalf("expected denial without -allow-caps, but command succeeded:\n%s", output)
	} else if !strings.Contains(string(output), "CAPABILITY_DENIED") {
		t.Fatalf("expected CAPABILITY_DENIED in output, got: %v\n%s", err, output)
	}

	allowedEnv := append(os.Environ(), "ZERO_TEST_ALLOW_CAPS_VAR=granted")
	if output, err := runWithEnv(allowedEnv, "-run-bc", "-allow-caps", "environment", bytecodeFile); err != nil {
		t.Fatalf("expected success with -allow-caps environment: %v\n%s", err, output)
	} else if !strings.Contains(string(output), "granted") {
		t.Fatalf("expected output to contain the env value, got:\n%s", output)
	}

	if output, err := runWithEnv(os.Environ(), "-run-bc", "-allow-caps", "not-a-real-capability", bytecodeFile); err == nil {
		t.Fatalf("expected rejection of an unknown -allow-caps value, but command succeeded:\n%s", output)
	}
}

func TestCompileWasmWritesSSAArtifactWithPhiControlFlow(t *testing.T) {
	zeroBinary := filepath.Join(t.TempDir(), "zero")
	if output, err := exec.Command("go", "build", "-o", zeroBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build zero binary: %v\n%s", err, output)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "native.zero")
	source := `(cli_app (let (x 4) (if (> x 2) (+ x 3) (- x 1))))`
	if err := os.WriteFile(inputFile, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write native backend input: %v", err)
	}

	if output, err := exec.Command(zeroBinary, "-compile-wasm", inputFile).CombinedOutput(); err != nil {
		t.Fatalf("failed to compile SSA WAT: %v\n%s", err, output)
	}
	defaultOutput := inputFile + ".ssa.wat"
	wat, err := os.ReadFile(defaultOutput)
	if err != nil {
		t.Fatalf("default SSA WAT output was not written to %s: %v", defaultOutput, err)
	}
	if !strings.Contains(string(wat), "(if (result i64)") {
		t.Fatalf("SSA WAT does not contain the lowered if/phi merge:\n%s", wat)
	}

	exactOutput := filepath.Join(tempDir, "nested", "exact.wat")
	if output, err := exec.Command(zeroBinary, "-compile-wasm", inputFile, "-o", exactOutput).CombinedOutput(); err != nil {
		t.Fatalf("failed to compile SSA WAT to exact output: %v\n%s", err, output)
	}
	if _, err := os.Stat(exactOutput); err != nil {
		t.Fatalf("exact SSA WAT output was not written to %s: %v", exactOutput, err)
	}
}

func TestCrashStateSerialization(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir, err := os.MkdirTemp("", "zero-crash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	inputFile := filepath.Join(outDir, "panic.zero")
	if err := os.WriteFile(inputFile, []byte(`(cli_app (let (z 0) (print (/ 1 z))))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to transpilation: %v\n%s", err, string(out))
	}

	serverFile := filepath.Join(outDir, "server.go")
	appBin := filepath.Join(outDir, "app")
	cmd = exec.Command("go", "build", "-o", appBin, serverFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build generated server.go: %v", err)
	}

	cmd = exec.Command(appBin)
	cmd.Dir = outDir
	_ = cmd.Run() // expected to exit with non-zero code

	crashFile := filepath.Join(outDir, "crash.json")
	data, err := os.ReadFile(crashFile)
	if err != nil {
		t.Fatalf("Failed to read crash.json: %v", err)
	}

	if len(data) == 0 {
		t.Fatalf("crash.json is empty")
	}
}

func TestWasmBackendWritesPortableWAT(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "math.zero")
	input := "(wasm_app (if (> (+ 2 3) 4) (return (* 6 7)) (return 0)))"
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate WAT: %v: %s", err, output)
	}

	wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
	if err != nil {
		t.Fatalf("Failed to read generated WAT: %v", err)
	}
	for _, fragment := range []string{
		"(module", "(func (export \"main\") (result i64)", "(if (result i64)", "(i64.gt_s", "(i64.add", "(i64.mul",
	} {
		if !strings.Contains(string(wat), fragment) {
			t.Errorf("Generated WAT is missing %q:\n%s", fragment, wat)
		}
	}
}

func TestWasmBackendRejectsUnsupportedNodes(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "unsupported.zero")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (env "TOKEN"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("Expected unsupported wasm node to fail")
	}
	if !strings.Contains(string(output), "Wasm backend does not support") {
		t.Errorf("Expected a clear unsupported-node diagnostic, got: %s", output)
	}
}

func TestWasmBackendRejectsSemanticLayoutErrorsBeforeOutput(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	tests := []struct {
		name   string
		input  string
		reason string
	}{
		{
			name:   "heterogeneous list",
			input:  `(wasm_app (list 1 "two"))`,
			reason: "list element 2 has type string, want int",
		},
		{
			name:   "incompatible branches",
			input:  `(wasm_app (if true 1 (to_float 2)))`,
			reason: "if branches have incompatible types int and float64",
		},
		{
			name:   "string aggregate expression",
			input:  `(wasm_app (list_get (list (to_string 1)) 0))`,
			reason: "to_string",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outDir := t.TempDir()
			inputFile := filepath.Join(outDir, "invalid.zero")
			if err := os.WriteFile(inputFile, []byte(test.input), 0644); err != nil {
				t.Fatalf("Failed to write input file: %v", err)
			}

			cmd = exec.Command("./zero", "-o", outDir, inputFile)
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("Expected semantic validation to fail")
			}
			if !strings.Contains(string(output), test.reason) {
				t.Fatalf("Expected diagnostic %q, got: %s", test.reason, output)
			}
			if _, statErr := os.Stat(filepath.Join(outDir, "app.wat")); !os.IsNotExist(statErr) {
				t.Fatalf("app.wat must not be written after semantic failure: %v", statErr)
			}
		})
	}
}

func TestWasmBackendAllocatesMultipleAggregateRegions(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "multi_aggregate.zero")
	input := `(wasm_app (if true (list_get (list 10 20) 1) (list_get (list 30 40) 0)))`
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate multi aggregate WAT: %v: %s", err, output)
	}
	wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
	if err != nil {
		t.Fatalf("Failed to read generated WAT: %v", err)
	}
	for _, fragment := range []string{
		`(data (i32.const 0)`,
		`(data (i32.const 24)`,
		`(i64.load (i32.const 0))`,
		`(i64.load (i32.const 24))`,
		`(i32.const 8)`,
		`(i32.const 32)`,
	} {
		if !strings.Contains(string(wat), fragment) {
			t.Errorf("Generated multi aggregate WAT is missing %q:\n%s", fragment, wat)
		}
	}
}

func TestWasmBackendDoesNotCountDictionaryKeyNamesAsAggregates(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "dict_key.zero")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (map_get (dict ("list" 10)) "list"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to generate WAT for dict key named list: %v: %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(outDir, "app.wat")); err != nil {
		t.Fatalf("Expected app.wat to be written: %v", err)
	}
}

func TestWasmBackendUsesFloatLayoutsAndConversions(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "float.zero")
	input := `(wasm_app (if (> (to_float 2) (to_float 1)) (return (to_float 7)) (return (to_float 0))))`
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate float WAT: %v: %s", err, output)
	}

	wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
	if err != nil {
		t.Fatalf("Failed to read generated WAT: %v", err)
	}
	for _, fragment := range []string{
		"(func (export \"main\") (result f64)", "(if (result f64)", "(f64.gt", "(f64.convert_i64_s (i64.const 2))",
	} {
		if !strings.Contains(string(wat), fragment) {
			t.Errorf("Generated float WAT is missing %q:\n%s", fragment, wat)
		}
	}
}

func TestWasmBackendEmitsIntegerListMemory(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "list.zero")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (list 1 2))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate list WAT: %v: %s", err, output)
	}
	wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
	if err != nil {
		t.Fatalf("Failed to read generated WAT: %v", err)
	}
	for _, fragment := range []string{
		"(memory (export \"memory\") 1)", "(data (i32.const 0)", "(func (export \"main\") (result i32)", "(i32.const 8)",
	} {
		if !strings.Contains(string(wat), fragment) {
			t.Errorf("Generated list WAT is missing %q:\n%s", fragment, wat)
		}
	}
}

func TestWasmBackendReadsIntegerListMemory(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "list_get.zero")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (list_get (list 10 20) 1))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate list_get WAT: %v: %s", err, output)
	}
	wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
	if err != nil {
		t.Fatalf("Failed to read generated WAT: %v", err)
	}
	for _, fragment := range []string{
		"(i64.load", "(i64.lt_u", "(i32.wrap_i64 (i64.const 1))", "(i32.mul", "(i64.const 0)", "(memory (export \"memory\") 1)",
	} {
		if !strings.Contains(string(wat), fragment) {
			t.Errorf("Generated list_get WAT is missing %q:\n%s", fragment, wat)
		}
	}
}

func TestWasmBackendInitializesDynamicIntegerAggregates(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	tests := []struct {
		name      string
		input     string
		fragments []string
	}{
		{
			name:  "list expression",
			input: `(wasm_app (list_get (list (+ 4 6) 20) 0))`,
			fragments: []string{
				"(i64.store (i32.const 8) (i64.add (i64.const 4) (i64.const 6)))",
				"(i64.load",
			},
		},
		{
			name:  "dictionary expression",
			input: `(wasm_app (map_get (dict ("answer" (+ 4 6))) "answer"))`,
			fragments: []string{
				"(i64.store (i32.const 256) (i64.add (i64.const 4) (i64.const 6)))",
				"(i64.load (i32.const 256))",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outDir := t.TempDir()
			inputFile := filepath.Join(outDir, "aggregate.zero")
			if err := os.WriteFile(inputFile, []byte(test.input), 0644); err != nil {
				t.Fatalf("Failed to write input file: %v", err)
			}
			cmd = exec.Command("./zero", "-o", outDir, inputFile)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("Failed to generate aggregate WAT: %v: %s", err, output)
			}
			wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
			if err != nil {
				t.Fatalf("Failed to read generated WAT: %v", err)
			}
			for _, fragment := range test.fragments {
				if !strings.Contains(string(wat), fragment) {
					t.Errorf("Generated WAT is missing %q:\n%s", fragment, wat)
				}
			}
		})
	}
}

func TestWasmBackendInitializesDynamicStringAggregatesAndKeys(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	tests := []struct {
		name      string
		input     string
		fragments []string
	}{
		{
			name:  "list string expression",
			input: `(wasm_app (list_get (list (if true "alpha" "beta")) 0))`,
			fragments: []string{
				"(i32.store (i32.const 8)",
				"(i32.load (i32.add (i32.const 8)",
				"\\61\\6c\\70\\68\\61\\00",
				"\\62\\65\\74\\61\\00",
			},
		},
		{
			name:  "dynamic integer dict key",
			input: `(wasm_app (map_get (dict ("a" 10) ("b" 20)) (if true "b" "a")))`,
			fragments: []string{
				"(i32.eq",
				"(i64.load (i32.const 256))",
				"(i64.load (i32.const 264))",
			},
		},
		{
			name:  "dynamic string dict value and key",
			input: `(wasm_app (map_get (dict ("a" "one") ("b" (if true "two" "alt"))) (if true "b" "a")))`,
			fragments: []string{
				"(i32.store (i32.const 20)",
				"(i32.load (i32.const 20))",
				"\\74\\77\\6f\\00",
				"\\61\\6c\\74\\00",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outDir := t.TempDir()
			inputFile := filepath.Join(outDir, "dynamic_string.zero")
			if err := os.WriteFile(inputFile, []byte(test.input), 0644); err != nil {
				t.Fatalf("Failed to write input file: %v", err)
			}
			cmd = exec.Command("./zero", "-o", outDir, inputFile)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("Failed to generate WAT: %v: %s", err, output)
			}
			wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
			if err != nil {
				t.Fatalf("Failed to read generated WAT: %v", err)
			}
			for _, fragment := range test.fragments {
				if !strings.Contains(string(wat), fragment) {
					t.Errorf("Generated WAT is missing %q:\n%s", fragment, wat)
				}
			}
		})
	}
}

func TestWasmBackendReadsStringListPointers(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "string_list_get.zero")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (list_get (list "alpha" "beta") 1))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate string-list WAT: %v: %s", err, output)
	}
	wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
	if err != nil {
		t.Fatalf("Failed to read generated WAT: %v", err)
	}
	for _, fragment := range []string{
		"(func (export \"main\") (result i32)", "(i32.load", "(i64.lt_u", "\\61\\6c\\70\\68\\61",
	} {
		if !strings.Contains(string(wat), fragment) {
			t.Errorf("Generated string-list WAT is missing %q:\n%s", fragment, wat)
		}
	}
}

func TestWasmBackendReadsStaticDictionaryValues(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "dict_get.zero")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (map_get (dict ("a" "one") ("b" "two")) "b"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate dict WAT: %v: %s", err, output)
	}
	wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
	if err != nil {
		t.Fatalf("Failed to read generated WAT: %v", err)
	}
	for _, fragment := range []string{
		"(func (export \"main\") (result i32)", "(i32.const 264)", "\\74\\77\\6f",
	} {
		if !strings.Contains(string(wat), fragment) {
			t.Errorf("Generated dict WAT is missing %q:\n%s", fragment, wat)
		}
	}
}

func TestWasmBackendReadsStaticIntegerDictionaryValues(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "zero", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build zero binary: %v", err)
	}
	defer os.Remove("zero")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "int_dict_get.zero")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (map_get (dict ("a" 10) ("b" 20)) "b"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to generate integer dict WAT: %v: %s", err, output)
	}
	wat, err := os.ReadFile(filepath.Join(outDir, "app.wat"))
	if err != nil {
		t.Fatalf("Failed to read generated WAT: %v", err)
	}
	for _, fragment := range []string{
		"(func (export \"main\") (result i64)", "(i64.load (i32.const 264))", "\\14\\00\\00\\00\\00\\00\\00\\00",
	} {
		if !strings.Contains(string(wat), fragment) {
			t.Errorf("Generated integer dict WAT is missing %q:\n%s", fragment, wat)
		}
	}
}

func TestMaskPlanIncludesSchemaBridge(t *testing.T) {
	source := `(cli_app
		(struct User (name string) (age int))
		(schema_bridge User (llm_generate "describe a user")))`
	input := filepath.Join(t.TempDir(), "bridge.zero")
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	command := exec.Command("go", "run", "zero.go", "-mask-plan", input)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("run -mask-plan: %v", err)
	}

	var plan struct {
		Bridges []struct {
			Target     string `json:"target"`
			Constraint struct {
				Kind   string `json:"kind"`
				Name   string `json:"name"`
				Fields []struct {
					Name string `json:"name"`
				} `json:"fields"`
			} `json:"constraint"`
		} `json:"bridges"`
	}
	if err := json.Unmarshal(output, &plan); err != nil {
		t.Fatalf("decode mask plan: %v\n%s", err, output)
	}
	if len(plan.Bridges) != 1 || plan.Bridges[0].Target != "User" ||
		plan.Bridges[0].Constraint.Kind != "struct" || plan.Bridges[0].Constraint.Name != "User" ||
		len(plan.Bridges[0].Constraint.Fields) != 2 {
		t.Fatalf("unexpected bridge plan: %+v", plan.Bridges)
	}
}

func TestSchemaBridgeEmitsWrappedSourceExpression(t *testing.T) {
	source := `(cli_app
		(struct User (name string) (age int))
		(print (schema_bridge User "ok")))`
	outDir := t.TempDir()
	input := filepath.Join(outDir, "bridge_run.zero")
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	command := exec.Command("go", "run", "zero.go", "-o", outDir, input)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("transpile schema_bridge: %v\n%s", err, output)
	}
	server := filepath.Join(outDir, "server.go")
	generated, err := os.ReadFile(server)
	if err != nil {
		t.Fatalf("read generated server: %v", err)
	}
	if !strings.Contains(string(generated), `fmt.Println("ok")`) {
		t.Fatalf("schema_bridge did not emit wrapped source expression:\n%s", generated)
	}
}

func TestOptimizationPlanIncludesSignatureMetadata(t *testing.T) {
	command := exec.Command("go", "run", "zero.go", "-optimization-plan", "tests/optimization_signature.zero")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("run -optimization-plan: %v", err)
	}

	var plan struct {
		Format     string `json:"format"`
		Signatures []struct {
			Name       string   `json:"name"`
			Metric     string   `json:"metric"`
			Tests      []string `json:"tests"`
			Candidates []struct {
				Label   string `json:"label"`
				Payload string `json:"payload"`
			} `json:"candidates"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
			BodyType string `json:"body_type"`
		} `json:"signatures"`
	}
	if err := json.Unmarshal(output, &plan); err != nil {
		t.Fatalf("decode optimization plan: %v\n%s", err, output)
	}
	if plan.Format != "zero.optimization_plan/v1" || len(plan.Signatures) != 1 {
		t.Fatalf("unexpected optimization plan: %+v", plan)
	}
	signature := plan.Signatures[0]
	if signature.Name != "support_prompt" || signature.Metric != "accuracy" ||
		len(signature.Tests) != 1 || signature.Tests[0] != "go test ./..." ||
		len(signature.Candidates) != 2 || signature.Candidates[0].Label != "baseline" ||
		signature.Candidates[1].Payload != "Answer only with verified facts." ||
		signature.Line != 2 || signature.Column != 1 || signature.BodyType != "void" {
		t.Fatalf("unexpected signature plan: %+v", signature)
	}
}

func TestOptimizationSignatureReportsMalformedCandidateAsJSON(t *testing.T) {
	input := filepath.Join(t.TempDir(), "invalid_optimization_signature.zero")
	source := `(cli_app
		(optimize_signature named
			(metric "accuracy")
			(test "go test ./...")
			(candidate baseline "prompt")
			(print "body")))`
	if err := os.WriteFile(input, []byte(source), 0o600); err != nil {
		t.Fatalf("write invalid signature: %v", err)
	}

	command := exec.Command("go", "run", "zero.go", "-optimization-plan", input)
	output, err := command.Output()
	if err == nil {
		t.Fatalf("malformed optimization signature unexpectedly succeeded:\n%s", output)
	}
	var reported struct {
		Reason string `json:"reason"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	if decodeErr := json.Unmarshal(output, &reported); decodeErr != nil {
		t.Fatalf("decode Zero JSON error: %v\n%s", decodeErr, output)
	}
	if reported.Reason != "optimize_signature candidate label must be a string" ||
		reported.Line != 5 || reported.Column <= 0 {
		t.Fatalf("unexpected Zero JSON error: %+v", reported)
	}
}

func TestOptimizationSignatureIsTransparentAcrossExecutionPaths(t *testing.T) {
	const expected = "optimization signature body\n"
	fixture := "tests/optimization_signature.zero"

	if output, err := exec.Command("go", "run", "zero.go", "-run", fixture).CombinedOutput(); err != nil {
		t.Fatalf("direct execution failed: %v\n%s", err, output)
	} else if string(output) != expected {
		t.Fatalf("direct output = %q, want %q", output, expected)
	}

	outDir := t.TempDir()
	if output, err := exec.Command("go", "run", "zero.go", "-o", outDir, fixture).CombinedOutput(); err != nil {
		t.Fatalf("Go codegen failed: %v\n%s", err, output)
	}
	generatedBinary := filepath.Join(outDir, "optimization-signature")
	if output, err := exec.Command("go", "build", "-o", generatedBinary, filepath.Join(outDir, "server.go")).CombinedOutput(); err != nil {
		t.Fatalf("generated Go did not build: %v\n%s", err, output)
	}
	if output, err := exec.Command(generatedBinary).CombinedOutput(); err != nil {
		t.Fatalf("generated program failed: %v\n%s", err, output)
	} else if string(output) != expected {
		t.Fatalf("generated output = %q, want %q", output, expected)
	}

	bytecodeFile := filepath.Join(outDir, "optimization-signature.zbc")
	if output, err := exec.Command("go", "run", "zero.go", "-compile-bc", fixture, "-o", bytecodeFile).CombinedOutput(); err != nil {
		t.Fatalf("bytecode compilation failed: %v\n%s", err, output)
	}
	if output, err := exec.Command("go", "run", "zero.go", "-run-bc", bytecodeFile).CombinedOutput(); err != nil {
		t.Fatalf("bytecode execution failed: %v\n%s", err, output)
	} else if string(output) != expected {
		t.Fatalf("bytecode output = %q, want %q", output, expected)
	}
}
