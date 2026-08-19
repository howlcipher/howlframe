package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Target string

const (
	TargetBytecode    Target = "bytecode"
	TargetInterpreter Target = "interpreter"
	TargetGo          Target = "go"
	TargetJavaScript  Target = "javascript"
)

type Status string

const (
	StatusPass               Status = "PASS"
	StatusSemanticMismatch   Status = "SEMANTIC_MISMATCH"
	StatusBackendUnsupported Status = "BACKEND_UNSUPPORTED"
	StatusCompileFailure     Status = "COMPILE_FAILURE"
	StatusRuntimeFailure     Status = "RUNTIME_FAILURE"
)

type ExecutionResult struct {
	Target       Target `json:"target"`
	ExitCode     int    `json:"exit_code"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	ErrorClass   string `json:"error_class,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	Status       Status `json:"status"`
}

type ParityReport struct {
	FixturePath     string                     `json:"fixture_path"`
	CanonicalResult ExecutionResult            `json:"canonical_result"`
	TargetResults   map[Target]ExecutionResult `json:"target_results"`
	Discrepancies   []string                   `json:"discrepancies,omitempty"`
	OverallStatus   Status                     `json:"overall_status"`
}

var (
	compilerBinPath string
	compilerOnce    sync.Once
	compilerErr     error
)

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "howlframe.go")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not find howlframe.go in parent directories")
}

func getCompiler() (string, error) {
	compilerOnce.Do(func() {
		repoRoot, err := findRepoRoot()
		if err != nil {
			compilerErr = err
			return
		}
		tmpDir, err := os.MkdirTemp("", "howlframe-compiler-*")
		if err != nil {
			compilerErr = err
			return
		}
		binPath := filepath.Join(tmpDir, "howlframe")
		cmd := exec.Command("go", "build", "-o", binPath, filepath.Join(repoRoot, "howlframe.go"))
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			compilerErr = fmt.Errorf("build howlframe compiler failed: %v\n%s", err, string(out))
			return
		}
		compilerBinPath = binPath
	})
	return compilerBinPath, compilerErr
}

func runCmdWithBuffers(cmd *exec.Cmd, input string) (int, string, string, error) {
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return exitCode, stdoutBuf.String(), stderrBuf.String(), err
}

func NormalizeOutput(s string) string {
	raw := strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	var trimmed []string
	for _, l := range lines {
		trimmed = append(trimmed, strings.TrimRight(l, " \t"))
	}
	return strings.TrimSpace(strings.Join(trimmed, "\n"))
}

func NormalizeError(errStr string) string {
	if errStr == "" {
		return ""
	}
	s := strings.ToLower(errStr)
	switch {
	case strings.Contains(s, "undefined variable") || strings.Contains(s, "undefined_var") || strings.Contains(s, "undefined var") || strings.Contains(s, "undefined reference"):
		return "UNDEFINED_VARIABLE"
	case strings.Contains(s, "division by zero") || strings.Contains(s, "divide by zero") || strings.Contains(s, "div_zero"):
		return "DIVISION_BY_ZERO"
	case strings.Contains(s, "type") || strings.Contains(s, "expected number") || strings.Contains(s, "expected boolean") || strings.Contains(s, "cannot convert"):
		return "TYPE_ERROR"
	case strings.Contains(s, "cannot read file") || strings.Contains(s, "failed to read") || strings.Contains(s, "no such file") || strings.Contains(s, "io_error"):
		return "IO_ERROR"
	case strings.Contains(s, "limit") || strings.Contains(s, "limit_exceeded") || strings.Contains(s, "instruction limit"):
		return "LIMIT_EXCEEDED"
	case strings.Contains(s, "capability") || strings.Contains(s, "capability_denied"):
		return "CAPABILITY_DENIED"
	default:
		return "RUNTIME_ERROR"
	}
}

