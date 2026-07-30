package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
	if err := os.WriteFile(inputFile, []byte(`(cli_app (call panic "test crash dump"))`), 0644); err != nil {
		t.Fatalf("Failed to write input file: %v", err)
	}

	cmd = exec.Command("./zero", "-o", outDir, inputFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to transpilation: %v", err)
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
