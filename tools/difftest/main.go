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

	files, err := filepath.Glob("tests/*.howl")
	if err != nil {
		log.Fatalf("failed to glob tests: %v", err)
	}

	var passed int
	var skipped int
	var failed int

	for _, file := range files {
		base := filepath.Base(file)
		if reason, ok := exempt[base]; ok {
			if reason == "unsupported in run" {
				pass, _, _ := testFixture(file)
				if pass {
					fmt.Printf("FAIL %s: stale exemption %q, fixture now passes differential testing\n", base, reason)
					failed++
					continue
				}
			}
			fmt.Printf("SKIP %s: %s\n", base, reason)
			skipped++
			continue
		}

		pass, skipReason, failMsg := testFixture(file)
		if skipReason != "" {
			fmt.Printf("SKIP %s: %s\n", base, skipReason)
			skipped++
			continue
		}

		fmt.Printf("RUN  %s...\n", base)
		if !pass {
			fmt.Printf("FAIL %s %s\n", base, failMsg)
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

func testFixture(file string) (passed bool, skipReason string, failMsg string) {
	content, err := os.ReadFile(file)
	if err != nil {
		return false, "", fmt.Sprintf("failed to read %s: %v", file, err)
	}
	if !strings.Contains(string(content), "(cli_app") {
		return false, "not a cli_app", ""
	}

	out1, err := runCommand("go", "run", "howlframe.go", "-run", file)
	if err != nil {
		return false, "", fmt.Sprintf("(interpreter): %v\n%s", err, out1)
	}

	outDir, err := os.MkdirTemp("", "difftest-*")
	if err != nil {
		return false, "", fmt.Sprintf("failed to make temp dir: %v", err)
	}
	defer os.RemoveAll(outDir)

	out2, err := runCommand("go", "run", "howlframe.go", "-o", outDir, file)
	if err != nil {
		return false, "", fmt.Sprintf("(codegen): %v\n%s", err, out2)
	}

	binaryPath := filepath.Join(outDir, "server")
	serverGoPath := filepath.Join(outDir, "server.go")
	out3, err := runCommand("go", "build", "-o", binaryPath, serverGoPath)
	if err != nil {
		return false, "", fmt.Sprintf("(go build): %v\n%s", err, out3)
	}

	out4, err := runCommand(binaryPath)
	if err != nil {
		return false, "", fmt.Sprintf("(go binary run): %v\n%s", err, out4)
	}

	bcPath := filepath.Join(outDir, "test.bc")
	out5, err := runCommand("go", "run", "howlframe.go", "-compile-bc", file, "-o", bcPath)
	if err != nil {
		return false, "", fmt.Sprintf("(compile-bc): %v\n%s", err, out5)
	}

	out6, err := runCommand("go", "run", "howlframe.go", "-run-bc", "-allow-caps", "network,filesystem,process,environment,database", bcPath)
	if err != nil {
		return false, "", fmt.Sprintf("(run-bc): %v\n%s", err, out6)
	}

	if out1 != out4 || out1 != out6 {
		msg := fmt.Sprintf("stdout mismatch\n--- interpreter ---\n%s\n--- go backend  ---\n%s\n--- bytecode    ---\n%s", out1, out4, out6)
		return false, "", msg
	}

	return true, "", ""
}

func runCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	// Inherit env, but inject HOWLFRAME_TEST_TOKEN for tests that need it
	cmd.Env = append(os.Environ(), "HOWLFRAME_TEST_TOKEN=expected-secret")
	out, err := cmd.CombinedOutput()
	return string(out), err
}
