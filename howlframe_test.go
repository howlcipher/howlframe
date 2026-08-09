package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/construct"
	"github.com/howlcipher/howlframe/internal/hfir"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
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

	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", err, output)
	}

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "deep-let.howl")
	if err := os.WriteFile(inputFile, []byte(source.String()), 0o644); err != nil {
		t.Fatalf("failed to write deep let chain: %v", err)
	}
	if output, err := exec.Command(howlframeBinary, "-o", outDir, inputFile).CombinedOutput(); err != nil {
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
	// Build the HowlFrame binary
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	// Create a temporary directory for output
	outDir, err := os.MkdirTemp("", "howlframe-test-out-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	// Create a dummy .howl file
	inputFile := filepath.Join(outDir, "dummy.howl")
	if err := os.WriteFile(inputFile, []byte("(cli_app (print \"Hello\"))"), 0644); err != nil {
		t.Fatalf("Failed to write dummy file: %v", err)
	}

	// Run the HowlFrame binary with -o flag
	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to run HowlFrame binary: %v", err)
	}

	// Check if server.go was created in the output directory
	serverFile := filepath.Join(outDir, "server.go")
	if _, err := os.Stat(serverFile); os.IsNotExist(err) {
		t.Errorf("Expected server.go to be created in %s, but it was not", outDir)
	}
}

func TestOutputDirectoryFlagAfterInputCreatesDirectoriesForEverySourceBackend(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", err, output)
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
			inputFile := filepath.Join(workDir, test.name+".howl")
			outputDir := filepath.Join(workDir, "nested", "artifacts")
			if err := os.WriteFile(inputFile, []byte(test.source), 0o644); err != nil {
				t.Fatalf("failed to write input: %v", err)
			}

			command := exec.Command(howlframeBinary, inputFile, "-o", outputDir)
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
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", err, output)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "store.howl")
	outputFile := filepath.Join(tempDir, "nested", "store.hfbc")
	if err := os.WriteFile(inputFile, []byte(`(cli_app (print "ok"))`), 0o644); err != nil {
		t.Fatalf("failed to write bytecode input: %v", err)
	}

	cmd := exec.Command(howlframeBinary, "-compile-bc", inputFile, "-o", outputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile bytecode: %v\n%s", err, output)
	}
	if _, err := os.Stat(outputFile); err != nil {
		t.Fatalf("bytecode output was not written to %s: %v", outputFile, err)
	}
}

func TestRunBytecodeAllowCapsFlagGatesCapabilities(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", err, output)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "env_read.howl")
	source := `(cli_app (print "value:" (env "HOWLFRAME_TEST_ALLOW_CAPS_VAR")))`
	if err := os.WriteFile(inputFile, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write bytecode input: %v", err)
	}

	bytecodeFile := filepath.Join(tempDir, "env_read.hfbc")
	if output, err := exec.Command(howlframeBinary, "-compile-bc", inputFile, "-o", bytecodeFile).CombinedOutput(); err != nil {
		t.Fatalf("failed to compile bytecode: %v\n%s", err, output)
	}

	runWithEnv := func(env []string, args ...string) ([]byte, error) {
		cmd := exec.Command(howlframeBinary, args...)
		cmd.Env = env
		return cmd.CombinedOutput()
	}

	if output, err := runWithEnv(os.Environ(), "-run-bc", bytecodeFile); err == nil {
		t.Fatalf("expected denial without -allow-caps, but command succeeded:\n%s", output)
	} else if !strings.Contains(string(output), "CAPABILITY_DENIED") {
		t.Fatalf("expected CAPABILITY_DENIED in output, got: %v\n%s", err, output)
	}

	if output, err := runWithEnv(os.Environ(), "-run-bc", "--max-instructions", "1000000", bytecodeFile); err == nil {
		t.Fatalf("expected denial with a large instruction budget but no capability, command succeeded:\n%s", output)
	} else if !strings.Contains(string(output), "CAPABILITY_DENIED") {
		t.Fatalf("expected CAPABILITY_DENIED under a custom instruction budget, got: %v\n%s", err, output)
	}

	allowedEnv := append(os.Environ(), "HOWLFRAME_TEST_ALLOW_CAPS_VAR=granted")
	if output, err := runWithEnv(allowedEnv, "-run-bc", "-allow-caps", "environment", bytecodeFile); err != nil {
		t.Fatalf("expected success with -allow-caps environment: %v\n%s", err, output)
	} else if !strings.Contains(string(output), "granted") {
		t.Fatalf("expected output to contain the env value, got:\n%s", output)
	}

	if output, err := runWithEnv(os.Environ(), "-run-bc", "-allow-caps", "not-a-real-capability", bytecodeFile); err == nil {
		t.Fatalf("expected rejection of an unknown -allow-caps value, but command succeeded:\n%s", output)
	}
}

