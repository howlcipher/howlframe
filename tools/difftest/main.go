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

	out2, err := runCommand("go", "run", "howlframe.go", "build", "--target=go", "-o", outDir, file)
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

	out6, err := runCommand("go", "run", "howlframe.go", "-run-bc", "-allow-caps", "network,filesystem,process,process:exec=/usr/bin/rm **,environment,database", bcPath)
	if err != nil {
		return false, "", fmt.Sprintf("(run-bc): %v\n%s", err, out6)
	}

	// #159: the experimental HFIR bytecode path is a third leg; any
	// divergence from the interpreter fails the run.
	hfirPath := filepath.Join(outDir, "test.hfir.bc")
	// #161: only a construct on the explicit HFIR fence list (the locked
	// promote-blocker fence, see TestProdFlipCriteriaLock) may skip the HFIR
	// leg. Any other Phase-1 subset refusal is a failure, and a fenced fixture
	// is reported as SKIP, never counted as Passed.
	out8, hfirFenced := "", ""
	out7, err := runCommand("go", "run", "howlframe.go", "-compile-hfir-bc", file, "-o", hfirPath)
	if err != nil {
		hfirFenced = hfirFencedConstruct(out7)
		if hfirFenced == "" {
			return false, "", fmt.Sprintf("(compile-hfir-bc): %v\n%s", err, out7)
		}
	}
	if hfirFenced == "" {
		out8, err = runCommand("go", "run", "howlframe.go", "-run-bc", "-allow-caps", "network,filesystem,process,process:exec=/usr/bin/rm **,environment,database", hfirPath)
		if err != nil {
			return false, "", fmt.Sprintf("(run-bc hfir): %v\n%s", err, out8)
		}
	}

	if out1 != out4 || out1 != out6 || (hfirFenced == "" && out1 != out8) {
		msg := fmt.Sprintf("stdout mismatch\n--- interpreter ---\n%s\n--- go backend  ---\n%s\n--- bytecode    ---\n%s\n--- hfir bytecode ---\n%s", out1, out4, out6, out8)
		return false, "", msg
	}
	if hfirFenced != "" {
		return false, fmt.Sprintf("HFIR leg fenced (%s); interpreter, go and bytecode legs agree", hfirFenced), ""
	}

	return true, "", ""
}

func runCommand(name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	// HOWLFRAME_TEST_TOKEN feeds tests/test_env.howl. HOWLFRAME_ALLOW_CAPS is
	// the generated Go and JavaScript grant, matching the all-caps list this
	// runner already passes to -run-bc. The interpreter still takes -allow-caps.
	cmd.Env = commandEnv(
		"HOWLFRAME_TEST_TOKEN=expected-secret",
		"HOWLFRAME_ALLOW_CAPS="+allKnownCaps,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// hfirFencedConstructs is the explicit list of constructs whose HFIR lowering
// refusal skips the HFIR leg. It must not grow to hide unsupported constructs.
var hfirFencedConstructs = []string{"optimize_signature"}

// hfirFencedConstruct returns the fenced construct named by an HFIR lowering
// refusal, or "" when the refusal is not for a construct on the list.
func hfirFencedConstruct(out string) string {
	for _, kind := range hfirFencedConstructs {
		if strings.Contains(out, `node kind \"`+kind+`\" is not in the Phase-1 executable subset`) ||
			strings.Contains(out, `node kind "`+kind+`" is not in the Phase-1 executable subset`) {
			return kind
		}
	}
	return ""
}
