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

const allKnownCaps = "network,filesystem,process,environment,database"

// RunOptions controls one differential execution.
// A nil AllowCaps keeps the historical grant of every known capability.
// DenyAll is an explicit empty grant. JSRoot, when set, rewrites the first
// cli_app root to that symbol before the JavaScript backend runs, so one
// cli_app fixture can be compared on the web_app host.
type RunOptions struct {
	CLIArgs   []string
	Input     string
	AllowCaps *string
	DenyAll   bool
	JSRoot    string
}

func (o RunOptions) capFlags() []string {
	if o.DenyAll {
		return nil
	}
	caps := allKnownCaps
	if o.AllowCaps != nil {
		caps = *o.AllowCaps
	}
	if caps == "" {
		return nil
	}
	return []string{"-allow-caps", caps}
}

// hostGrantValue is the runner grant for generated Go and JavaScript.
// Those hosts read HOWLFRAME_ALLOW_CAPS at the env call. An empty value
// denies. Nil AllowCaps keeps the historical grant of every known capability.
func (o RunOptions) hostGrantValue() string {
	if o.DenyAll {
		return ""
	}
	if o.AllowCaps != nil {
		return *o.AllowCaps
	}
	return allKnownCaps
}

// commandEnv returns the process environment with overrides replacing any
// inherited values of the same name. A later append would lose to the
// inherited key, and an inherited HOWLFRAME_ALLOW_CAPS would widen a denial.
func commandEnv(overrides ...string) []string {
	replaced := make(map[string]string, len(overrides))
	order := make([]string, 0, len(overrides))
	for _, ov := range overrides {
		key, val, ok := strings.Cut(ov, "=")
		if !ok {
			continue
		}
		if _, seen := replaced[key]; !seen {
			order = append(order, key)
		}
		replaced[key] = val
	}
	out := make([]string, 0, len(os.Environ())+len(order))
	for _, entry := range os.Environ() {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, drop := replaced[key]; drop {
				continue
			}
		}
		out = append(out, entry)
	}
	for _, key := range order {
		out = append(out, key+"="+replaced[key])
	}
	return out
}

// isStructuredRuntimeRejection reports runtime failures whose text is a
// shared error class. Checker diagnostics also use a "reason" field, so
// the interpreter path must not treat every JSON reason as a compile failure.
func isStructuredRuntimeRejection(out string) bool {
	s := strings.ToLower(out)
	if strings.Contains(s, "type_error:") || strings.Contains(s, `"code":"type_error"`) {
		return true
	}
	if strings.Contains(s, "capability denied") || strings.Contains(s, `"code":"capability_denied"`) {
		return true
	}
	if strings.Contains(s, "division by zero") || strings.Contains(s, "divide by zero") {
		return true
	}
	return false
}

func ExecuteBytecode(filePath string, cliArgs []string, input string) ExecutionResult {
	return executeBytecode(filePath, RunOptions{CLIArgs: cliArgs, Input: input})
}