func TestRunBytecodeMaxInstructionsFlag(t *testing.T) {
	howlframeBinary := buildHowlFrameBinaryForTest(t)
	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "print.howl")
	bytecodeFile := filepath.Join(tempDir, "print.hfbc")
	if err := os.WriteFile(inputFile, []byte(`(cli_app (print "ok"))`), 0o644); err != nil {
		t.Fatalf("write bytecode input: %v", err)
	}
	if output, err := exec.Command(howlframeBinary, "-compile-bc", inputFile, "-o", bytecodeFile).CombinedOutput(); err != nil {
		t.Fatalf("compile bytecode: %v\n%s", err, output)
	}

	tests := []struct {
		name       string
		args       []string
		wantOutput string
		wantError  string
	}{
		{
			name:       "omitted preserves default",
			args:       []string{"-run-bc", bytecodeFile},
			wantOutput: "ok\n",
		},
		{
			name:      "explicit smaller limit",
			args:      []string{"-run-bc", "--max-instructions", "1", bytecodeFile},
			wantError: `"code":"LIMIT_EXCEEDED"`,
		},
		{
			name:       "exact required limit",
			args:       []string{"-run-bc", "--max-instructions", "2", bytecodeFile},
			wantOutput: "ok\n",
		},
		{
			name:       "greater limit",
			args:       []string{"-run-bc", "--max-instructions", "3", bytecodeFile},
			wantOutput: "ok\n",
		},
		{
			name:      "zero rejected",
			args:      []string{"-run-bc", "--max-instructions", "0", bytecodeFile},
			wantError: "-max-instructions must be greater than zero",
		},
		{
			name:      "negative rejected",
			args:      []string{"-run-bc", "--max-instructions", "-1", bytecodeFile},
			wantError: "-max-instructions must be greater than zero",
		},
		{
			name:      "malformed rejected",
			args:      []string{"-run-bc", "--max-instructions", "invalid", bytecodeFile},
			wantError: "invalid value",
		},
		{
			name:      "overflow rejected",
			args:      []string{"-run-bc", "--max-instructions", "999999999999999999999999999999", bytecodeFile},
			wantError: "value out of range",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := exec.Command(howlframeBinary, test.args...).CombinedOutput()
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("command failed: %v\n%s", err, output)
				}
				if string(output) != test.wantOutput {
					t.Fatalf("output = %q, want %q", output, test.wantOutput)
				}
				return
			}
			if err == nil {
				t.Fatalf("command succeeded, want error containing %q; output: %s", test.wantError, output)
			}
			if !strings.Contains(string(output), test.wantError) {
				t.Fatalf("error output = %q, want substring %q", output, test.wantError)
			}
			if strings.Contains(string(output), "ok\n") {
				t.Fatalf("application executed before invalid/insufficient policy failed: %s", output)
			}
		})
	}
}

