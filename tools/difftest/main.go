package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	manifestData, err := os.ReadFile("tools/difftest/manifest.json")
	if err != nil {
		log.Fatalf("failed to read manifest: %v", err)
	}

	var exempt map[string]string
	if err := json.Unmarshal(manifestData, &exempt); err != nil {
		log.Fatalf("failed to parse manifest: %v", err)
	}

	files, err := filepath.Glob("tests/*.zero")
	if err != nil {
		log.Fatalf("failed to glob tests: %v", err)
	}

	var passed int
	var skipped int
	var failed int

	for _, file := range files {
		base := filepath.Base(file)
		if reason, ok := exempt[base]; ok {
			fmt.Printf("SKIP %s: %s\n", base, reason)
			skipped++
			continue
		}

		// Also try to sniff root form.
		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("failed to read %s: %v", file, err)
		}
		if !strings.Contains(string(content), "(cli_app") {
			fmt.Printf("SKIP %s: not a cli_app\n", base)
			skipped++
			continue
		}

		fmt.Printf("RUN  %s...\n", base)

		out1, err := runCommand("go", "run", "zero.go", "-run", file)
		if err != nil {
			fmt.Printf("FAIL %s (interpreter): %v\n%s\n", base, err, out1)
			failed++
			continue
		}

		outDir, err := os.MkdirTemp("", "difftest-*")
		if err != nil {
			log.Fatalf("failed to make temp dir: %v", err)
		}

		out2, err := runCommand("go", "run", "zero.go", "-o", outDir, file)
		if err != nil {
			fmt.Printf("FAIL %s (codegen): %v\n%s\n", base, err, out2)
			failed++
			continue
		}

		// Zero transpiler output directory name sets the resulting package/binary name.
		// `filepath.Base` of the input file without `.zero` is used if `-o` is not provided,
		// but with `-o` it writes `server.go` inside. Let's just build `server.go`.
		binaryPath := filepath.Join(outDir, "server")
		serverGoPath := filepath.Join(outDir, "server.go")
		out3, err := runCommand("go", "build", "-o", binaryPath, serverGoPath)
		if err != nil {
			fmt.Printf("FAIL %s (go build): %v\n%s\n", base, err, out3)
			failed++
			continue
		}

		out4, err := runCommand(binaryPath)
		if err != nil {
			fmt.Printf("FAIL %s (go binary run): %v\n%s\n", base, err, out4)
			failed++
			continue
		}

		bcPath := filepath.Join(outDir, "test.bc")
		out5, err := runCommand("go", "run", "zero.go", "-compile-bc", file, "-o", bcPath)
		if err != nil {
			fmt.Printf("FAIL %s (compile-bc): %v\n%s\n", base, err, out5)
			failed++
			continue
		}

		out6, err := runCommand("go", "run", "zero.go", "-run-bc", bcPath)
		if err != nil {
			fmt.Printf("FAIL %s (run-bc): %v\n%s\n", base, err, out6)
			failed++
			continue
		}

		os.RemoveAll(outDir)

		if out1 != out4 || out1 != out6 {
			fmt.Printf("FAIL %s: stdout mismatch\n", base)
			fmt.Printf("--- interpreter ---\n%s\n", out1)
			fmt.Printf("--- go backend  ---\n%s\n", out4)
			fmt.Printf("--- bytecode    ---\n%s\n", out6)
			failed++
			continue
		}

		passed++
	}

	fmt.Printf("\nDone. Passed: %d, Skipped: %d, Failed: %d\n", passed, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func runCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	// Inherit env, but inject ZERO_TEST_TOKEN for tests that need it
	cmd.Env = append(os.Environ(), "ZERO_TEST_TOKEN=expected-secret")
	out, err := cmd.CombinedOutput()
	return string(out), err
}