func ExecuteBytecode(filePath string, cliArgs []string, input string) ExecutionResult {
	compiler, err := getCompiler()
	if err != nil {
		return ExecutionResult{Target: TargetBytecode, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
	}

	tmpDir, err := os.MkdirTemp("", "howlframe-bc-*")
	if err != nil {
		return ExecutionResult{Target: TargetBytecode, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
	}
	defer os.RemoveAll(tmpDir)

	bcPath := filepath.Join(tmpDir, "app.hfbc")
	compileCmd := exec.Command(compiler, "-compile-bc", filePath, "-o", bcPath)
	compileOut, compileErr := compileCmd.CombinedOutput()
	if compileErr != nil {
		errMsg := strings.TrimSpace(string(compileOut))
		status := StatusCompileFailure
		if strings.Contains(errMsg, "not feasible") || strings.Contains(errMsg, "unsupported") {
			status = StatusBackendUnsupported
		}
		return ExecutionResult{
			Target:       TargetBytecode,
			ExitCode:     1,
			ErrorMessage: errMsg,
			ErrorClass:   NormalizeError(errMsg),
			Status:       status,
		}
	}

	runArgs := append([]string{"-run-bc", "-allow-caps", "network,filesystem,process,environment,database", bcPath}, cliArgs...)
	runCmd := exec.Command(compiler, runArgs...)
	runCmd.Env = append(os.Environ(), "HOWLFRAME_TEST_TOKEN=expected-secret")
	exitCode, stdout, stderr, runErr := runCmdWithBuffers(runCmd, input)

	res := ExecutionResult{
		Target:   TargetBytecode,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Status:   StatusPass,
	}
	if runErr != nil && exitCode != 0 {
		outAll := stderr + " " + stdout
		res.ErrorMessage = strings.TrimSpace(outAll)
		res.ErrorClass = NormalizeError(outAll)
		if strings.Contains(outAll, `"phase":"runtime"`) || strings.Contains(outAll, "panic:") || strings.Contains(outAll, "division by zero") || strings.Contains(outAll, "VMError") {
			res.Status = StatusRuntimeFailure
		} else if res.Stdout != "" || res.Stderr != "" {
			res.Status = StatusPass
		} else {
			res.Status = StatusRuntimeFailure
		}
	}
	return res
}

func ExecuteInterpreter(filePath string, cliArgs []string, input string) ExecutionResult {
	compiler, err := getCompiler()
	if err != nil {
		return ExecutionResult{Target: TargetInterpreter, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
	}

	runArgs := append([]string{"-run", filePath}, cliArgs...)
	runCmd := exec.Command(compiler, runArgs...)
	runCmd.Env = append(os.Environ(), "HOWLFRAME_TEST_TOKEN=expected-secret")
	exitCode, stdout, stderr, runErr := runCmdWithBuffers(runCmd, input)

	outCombined := stdout + stderr
	if strings.Contains(outCombined, "not supported under -run") || strings.Contains(outCombined, "-run only supports cli_app") {
		return ExecutionResult{
			Target:       TargetInterpreter,
			ExitCode:     exitCode,
			ErrorMessage: strings.TrimSpace(outCombined),
			ErrorClass:   "UNSUPPORTED",
			Status:       StatusBackendUnsupported,
		}
	}

	res := ExecutionResult{
		Target:   TargetInterpreter,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Status:   StatusPass,
	}

	if runErr != nil && exitCode != 0 {
		res.ErrorMessage = strings.TrimSpace(outCombined)
		res.ErrorClass = NormalizeError(outCombined)
		if strings.Contains(outCombined, "division by zero") || strings.Contains(outCombined, "panic:") {
			res.Status = StatusRuntimeFailure
		} else if strings.Contains(outCombined, `"reason"`) {
			res.Status = StatusCompileFailure
		} else if res.Stdout != "" || res.Stderr != "" {
			res.Status = StatusPass
		} else {
			res.Status = StatusRuntimeFailure
		}
	}
	return res
}

func ExecuteGoBackend(filePath string, cliArgs []string, input string) ExecutionResult {
	compiler, err := getCompiler()
	if err != nil {
		return ExecutionResult{Target: TargetGo, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
	}

	tmpDir, err := os.MkdirTemp("", "howlframe-go-*")
	if err != nil {
		return ExecutionResult{Target: TargetGo, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
	}
	defer os.RemoveAll(tmpDir)

	codegenCmd := exec.Command(compiler, filePath, "-o", tmpDir)
	codegenOut, codegenErr := codegenCmd.CombinedOutput()
	if codegenErr != nil {
		errMsg := strings.TrimSpace(string(codegenOut))
		return ExecutionResult{
			Target:       TargetGo,
			ExitCode:     1,
			ErrorMessage: errMsg,
			ErrorClass:   NormalizeError(errMsg),
			Status:       StatusCompileFailure,
		}
	}

	serverGoPath := filepath.Join(tmpDir, "server.go")
	binPath := filepath.Join(tmpDir, "server")
	buildCmd := exec.Command("go", "build", "-o", binPath, serverGoPath)
	buildOut, buildErr := buildCmd.CombinedOutput()
	if buildErr != nil {
		return ExecutionResult{
			Target:       TargetGo,
			ExitCode:     1,
			ErrorMessage: fmt.Sprintf("go build failed: %v\n%s", buildErr, string(buildOut)),
			ErrorClass:   "COMPILE_ERROR",
			Status:       StatusCompileFailure,
		}
	}

	runCmd := exec.Command(binPath, cliArgs...)
	runCmd.Dir = tmpDir
	runCmd.Env = append(os.Environ(), "HOWLFRAME_TEST_TOKEN=expected-secret")
	exitCode, stdout, stderr, runErr := runCmdWithBuffers(runCmd, input)

	res := ExecutionResult{
		Target:   TargetGo,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Status:   StatusPass,
	}
	if runErr != nil && exitCode != 0 {
		outAll := stderr + " " + stdout
		crashPath := filepath.Join(tmpDir, "crash.json")
		if crashData, err := os.ReadFile(crashPath); err == nil {
			res.ErrorMessage = string(crashData)
			res.ErrorClass = NormalizeError(string(crashData))
			res.Status = StatusRuntimeFailure
		} else {
			res.ErrorMessage = strings.TrimSpace(outAll)
			res.ErrorClass = NormalizeError(outAll)
			if strings.Contains(outAll, "panic:") || strings.Contains(outAll, "runtime error") {
				res.Status = StatusRuntimeFailure
			} else if res.Stdout != "" || res.Stderr != "" {
				res.Status = StatusPass
			} else {
				res.Status = StatusRuntimeFailure
			}
		}
	}
	return res
}

func ExecuteJSBackend(filePath string, cliArgs []string, input string) ExecutionResult {
	compiler, err := getCompiler()
	if err != nil {
		return ExecutionResult{Target: TargetJavaScript, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
	}

	if _, err := exec.LookPath("node"); err != nil {
		return ExecutionResult{
			Target:       TargetJavaScript,
			ExitCode:     0,
			ErrorMessage: "node runtime not available",
			Status:       StatusBackendUnsupported,
		}
	}

	tmpDir, err := os.MkdirTemp("", "howlframe-js-*")
	if err != nil {
		return ExecutionResult{Target: TargetJavaScript, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
	}
	defer os.RemoveAll(tmpDir)

	codegenCmd := exec.Command(compiler, filePath, "-o", tmpDir)
	codegenOut, codegenErr := codegenCmd.CombinedOutput()
	if codegenErr != nil {
		errMsg := strings.TrimSpace(string(codegenOut))
		return ExecutionResult{
			Target:       TargetJavaScript,
			ExitCode:     1,
			ErrorMessage: errMsg,
			ErrorClass:   NormalizeError(errMsg),
			Status:       StatusCompileFailure,
		}
	}

	jsPath := filepath.Join(tmpDir, "app.js")
	if _, err := os.Stat(jsPath); err != nil {
		return ExecutionResult{
			Target:       TargetJavaScript,
			ExitCode:     0,
			ErrorMessage: "not a web_app target (no app.js emitted)",
			Status:       StatusBackendUnsupported,
		}
	}

	runCmd := exec.Command("node", append([]string{jsPath}, cliArgs...)...)
	exitCode, stdout, stderr, runErr := runCmdWithBuffers(runCmd, input)

	res := ExecutionResult{
		Target:   TargetJavaScript,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Status:   StatusPass,
	}
	if runErr != nil {
		res.ErrorMessage = strings.TrimSpace(stderr)
		res.ErrorClass = NormalizeError(stderr)
		res.Status = StatusRuntimeFailure
	}
	return res
}

func VerifyParity(filePath string, cliArgs []string, input string, targets []Target) (ParityReport, error) {
	canonical := ExecuteBytecode(filePath, cliArgs, input)
	report := ParityReport{
		FixturePath:     filePath,
		CanonicalResult: canonical,
		TargetResults:   make(map[Target]ExecutionResult),
		OverallStatus:   StatusPass,
	}

	for _, tgt := range targets {
		var candidate ExecutionResult
		switch tgt {
		case TargetBytecode:
			candidate = canonical
		case TargetInterpreter:
			candidate = ExecuteInterpreter(filePath, cliArgs, input)
		case TargetGo:
			candidate = ExecuteGoBackend(filePath, cliArgs, input)
		case TargetJavaScript:
			candidate = ExecuteJSBackend(filePath, cliArgs, input)
		default:
			return report, fmt.Errorf("unsupported target: %s", tgt)
		}
		report.TargetResults[tgt] = candidate

		// If candidate is explicitly unsupported, don't fail parity
		if candidate.Status == StatusBackendUnsupported {
			continue
		}

		// If canonical was a compile failure, candidate must also fail compilation
		if canonical.Status == StatusCompileFailure {
			if candidate.Status != StatusCompileFailure {
				report.Discrepancies = append(report.Discrepancies, fmt.Sprintf("target %s succeeded compilation but canonical failed", tgt))
				report.OverallStatus = StatusSemanticMismatch
			}
			continue
		}

		// If canonical was a runtime failure, candidate must also fail runtime
		if canonical.Status == StatusRuntimeFailure {
			if candidate.Status != StatusRuntimeFailure {
				report.Discrepancies = append(report.Discrepancies, fmt.Sprintf("target %s succeeded but canonical failed runtime", tgt))
				report.OverallStatus = StatusSemanticMismatch
			} else if canonical.ErrorClass != "" && candidate.ErrorClass != "" && canonical.ErrorClass != candidate.ErrorClass {
				report.Discrepancies = append(report.Discrepancies, fmt.Sprintf("target %s error class mismatch: got %s, want %s", tgt, candidate.ErrorClass, canonical.ErrorClass))
				report.OverallStatus = StatusSemanticMismatch
			}
			continue
		}

		// Both must succeed and match stdout, stderr, and exit code
		if candidate.Status != StatusPass {
			report.Discrepancies = append(report.Discrepancies, fmt.Sprintf("target %s failed (%s: %s) but canonical passed", tgt, candidate.Status, candidate.ErrorMessage))
			report.OverallStatus = StatusSemanticMismatch
			continue
		}

		normCanonicalOut := NormalizeOutput(canonical.Stdout)
		normCandidateOut := NormalizeOutput(candidate.Stdout)
		if normCanonicalOut != normCandidateOut {
			report.Discrepancies = append(report.Discrepancies, fmt.Sprintf("target %s stdout mismatch:\n--- canonical ---\n%s\n--- target ---\n%s", tgt, normCanonicalOut, normCandidateOut))
			report.OverallStatus = StatusSemanticMismatch
		}

		normCanonicalErr := NormalizeOutput(canonical.Stderr)
		normCandidateErr := NormalizeOutput(candidate.Stderr)
		if normCanonicalErr != normCandidateErr {
			report.Discrepancies = append(report.Discrepancies, fmt.Sprintf("target %s stderr mismatch:\n--- canonical ---\n%s\n--- target ---\n%s", tgt, normCanonicalErr, normCandidateErr))
			report.OverallStatus = StatusSemanticMismatch
		}

		if canonical.ExitCode != candidate.ExitCode {
			report.Discrepancies = append(report.Discrepancies, fmt.Sprintf("target %s exit code mismatch: got %d, want %d", tgt, candidate.ExitCode, canonical.ExitCode))
			report.OverallStatus = StatusSemanticMismatch
		}
	}

	return report, nil
}
