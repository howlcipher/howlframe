package repo_analyst_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixtureFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
}

func runRepoAnalyst(t *testing.T, compiler, artifact string, args ...string) (string, error) {
	return runRepoAnalystWithBudget(t, compiler, artifact, 0, args...)
}

func runRepoAnalystWithBudget(t *testing.T, compiler, artifact string, maxInstructions int, args ...string) (string, error) {
	t.Helper()
	commandArgs := []string{
		"-run-bc",
		"-allow-caps", "process,filesystem",
	}
	if maxInstructions > 0 {
		commandArgs = append(commandArgs, "--max-instructions", fmt.Sprint(maxInstructions))
	}
	commandArgs = append(commandArgs, artifact)
	commandArgs = append(commandArgs, args...)
	command := exec.Command(compiler, commandArgs...)
	output, err := command.CombinedOutput()
	return string(output), err
}

func requireExitCode(t *testing.T, err error, want int) {
	t.Helper()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("error = %v, want process exit %d", err, want)
	}
	if exitError.ExitCode() != want {
		t.Fatalf("exit code = %d, want %d", exitError.ExitCode(), want)
	}
}

func TestRepoAnalystStandaloneBytecode(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	scratchDir := t.TempDir()
	compiler := filepath.Join(scratchDir, "howlframe")
	artifact := filepath.Join(scratchDir, "repo_analyst.hfbc")
	applicationSourceDir := filepath.Join(scratchDir, "repo_analyst_source")
	for _, name := range []string{
		"repo_analyst.howl",
		"discovery.howl",
		"classification.howl",
		"text_analysis.howl",
		"report.howl",
	} {
		sourcePath := filepath.Join(repositoryRoot, "examples", "repo_analyst", name)
		source, readErr := os.ReadFile(sourcePath)
		if readErr != nil {
			t.Fatalf("read application source %s: %v", name, readErr)
		}
		writeFixtureFile(t, filepath.Join(applicationSourceDir, name), source)
	}
	applicationSource := filepath.Join(applicationSourceDir, "repo_analyst.howl")

	build := exec.Command("go", "build", "-o", compiler, "howlframe.go")
	build.Dir = repositoryRoot
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build HowlFrame scratch binary: %v\n%s", buildErr, output)
	}

	compile := exec.Command(compiler, "-compile-bc", applicationSource, "-o", artifact)
	if output, compileErr := compile.CombinedOutput(); compileErr != nil {
		t.Fatalf("compile Repo Analyst to bytecode: %v\n%s", compileErr, output)
	}
	if removeErr := os.RemoveAll(applicationSourceDir); removeErr != nil {
		t.Fatalf("remove compiled application sources: %v", removeErr)
	}

	fixture := filepath.Join(scratchDir, "sample_repo")
	mainContent := []byte(strings.Repeat("reference application content ", 12) + "TODO first TODO second\n")
	mainPath := filepath.Join(fixture, "main.howl")
	writeFixtureFile(t, mainPath, mainContent)
	writeFixtureFile(t, filepath.Join(fixture, "pkg", "worker.go"), []byte("package worker\n// FIXME: improve\n"))
	writeFixtureFile(t, filepath.Join(fixture, "pkg", "worker_test.go"), []byte("package worker\n"))
	writeFixtureFile(t, filepath.Join(fixture, "tools", "script.py"), []byte("print('ok')\n"))
	writeFixtureFile(t, filepath.Join(fixture, "web", "ui.js"), []byte("export const ui = true;\n"))
	writeFixtureFile(t, filepath.Join(fixture, "src", "index.ts"), []byte("export const main = true;\n"))
	writeFixtureFile(t, filepath.Join(fixture, "config.yaml"), []byte("name: sample\n"))
	writeFixtureFile(t, filepath.Join(fixture, "asset.bin"), []byte{0x00, 0xff})

	wantReport := fmt.Sprintf(`howlframe.repo_analyst/v1
repository_path=%s
total_files=8
howlframe_files=1
go_files=2
python_files=1
javascript_files=1
typescript_files=1
configuration_files=1
test_files=1
likely_entry_points=2
text_files=7
unreadable_text_files=0
todo_markers=2
fixme_markers=1
largest_text_file=%s
largest_text_file_bytes=%d`, fixture, mainPath, len(mainContent))

	firstOutput, runErr := runRepoAnalyst(t, compiler, artifact, fixture)
	if runErr != nil {
		t.Fatalf("run Repo Analyst: %v\n%s", runErr, firstOutput)
	}
	if firstOutput != wantReport+"\n" {
		t.Fatalf("stdout mismatch\ngot:\n%s\nwant:\n%s", firstOutput, wantReport+"\n")
	}

	secondOutput, runErr := runRepoAnalyst(t, compiler, artifact, fixture)
	if runErr != nil {
		t.Fatalf("repeat Repo Analyst run: %v\n%s", runErr, secondOutput)
	}
	if secondOutput != firstOutput {
		t.Fatalf("repeated run was not deterministic\nfirst:\n%s\nsecond:\n%s", firstOutput, secondOutput)
	}

	budgetFixture := filepath.Join(scratchDir, "instruction_budget_repo")
	largeContent := []byte(strings.Repeat("x", 20000))
	largePath := filepath.Join(budgetFixture, "large.txt")
	writeFixtureFile(t, largePath, largeContent)
	defaultBudgetOutput, defaultBudgetErr := runRepoAnalyst(t, compiler, artifact, budgetFixture)
	requireExitCode(t, defaultBudgetErr, 1)
	if !strings.Contains(defaultBudgetOutput, `"code":"LIMIT_EXCEEDED"`) ||
		!strings.Contains(defaultBudgetOutput, `"message":"instruction limit exceeded"`) {
		t.Fatalf("default instruction-budget failure = %q", defaultBudgetOutput)
	}

	wantLargeReport := fmt.Sprintf(`howlframe.repo_analyst/v1
repository_path=%s
total_files=1
howlframe_files=0
go_files=0
python_files=0
javascript_files=0
typescript_files=0
configuration_files=0
test_files=0
likely_entry_points=0
text_files=1
unreadable_text_files=0
todo_markers=0
fixme_markers=0
largest_text_file=%s
largest_text_file_bytes=%d`, budgetFixture, largePath, len(largeContent))
	largeBudgetOutput, largeBudgetErr := runRepoAnalystWithBudget(t, compiler, artifact, 1000000, budgetFixture)
	if largeBudgetErr != nil {
		t.Fatalf("run Repo Analyst with explicit instruction budget: %v\n%s", largeBudgetErr, largeBudgetOutput)
	}
	if largeBudgetOutput != wantLargeReport+"\n" {
		t.Fatalf("large-budget report mismatch\ngot:\n%s\nwant:\n%s", largeBudgetOutput, wantLargeReport+"\n")
	}

	reportPath := filepath.Join(scratchDir, "report.txt")
	fileOutput, runErr := runRepoAnalyst(t, compiler, artifact, fixture, reportPath)
	if runErr != nil {
		t.Fatalf("write Repo Analyst report: %v\n%s", runErr, fileOutput)
	}
	if fileOutput != "" {
		t.Fatalf("file-output run wrote stdout %q", fileOutput)
	}
	writtenReport, readErr := os.ReadFile(reportPath)
	if readErr != nil {
		t.Fatalf("read written report: %v", readErr)
	}
	if string(writtenReport) != wantReport {
		t.Fatalf("written report mismatch\ngot:\n%s\nwant:\n%s", writtenReport, wantReport)
	}

	tiedPath := filepath.Join(fixture, "docs", "same-size.md")
	writeFixtureFile(t, tiedPath, []byte(strings.Repeat("x", len(mainContent))))
	tiedReport := strings.Replace(wantReport, "total_files=8", "total_files=9", 1)
	tiedReport = strings.Replace(tiedReport, "text_files=7", "text_files=8", 1)
	tiedReport = strings.Replace(tiedReport, "largest_text_file="+mainPath, "largest_text_file=", 1)
	tiedOutput, runErr := runRepoAnalyst(t, compiler, artifact, fixture)
	if runErr != nil {
		t.Fatalf("run Repo Analyst with tied largest files: %v\n%s", runErr, tiedOutput)
	}
	if tiedOutput != tiedReport+"\n" {
		t.Fatalf("tied-largest report mismatch\ngot:\n%s\nwant:\n%s", tiedOutput, tiedReport+"\n")
	}

	usageOutput, usageErr := runRepoAnalyst(t, compiler, artifact)
	requireExitCode(t, usageErr, 2)
	if usageOutput != "usage: repo_analyst <repository-path> [output-file]" {
		t.Fatalf("usage output = %q", usageOutput)
	}

	missingPath := filepath.Join(fixture, "missing")
	discoveryOutput, discoveryErr := runRepoAnalyst(t, compiler, artifact, missingPath)
	requireExitCode(t, discoveryErr, 3)
	if !strings.HasPrefix(discoveryOutput, "repo_analyst: repository discovery failed:") {
		t.Fatalf("discovery error = %q", discoveryOutput)
	}

	denied := exec.Command(compiler, "-run-bc", "--max-instructions", "1000000", artifact, fixture)
	deniedOutput, deniedErr := denied.CombinedOutput()
	requireExitCode(t, deniedErr, 3)
	if !strings.Contains(string(deniedOutput), `"code":"CAPABILITY_DENIED"`) ||
		!strings.Contains(string(deniedOutput), `"opcode":"EXEC"`) {
		t.Fatalf("capability denial = %q", deniedOutput)
	}
}