func executeBytecode(filePath string, opts RunOptions) ExecutionResult {
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

	runArgs := append([]string{"-run-bc"}, opts.capFlags()...)
	runArgs = append(runArgs, bcPath)
	runArgs = append(runArgs, opts.CLIArgs...)
	runCmd := exec.Command(compiler, runArgs...)
	runCmd.Env = append(os.Environ(), "HOWLFRAME_TEST_TOKEN=expected-secret")
	exitCode, stdout, stderr, runErr := runCmdWithBuffers(runCmd, opts.Input)

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
		if strings.Contains(outAll, `"phase":"runtime"`) || strings.Contains(outAll, "panic:") || strings.Contains(outAll, "division by zero") || strings.Contains(outAll, "VMError") || isStructuredRuntimeRejection(outAll) {
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
	return executeInterpreter(filePath, RunOptions{CLIArgs: cliArgs, Input: input})
}

func executeInterpreter(filePath string, opts RunOptions) ExecutionResult {
	compiler, err := getCompiler()
	if err != nil {
		return ExecutionResult{Target: TargetInterpreter, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
	}

	runArgs := append([]string{"-run"}, opts.capFlags()...)
	runArgs = append(runArgs, filePath)
	runArgs = append(runArgs, opts.CLIArgs...)
	runCmd := exec.Command(compiler, runArgs...)
	runCmd.Env = append(os.Environ(), "HOWLFRAME_TEST_TOKEN=expected-secret")
	exitCode, stdout, stderr, runErr := runCmdWithBuffers(runCmd, opts.Input)

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
		if isStructuredRuntimeRejection(outCombined) || strings.Contains(outCombined, "panic:") {
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
	return executeGoBackend(filePath, RunOptions{CLIArgs: cliArgs, Input: input})
}

func executeGoBackend(filePath string, opts RunOptions) ExecutionResult {
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

	runCmd := exec.Command(binPath, opts.CLIArgs...)
	runCmd.Dir = tmpDir
	runCmd.Env = commandEnv(
		"HOWLFRAME_TEST_TOKEN=expected-secret",
		"HOWLFRAME_ALLOW_CAPS="+opts.hostGrantValue(),
	)
	exitCode, stdout, stderr, runErr := runCmdWithBuffers(runCmd, opts.Input)

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
			if strings.Contains(outAll, "panic:") || strings.Contains(outAll, "runtime error") || isStructuredRuntimeRejection(outAll) {
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
	return executeJSBackend(filePath, RunOptions{CLIArgs: cliArgs, Input: input})
}

func executeJSBackend(filePath string, opts RunOptions) ExecutionResult {
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

	sourcePath := filePath
	if opts.JSRoot != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return ExecutionResult{Target: TargetJavaScript, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
		}
		rewritten := strings.Replace(string(data), "(cli_app", "("+opts.JSRoot, 1)
		if rewritten == string(data) {
			return ExecutionResult{
				Target:       TargetJavaScript,
				ExitCode:     1,
				ErrorMessage: "fixture has no cli_app root to rewrite",
				Status:       StatusCompileFailure,
			}
		}
		sourcePath = filepath.Join(tmpDir, "app.howl")
		if err := os.WriteFile(sourcePath, []byte(rewritten), 0o644); err != nil {
			return ExecutionResult{Target: TargetJavaScript, ExitCode: 1, ErrorMessage: err.Error(), Status: StatusCompileFailure}
		}
	}

	codegenCmd := exec.Command(compiler, sourcePath, "-o", tmpDir)
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

	runCmd := exec.Command("node", append([]string{jsPath}, opts.CLIArgs...)...)
	runCmd.Env = commandEnv("HOWLFRAME_ALLOW_CAPS=" + opts.hostGrantValue())
	exitCode, stdout, stderr, runErr := runCmdWithBuffers(runCmd, opts.Input)

	res := ExecutionResult{
		Target:   TargetJavaScript,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Status:   StatusPass,
	}
	if runErr != nil {
		outAll := stderr + " " + stdout
		res.ErrorMessage = strings.TrimSpace(outAll)
		res.ErrorClass = NormalizeError(outAll)
		res.Status = StatusRuntimeFailure
	}
	return res
}

// compareToCanonical returns the parity discrepancies between one target and
// the bytecode canonical result. An unsupported target has no discrepancies.
func compareToCanonical(canonical, candidate ExecutionResult, tgt Target) []string {
	if candidate.Status == StatusBackendUnsupported {
		return nil
	}

	var discrepancies []string
	if canonical.Status == StatusCompileFailure {
		if candidate.Status != StatusCompileFailure {
			discrepancies = append(discrepancies, fmt.Sprintf("target %s succeeded compilation but canonical failed", tgt))
		}
		return discrepancies
	}

	if canonical.Status == StatusRuntimeFailure {
		if candidate.Status != StatusRuntimeFailure {
			discrepancies = append(discrepancies, fmt.Sprintf("target %s succeeded but canonical failed runtime", tgt))
		} else if canonical.ErrorClass != "" && candidate.ErrorClass != "" && canonical.ErrorClass != candidate.ErrorClass {
			discrepancies = append(discrepancies, fmt.Sprintf("target %s error class mismatch: got %s, want %s", tgt, candidate.ErrorClass, canonical.ErrorClass))
		}
		return discrepancies
	}

	if candidate.Status != StatusPass {
		return []string{fmt.Sprintf("target %s failed (%s: %s) but canonical passed", tgt, candidate.Status, candidate.ErrorMessage)}
	}

	normCanonicalOut := NormalizeOutput(canonical.Stdout)
	normCandidateOut := NormalizeOutput(candidate.Stdout)
	if normCanonicalOut != normCandidateOut {
		discrepancies = append(discrepancies, fmt.Sprintf("target %s stdout mismatch:\n--- canonical ---\n%s\n--- target ---\n%s", tgt, normCanonicalOut, normCandidateOut))
	}

	normCanonicalErr := NormalizeOutput(canonical.Stderr)
	normCandidateErr := NormalizeOutput(candidate.Stderr)
	if normCanonicalErr != normCandidateErr {
		discrepancies = append(discrepancies, fmt.Sprintf("target %s stderr mismatch:\n--- canonical ---\n%s\n--- target ---\n%s", tgt, normCanonicalErr, normCandidateErr))
	}

	if canonical.ExitCode != candidate.ExitCode {
		discrepancies = append(discrepancies, fmt.Sprintf("target %s exit code mismatch: got %d, want %d", tgt, candidate.ExitCode, canonical.ExitCode))
	}
	return discrepancies
}

func VerifyParity(filePath string, cliArgs []string, input string, targets []Target) (ParityReport, error) {
	return VerifyParityWithOptions(filePath, targets, RunOptions{CLIArgs: cliArgs, Input: input})
}

func VerifyParityWithOptions(filePath string, targets []Target, opts RunOptions) (ParityReport, error) {
	canonical := executeBytecode(filePath, opts)
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
			candidate = executeInterpreter(filePath, opts)
		case TargetGo:
			candidate = executeGoBackend(filePath, opts)
		case TargetJavaScript:
			candidate = executeJSBackend(filePath, opts)
		default:
			return report, fmt.Errorf("unsupported target: %s", tgt)
		}
		report.TargetResults[tgt] = candidate
		discrepancies := compareToCanonical(canonical, candidate, tgt)
		if len(discrepancies) > 0 {
			report.Discrepancies = append(report.Discrepancies, discrepancies...)
			report.OverallStatus = StatusSemanticMismatch
		}
	}

	return report, nil
}