func TestCompileWasmWritesSSAArtifactWithPhiControlFlow(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", err, output)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "native.howl")
	source := `(cli_app (let (x 4) (if (> x 2) (+ x 3) (- x 1))))`
	if err := os.WriteFile(inputFile, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write native backend input: %v", err)
	}

	if output, err := exec.Command(howlframeBinary, "-compile-wasm", inputFile).CombinedOutput(); err != nil {
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
	if output, err := exec.Command(howlframeBinary, "-compile-wasm", inputFile, "-o", exactOutput).CombinedOutput(); err != nil {
		t.Fatalf("failed to compile SSA WAT to exact output: %v\n%s", err, output)
	}
	if _, err := os.Stat(exactOutput); err != nil {
		t.Fatalf("exact SSA WAT output was not written to %s: %v", exactOutput, err)
	}
}

func TestCrashStateSerialization(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir, err := os.MkdirTemp("", "howlframe-crash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	inputFile := filepath.Join(outDir, "panic.howl")
	if err := os.WriteFile(inputFile, []byte(`(cli_app (let (z 0) (print (/ 1 z))))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "math.howl")
	input := "(wasm_app (if (> (+ 2 3) 4) (return (* 6 7)) (return 0)))"
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "unsupported.howl")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (env "TOKEN"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("Expected unsupported wasm node to fail")
	}
	if !strings.Contains(string(output), "Wasm backend does not support") {
		t.Errorf("Expected a clear unsupported-node diagnostic, got: %s", output)
	}
}

func TestWasmBackendRejectsSemanticLayoutErrorsBeforeOutput(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

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
			inputFile := filepath.Join(outDir, "invalid.howl")
			if err := os.WriteFile(inputFile, []byte(test.input), 0644); err != nil {
				t.Fatalf("Failed to write input file: %v", err)
			}

			cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "multi_aggregate.howl")
	input := `(wasm_app (if true (list_get (list 10 20) 1) (list_get (list 30 40) 0)))`
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "dict_key.howl")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (map_get (dict ("list" 10)) "list"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to generate WAT for dict key named list: %v: %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(outDir, "app.wat")); err != nil {
		t.Fatalf("Expected app.wat to be written: %v", err)
	}
}

func TestWasmBackendUsesFloatLayoutsAndConversions(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "float.howl")
	input := `(wasm_app (if (> (to_float 2) (to_float 1)) (return (to_float 7)) (return (to_float 0))))`
	if err := os.WriteFile(inputFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "list.howl")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (list 1 2))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "list_get.howl")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (list_get (list 10 20) 1))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

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
			inputFile := filepath.Join(outDir, "aggregate.howl")
			if err := os.WriteFile(inputFile, []byte(test.input), 0644); err != nil {
				t.Fatalf("Failed to write input file: %v", err)
			}
			cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

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
			inputFile := filepath.Join(outDir, "dynamic_string.howl")
			if err := os.WriteFile(inputFile, []byte(test.input), 0644); err != nil {
				t.Fatalf("Failed to write input file: %v", err)
			}
			cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "string_list_get.howl")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (list_get (list "alpha" "beta") 1))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "dict_get.howl")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (map_get (dict ("a" "one") ("b" "two")) "b"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	cmd := exec.Command("go", "build", "-o", "howlframe", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build HowlFrame binary: %v", err)
	}
	defer os.Remove("howlframe")

	outDir := t.TempDir()
	inputFile := filepath.Join(outDir, "int_dict_get.howl")
	if err := os.WriteFile(inputFile, []byte(`(wasm_app (map_get (dict ("a" 10) ("b" 20)) "b"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}
	cmd = exec.Command("./howlframe", "-o", outDir, inputFile)
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
	input := filepath.Join(t.TempDir(), "bridge.howl")
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	command := exec.Command("go", "run", "howlframe.go", "-mask-plan", input)
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
	input := filepath.Join(outDir, "bridge_run.howl")
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	command := exec.Command("go", "run", "howlframe.go", "-o", outDir, input)
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
	command := exec.Command("go", "run", "howlframe.go", "-optimization-plan", "tests/optimization_signature.howl")
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
	if plan.Format != "howlframe.optimization_plan/v1" || len(plan.Signatures) != 1 {
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
	input := filepath.Join(t.TempDir(), "invalid_optimization_signature.howl")
	source := `(cli_app
		(optimize_signature named
			(metric "accuracy")
			(test "go test ./...")
			(candidate baseline "prompt")
			(print "body")))`
	if err := os.WriteFile(input, []byte(source), 0o600); err != nil {
		t.Fatalf("write invalid signature: %v", err)
	}

	command := exec.Command("go", "run", "howlframe.go", "-optimization-plan", input)
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
		t.Fatalf("decode HowlFrame JSON error: %v\n%s", decodeErr, output)
	}
	if reported.Reason != "optimize_signature candidate label must be a string" ||
		reported.Line != 5 || reported.Column <= 0 {
		t.Fatalf("unexpected HowlFrame JSON error: %+v", reported)
	}
}

func TestOptimizationSignatureIsTransparentAcrossExecutionPaths(t *testing.T) {
	const expected = "optimization signature body\n"
	fixture := "tests/optimization_signature.howl"

	if output, err := exec.Command("go", "run", "howlframe.go", "-run", fixture).CombinedOutput(); err != nil {
		t.Fatalf("direct execution failed: %v\n%s", err, output)
	} else if string(output) != expected {
		t.Fatalf("direct output = %q, want %q", output, expected)
	}

	outDir := t.TempDir()
	if output, err := exec.Command("go", "run", "howlframe.go", "-o", outDir, fixture).CombinedOutput(); err != nil {
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

	bytecodeFile := filepath.Join(outDir, "optimization-signature.hfbc")
	if output, err := exec.Command("go", "run", "howlframe.go", "-compile-bc", fixture, "-o", bytecodeFile).CombinedOutput(); err != nil {
		t.Fatalf("bytecode compilation failed: %v\n%s", err, output)
	}
	if output, err := exec.Command("go", "run", "howlframe.go", "-run-bc", bytecodeFile).CombinedOutput(); err != nil {
		t.Fatalf("bytecode execution failed: %v\n%s", err, output)
	} else if string(output) != expected {
		t.Fatalf("bytecode output = %q, want %q", output, expected)
	}
}

// TestHFIRGateAcceptsAllExistingFixtures is the empirical check on
// improvement #87 Phase 2's production HFIR gate: it must not false-positive
// on any real, currently-passing tests/*.howl fixture. This must pass before
// any other HFIR-gate test result is trusted. One fixture is skipped as a
// pre-existing, HFIR-unrelated failure that already fails checker.Check
// before runHFIRGate is ever reached (confirmed by running it through
// -validate on the pre-HFIR-gate binary): tests/routes.howl is an
// include-only fragment with no standalone root, so it is only ever valid
// when pulled in by an importer. tests/test_include.howl was skipped here
// too until bugs.md #43 was fixed; it now passes the sweep unexempted.
// tests/module_math.howl (improvements.md #95) is skipped for the identical
// reason as routes.howl: it is a bare (module ...) file with no importer, so
// it is only ever valid when pulled in via tests/module_main.howl's (use
// ...), which itself is a real cli_app root and passes the sweep unexempted.
func TestHFIRGateAcceptsAllExistingFixtures(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	skip := map[string]bool{
		"routes.howl":      true,
		"module_math.howl": true,
	}

	fixtures, err := filepath.Glob("tests/*.howl")
	if err != nil {
		t.Fatalf("failed to glob tests/*.howl: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("expected at least one tests/*.howl fixture")
	}

	for _, fixture := range fixtures {
		name := filepath.Base(fixture)
		if skip[name] {
			continue
		}
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(howlframeBinary, "-validate", fixture)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("HFIR gate false-positived on a known-good fixture: %v\n%s", err, output)
			}
		})
	}
}

// TestHFIRGateRejectsTargetInfeasibleWasmConstructBeforeArtifact proves
// genuinely new coverage from wiring HFIR verification into -compile-wasm:
// spawn_agent/exec are not structurally rejected by checker.Check for a
// cli_app root (only wasm_app roots get that rejection, via
// checker.checkWasmApp's own construct whitelist, independent of HFIR - see
// the comment on the skipped legacy-wasm_app case below), so this is the
// first point in the pipeline that rejects them for the wasm target.
func TestHFIRGateRejectsTargetInfeasibleWasmConstructBeforeArtifact(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "infeasible.howl")
	source := `(cli_app (spawn_agent "x" (task "y")))`
	if err := os.WriteFile(inputFile, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	cmd := exec.Command(howlframeBinary, "-compile-wasm", inputFile)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected -compile-wasm to reject a wasm-target-infeasible construct, got success:\n%s", output)
	}
	if !strings.Contains(string(output), "HFIR_TARGET_INFEASIBLE") {
		t.Fatalf("expected HFIR_TARGET_INFEASIBLE in output, got: %s", output)
	}
	if _, statErr := os.Stat(inputFile + ".ssa.wat"); !os.IsNotExist(statErr) {
		t.Fatalf(".ssa.wat must not be written after a HFIR gate rejection: %v", statErr)
	}

	// Note: the equivalent legacy wasm_app-root fixture
	// ("(wasm_app (spawn_agent ...))") is NOT tested here because
	// checker.checkWasmApp already rejects it structurally at
	// checker.Check time (howlframe.go:79), before this branch's runHFIRGate call
	// is ever reached - confirmed live during implementation. That path's
	// rejection predates and is independent of this phase's HFIR wiring, so
	// it would not exercise anything new.
}

// TestHFIRGateDiagnosticOrderingIsDeterministic is the CLI-level companion to
// internal/hfir/verifier_test.go's TestVerifierDiagnosticOrderIsDeterministic,
// confirming the same ordering guarantee survives through
// reportHFIRDiagnostics's JSON array output.
func TestHFIRGateDiagnosticOrderingIsDeterministic(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "two_infeasible.howl")
	source := `(cli_app (spawn_agent "x" (task "y")) (exec "ls"))`
	if err := os.WriteFile(inputFile, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	var outputs []string
	for i := 0; i < 3; i++ {
		output, err := exec.Command(howlframeBinary, "-compile-wasm", inputFile).CombinedOutput()
		if err == nil {
			t.Fatalf("run %d: expected -compile-wasm to reject, got success:\n%s", i, output)
		}
		outputs = append(outputs, string(output))
	}
	for i := 1; i < len(outputs); i++ {
		if outputs[i] != outputs[0] {
			t.Fatalf("diagnostic output differs across repeated runs:\nrun 0: %s\nrun %d: %s", outputs[0], i, outputs[i])
		}
	}
	var diags []struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(outputs[0]), &diags); err != nil {
		t.Fatalf("failed to parse diagnostic JSON array: %v\n%s", err, outputs[0])
	}
	if len(diags) != 2 || diags[0].Code != "HFIR_TARGET_INFEASIBLE" || diags[1].Code != "HFIR_TARGET_INFEASIBLE" {
		t.Fatalf("expected exactly 2 HFIR_TARGET_INFEASIBLE diagnostics in order, got: %v", diags)
	}
}

// TestValidateModeRunsHFIRGateAndHasNoSideEffects confirms -validate remains
// side-effect free now that it also runs the HFIR gate: a valid fixture
// succeeds and leaves no files behind anywhere in the working directory.
func TestValidateModeRunsHFIRGateAndHasNoSideEffects(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	workDir := t.TempDir()
	inputFile := filepath.Join(workDir, "valid.howl")
	if err := os.WriteFile(inputFile, []byte(`(cli_app (print "ok"))`), 0o644); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	before, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatalf("failed to read workDir before running -validate: %v", err)
	}

	cmd := exec.Command(howlframeBinary, "-validate", inputFile)
	cmd.Dir = workDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("-validate failed on a valid fixture: %v\n%s", err, output)
	}

	after, err := os.ReadDir(workDir)
	if err != nil {
		t.Fatalf("failed to read workDir after running -validate: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("-validate left files behind: before=%v after=%v", before, after)
	}
}

// TestHFIRGateForCompileBcDoesNotRegressValidPrograms confirms the HFIR gate
// added to -compile-bc doesn't break a valid program's bytecode compilation.
// Note: unlike -compile-wasm, there is currently no way to make this gate
// reject real HowlFrame source through -compile-bc - HFIR_UNBOUND_REF is excluded
// from the production gate's blocking set (bugs.md #42), HFIR_INVALID_REF
// cannot arise from LowerAST's own construction on real parsed source (every
// data/control edge it creates always points at a node it just added), and
// isFeasible has no rules for the "bytecode" target. This is documented
// honestly rather than tested against a contrived synthetic failure -
// bytecode-specific target-feasibility rules are out of scope for this
// phase (see improvements.md #87's Phase 2 status note).
func TestHFIRGateForCompileBcDoesNotRegressValidPrograms(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "valid.howl")
	outputFile := filepath.Join(tempDir, "valid.bc.bin")
	if err := os.WriteFile(inputFile, []byte(`(cli_app (print "ok"))`), 0o644); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	cmd := exec.Command(howlframeBinary, "-compile-bc", inputFile, "-o", outputFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("-compile-bc failed on a valid program after adding the HFIR gate: %v\n%s", err, output)
	}
	if _, err := os.Stat(outputFile); err != nil {
		t.Fatalf("bytecode output was not written: %v", err)
	}
}

// TestHFIRTargetBytecodeMatchesVerifierConstant keeps the CLI's target
// identity and internal/hfir's construct-support rule spelled the same way. If
// they drift, -compile-bc silently stops being gated at all.
func TestHFIRTargetBytecodeMatchesVerifierConstant(t *testing.T) {
	if string(hfirTargetBytecode) != hfir.TargetBytecode {
		t.Fatalf("hfirTargetBytecode = %q, but hfir.TargetBytecode = %q", hfirTargetBytecode, hfir.TargetBytecode)
	}
}

// TestCompileBcFailsClosedOnUnsupportedConstruct is the direct regression test
// for bugs.md #45. Before the fix, this fixture compiled to an artifact with
// exit code 0 and then ran producing no output at all - the match expression,
// which should print zero/one/other, was dropped by compileNode's switch
// because it had no default case.
func TestCompileBcFailsClosedOnUnsupportedConstruct(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	outputFile := filepath.Join(t.TempDir(), "advanced.bc.bin")
	cmd := exec.Command(howlframeBinary, "-compile-bc", "tests/test_advanced_control.howl", "-o", outputFile)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected -compile-bc to reject an unsupported construct, got success:\n%s", output)
	}
	for _, want := range []string{"HFIR_TARGET_INFEASIBLE", `\"match\"`} {
		if !strings.Contains(string(output), want) {
			t.Errorf("expected %s in output, got: %s", want, output)
		}
	}
	if _, statErr := os.Stat(outputFile); !os.IsNotExist(statErr) {
		t.Fatalf("no artifact may be written after a fail-closed rejection: %v", statErr)
	}
}

// TestCompileBcFailsClosedCitingOwningTracker proves the diagnostic points at
// the backlog item that owns the gap, so the failure is actionable rather than
// just a wall.
func TestCompileBcFailsClosedCitingOwningTracker(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	outputFile := filepath.Join(t.TempDir(), "void.bc.bin")
	cmd := exec.Command(howlframeBinary, "-compile-bc", "tests/test_void_defun.howl", "-o", outputFile)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected -compile-bc to reject a test block, got success:\n%s", output)
	}
	for _, want := range []string{"HFIR_TARGET_INFEASIBLE", `\"test\"`, "improvements.md #96"} {
		if !strings.Contains(string(output), want) {
			t.Errorf("expected %s in output, got: %s", want, output)
		}
	}
	if _, statErr := os.Stat(outputFile); !os.IsNotExist(statErr) {
		t.Fatalf("no artifact may be written after a fail-closed rejection: %v", statErr)
	}
}

// TestCompileBcFailsClosedOnUnknownHead covers the open-ended half of the bug:
// a head nobody has ever implemented used to pass -validate AND -compile-bc,
// and the resulting program ran to exit 0 while skipping it.
func TestCompileBcFailsClosedOnUnknownHead(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "bogus.howl")
	outputFile := filepath.Join(tempDir, "bogus.bc.bin")
	source := `(cli_app (do (print "before") (totally_made_up_head "x" 42) (print "after")))`
	if err := os.WriteFile(inputFile, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	cmd := exec.Command(howlframeBinary, "-compile-bc", inputFile, "-o", outputFile)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected -compile-bc to reject an unknown head, got success:\n%s", output)
	}
	if !strings.Contains(string(output), "totally_made_up_head") {
		t.Errorf("expected the offending head to be named, got: %s", output)
	}
	if _, statErr := os.Stat(outputFile); !os.IsNotExist(statErr) {
		t.Fatalf("no artifact may be written after a fail-closed rejection: %v", statErr)
	}
}

// TestCompileBcAcceptsCompileTimeOnlyAnnotations proves the fix distinguishes
// "emits no instructions because it is an annotation" from "emits no
// instructions because nobody implemented it". type_hint, type_hints and
// type_param must still compile AND run unchanged.
func TestCompileBcAcceptsCompileTimeOnlyAnnotations(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "annotated.howl")
	outputFile := filepath.Join(tempDir, "annotated.bc.bin")
	source := `(cli_app
  (defun add (a b)
    (type_hints (a int) (b int) (return int))
    (return (+ a b))
  )
  (defun shout (a)
    (type_hint return "void")
    (print a)
  )
  (do
    (print (call add 2 3))
    (call shout "done")
  )
)`
	if err := os.WriteFile(inputFile, []byte(source), 0o644); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	if output, err := exec.Command(howlframeBinary, "-compile-bc", inputFile, "-o", outputFile).CombinedOutput(); err != nil {
		t.Fatalf("-compile-bc rejected a CompileTimeOnly annotation: %v\n%s", err, output)
	}
	if _, err := os.Stat(outputFile); err != nil {
		t.Fatalf("bytecode output was not written: %v", err)
	}

	output, err := exec.Command(howlframeBinary, "-run-bc", outputFile).CombinedOutput()
	if err != nil {
		t.Fatalf("-run-bc failed: %v\n%s", err, output)
	}
	got := strings.TrimSpace(string(output))
	if got != "5\ndone" {
		t.Fatalf("-run-bc output = %q, want \"5\\ndone\"", got)
	}
}

// TestCompileBcCorpusPartitionMatchesRegistry is the empirical guard for the
// whole change, and it exists because TestHFIRGateAcceptsAllExistingFixtures
// cannot serve that role: that test runs -validate, which uses hfirTargetNone,
// and internal/hfir's Verify only applies target rules for a non-empty target -
// so it never reaches the construct-support check at all.
//
// For every tracked fixture, -compile-bc's exit status must agree exactly with
// construct.Scan. A fixture that fails closed without a registered violation
// is a false positive; one that compiles despite a violation means the gate
// was bypassed.
func TestCompileBcCorpusPartitionMatchesRegistry(t *testing.T) {
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}

	// routes.howl is an include-only fragment with no standalone root, so it
	// fails earlier, in checker.Check, for reasons unrelated to construct
	// support. (tests/test_include.howl was skipped here too until bugs.md
	// #43 was fixed.) module_math.howl (improvements.md #95) is the same
	// shape: a bare (module ...) file with no importer, valid only via
	// module_main.howl's (use ...).
	skip := map[string]bool{
		"routes.howl":      true,
		"module_math.howl": true,
	}

	fixtures, err := filepath.Glob("tests/*.howl")
	if err != nil {
		t.Fatalf("failed to glob tests/*.howl: %v", err)
	}
	if len(fixtures) == 0 {
		t.Fatal("expected at least one tests/*.howl fixture")
	}

	outDir := t.TempDir()
	for _, fixture := range fixtures {
		name := filepath.Base(fixture)
		if skip[name] {
			continue
		}
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}
			// Scan the same AST the real -compile-bc path scans: howlframe.go
			// expands includes and resolves modules before lowering, so a
			// raw parse would see (use ...) heads the compiler never does
			// and report violations -compile-bc does not.
			root := parser.NewParser(lexer.NewLexer(string(source)), name).ParseExpression()
			parser.ExpandIncludes(root, filepath.Dir(fixture), 0)
			ast.ResolveModules(root)
			violations := construct.Scan(root)

			outputFile := filepath.Join(outDir, name+".bc.bin")
			output, runErr := exec.Command(howlframeBinary, "-compile-bc", fixture, "-o", outputFile).CombinedOutput()

			if len(violations) == 0 {
				if runErr != nil {
					t.Fatalf("-compile-bc failed on a fixture the registry considers supported: %v\n%s", runErr, output)
				}
				if _, err := os.Stat(outputFile); err != nil {
					t.Fatalf("bytecode output was not written: %v", err)
				}
				return
			}

			if runErr == nil {
				t.Fatalf("-compile-bc accepted a fixture containing %q; the gate was bypassed:\n%s",
					violations[0].Name, output)
			}
			if !strings.Contains(string(output), violations[0].Name) {
				t.Errorf("expected the diagnostic to name %q, got: %s", violations[0].Name, output)
			}
			if _, statErr := os.Stat(outputFile); !os.IsNotExist(statErr) {
				t.Fatalf("no artifact may be written after a fail-closed rejection: %v", statErr)
			}
		})
	}
}

// The tests below cover improvements.md #95's remaining acceptance criteria
// beyond the corpus sweeps above: negative paths (private access, missing
// module, nested/circular imports), the compile-time-linking proof (bytecode
// runs after the source module disappears), interpreter/bytecode parity, and
// HFIR provenance. tests/module_math.howl and tests/module_main.howl already
// prove the happy path through every real sweep (-validate, -compile-bc,
// construct.Scan, tools/difftest); these are the shapes that must not become
// permanent corpus fixtures, so they use scratch t.TempDir() files instead.

func buildHowlFrameBinaryForTest(t *testing.T) string {
	t.Helper()
	howlframeBinary := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", howlframeBinary, ".").CombinedOutput(); err != nil {
		t.Fatalf("failed to build HowlFrame binary: %v\n%s", output, err)
	}
	return howlframeBinary
}

// TestModuleInterpreterAndBytecodeParity proves -run and -compile-bc+-run-bc
// agree on the same multi-file module program, using the checked-in fixture
// through the real pipeline (not a hand-built AST).
func TestModuleInterpreterAndBytecodeParity(t *testing.T) {
	howlframeBinary := buildHowlFrameBinaryForTest(t)

	runOut, err := exec.Command(howlframeBinary, "-run", "tests/module_main.howl").CombinedOutput()
	if err != nil {
		t.Fatalf("-run failed: %v\n%s", err, runOut)
	}

	bcFile := filepath.Join(t.TempDir(), "module_main.bc.bin")
	if output, err := exec.Command(howlframeBinary, "-compile-bc", "tests/module_main.howl", "-o", bcFile).CombinedOutput(); err != nil {
		t.Fatalf("-compile-bc failed: %v\n%s", err, output)
	}
	runBcOut, err := exec.Command(howlframeBinary, "-run-bc", bcFile).CombinedOutput()
	if err != nil {
		t.Fatalf("-run-bc failed: %v\n%s", err, runBcOut)
	}

	const want = "42\n20\n"
	if string(runOut) != want {
		t.Errorf("-run output = %q, want %q", runOut, want)
	}
	if string(runBcOut) != want {
		t.Errorf("-run-bc output = %q, want %q", runBcOut, want)
	}
}

// TestModuleBytecodeRunsWithoutSourceModule is the direct proof that module
// linking is compile-time, not an accidental runtime dependency on the
// original .howl source: compile to bytecode, then make the imported module
// file unavailable, then run the bytecode.
func TestModuleBytecodeRunsWithoutSourceModule(t *testing.T) {
	howlframeBinary := buildHowlFrameBinaryForTest(t)

	scratch := t.TempDir()
	mathSrc, err := os.ReadFile("tests/module_math.howl")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	mainSrc, err := os.ReadFile("tests/module_main.howl")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	mathPath := filepath.Join(scratch, "module_math.howl")
	mainPath := filepath.Join(scratch, "module_main.howl")
	if err := os.WriteFile(mathPath, mathSrc, 0o644); err != nil {
		t.Fatalf("failed to write module_math.howl: %v", err)
	}
	if err := os.WriteFile(mainPath, mainSrc, 0o644); err != nil {
		t.Fatalf("failed to write module_main.howl: %v", err)
	}

	bcFile := filepath.Join(scratch, "module_main.bc.bin")
	if output, err := exec.Command(howlframeBinary, "-compile-bc", mainPath, "-o", bcFile).CombinedOutput(); err != nil {
		t.Fatalf("-compile-bc failed: %v\n%s", err, output)
	}

	if err := os.Remove(mathPath); err != nil {
		t.Fatalf("failed to remove module source: %v", err)
	}

	output, err := exec.Command(howlframeBinary, "-run-bc", bcFile).CombinedOutput()
	if err != nil {
		t.Fatalf("-run-bc failed after the source module was removed: %v\n%s", err, output)
	}
	if string(output) != "42\n20\n" {
		t.Errorf("-run-bc output = %q, want %q", output, "42\n20\n")
	}
}

// TestModulePrivateSymbolIsNotReachable tests the actual namespace contract,
// not obscurity: calling a non-exported symbol through its module's alias
// must fail closed with a structured diagnostic, before execution.
func TestModulePrivateSymbolIsNotReachable(t *testing.T) {
	howlframeBinary := buildHowlFrameBinaryForTest(t)

	dir := t.TempDir()
	mustWriteFile(t, dir, "priv_math.howl", `(module
	(export (defun add_one (n)
		(type_hint n "int")
		(type_hint return "int")
		(return (+ n 1))
	))
	(defun hidden ()
		(type_hint return "int")
		(return 99)
	)
)`)
	mainPath := mustWriteFile(t, dir, "priv_main.howl", `(cli_app
	(use "priv_math.howl" as math)
	(print (call math/hidden))
)`)

	output, err := exec.Command(howlframeBinary, "-validate", mainPath).CombinedOutput()
	if err == nil {
		t.Fatalf("expected -validate to reject a call to a private symbol, got success:\n%s", output)
	}
	if !strings.Contains(string(output), "math/hidden") {
		t.Errorf("expected the diagnostic to name the unreachable symbol, got: %s", output)
	}

	bcFile := filepath.Join(dir, "priv_main.bc.bin")
	if output, err := exec.Command(howlframeBinary, "-compile-bc", mainPath, "-o", bcFile).CombinedOutput(); err == nil {
		t.Fatalf("expected -compile-bc to reject a call to a private symbol, got success:\n%s", output)
	}
	if _, statErr := os.Stat(bcFile); !os.IsNotExist(statErr) {
		t.Fatalf("no artifact may be written after a fail-closed rejection: %v", statErr)
	}
}

// TestModuleMissingImportFailsClosed proves a missing use target fails
// closed with source location, rather than a confusing downstream error.
func TestModuleMissingImportFailsClosed(t *testing.T) {
	howlframeBinary := buildHowlFrameBinaryForTest(t)

	dir := t.TempDir()
	mainPath := mustWriteFile(t, dir, "missing_main.howl", `(cli_app
	(use "does_not_exist.howl" as nope)
	(print (call nope/foo))
)`)

	output, err := exec.Command(howlframeBinary, "-validate", mainPath).CombinedOutput()
	if err == nil {
		t.Fatalf("expected -validate to reject a missing module, got success:\n%s", output)
	}
	var diag struct {
		Reason string `json:"reason"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	if jsonErr := json.Unmarshal(output, &diag); jsonErr != nil {
		t.Fatalf("expected a structured JSON diagnostic, got: %s", output)
	}
	if !strings.Contains(diag.Reason, "does_not_exist.howl") {
		t.Errorf("expected the diagnostic to name the missing module, got: %q", diag.Reason)
	}
	if diag.Line == 0 {
		t.Errorf("expected a nonzero source line in the diagnostic, got: %+v", diag)
	}
}

// TestModuleNestedImportFailsClosed proves that a module importing another
// module (main uses A, A uses B) is rejected with a dedicated diagnostic
// naming both files, rather than the confusing "undefined reference" symptom
// that mis-linking produced before this change. Real transitive module
// linking is deliberately deferred (see the #95 journal); this is the
// documented scope boundary, not a bug.
func TestModuleNestedImportFailsClosed(t *testing.T) {
	howlframeBinary := buildHowlFrameBinaryForTest(t)

	dir := t.TempDir()
	mustWriteFile(t, dir, "nest_b.howl", `(module
	(export (defun b_fn () (type_hint return "int") (return 5)))
)`)
	mustWriteFile(t, dir, "nest_a.howl", `(module
	(use "nest_b.howl" as b)
	(export (defun a_fn () (type_hint return "int") (return (call b/b_fn))))
)`)
	mainPath := mustWriteFile(t, dir, "nest_main.howl", `(cli_app
	(use "nest_a.howl" as a)
	(print (call a/a_fn))
)`)

	output, err := exec.Command(howlframeBinary, "-validate", mainPath).CombinedOutput()
	if err == nil {
		t.Fatalf("expected -validate to reject a nested module import, got success:\n%s", output)
	}
	for _, want := range []string{"nested module import", "nest_a.howl", "nest_b.howl"} {
		if !strings.Contains(string(output), want) {
			t.Errorf("expected %q in output, got: %s", want, output)
		}
	}
}

// TestModuleCircularImportFailsClosed proves a circular module dependency is
// rejected deterministically, and fast - not via the generic depth>100
// include guard. A cycle requires at least one module-to-module use, which
// TestModuleNestedImportFailsClosed's boundary already forbids outright, so
// no cycle can be constructed at all; this test locks that guarantee in.
func TestModuleCircularImportFailsClosed(t *testing.T) {
	howlframeBinary := buildHowlFrameBinaryForTest(t)

	dir := t.TempDir()
	mustWriteFile(t, dir, "circ_a.howl", `(module
	(use "circ_b.howl" as b)
	(export (defun a_fn (n) (type_hint n "int") (type_hint return "int") (return (call b/b_fn n))))
)`)
	mustWriteFile(t, dir, "circ_b.howl", `(module
	(use "circ_a.howl" as a)
	(export (defun b_fn (n) (type_hint n "int") (type_hint return "int") (return (call a/a_fn n))))
)`)
	mainPath := mustWriteFile(t, dir, "circ_main.howl", `(cli_app
	(use "circ_a.howl" as a)
	(print (call a/a_fn 1))
)`)

	done := make(chan struct{})
	var output []byte
	var cmdErr error
	go func() {
		output, cmdErr = exec.Command(howlframeBinary, "-validate", mainPath).CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("circular import was not rejected within 10s; the nested-use boundary may have regressed to depth-based recursion")
	}

	if cmdErr == nil {
		t.Fatalf("expected -validate to reject a circular module dependency, got success:\n%s", output)
	}
	if !strings.Contains(string(output), "nested module import") {
		t.Errorf("expected the circular case to hit the nested-import boundary, got: %s", output)
	}
}

// TestModuleHFIRProvenanceSurvivesResolution proves that after module
// resolution, HFIR nodes still carry source provenance pointing at the
// originating file: an imported function's nodes are attributed to the
// module file, and the importer's own statements to the importer file - with
// no HFIR code changes needed, since ast.Node.Filename is set once at parse
// time and ast.ResolveModules only ever rewrites .Value, never .Filename.
func TestModuleHFIRProvenanceSurvivesResolution(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, dir, "prov_math.howl", `(module
	(export (defun add_one (n)
		(type_hint n "int")
		(type_hint return "int")
		(return (+ n 1))
	))
)`)

	source := `(cli_app (use "prov_math.howl" as math) (print (call math/add_one 41)))`
	root := parser.NewParser(lexer.NewLexer(source), "prov_main.howl").ParseExpression()
	parser.ExpandIncludes(root, dir, 0)
	ast.ResolveModules(root)

	graph, err := hfir.LowerAST(root, "prov_main.howl")
	if err != nil {
		t.Fatalf("hfir.LowerAST failed: %v", err)
	}

	var sawImported, sawImporter bool
	for _, node := range graph.Nodes {
		switch node.Provenance.Filename {
		case "prov_math.howl":
			sawImported = true
		case "prov_main.howl":
			sawImporter = true
		}
	}
	if !sawImported {
		t.Error("expected at least one HFIR node with Filename \"prov_math.howl\" from the imported module")
	}
	if !sawImporter {
		t.Error("expected at least one HFIR node with Filename \"prov_main.howl\" from the importer")
	}
}

// mustWriteFile writes content to dir/name and returns the full path.
func mustWriteFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	return path
}
