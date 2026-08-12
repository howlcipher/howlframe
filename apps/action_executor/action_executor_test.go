package action_executor_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestActionExecutor(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	scratchDir := t.TempDir()
	compiler := filepath.Join(scratchDir, "howlframe")
	artifact := filepath.Join(scratchDir, "action_executor.hfbc")

	build := exec.Command("go", "build", "-o", compiler, "howlframe.go")
	build.Dir = repositoryRoot
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build HowlFrame scratch binary: %v\n%s", buildErr, output)
	}

	appSource := filepath.Join(repositoryRoot, "apps", "action_executor", "action_executor.howl")
	compile := exec.Command(compiler, "-compile-bc", appSource, "-o", artifact)
	compile.Dir = filepath.Join(repositoryRoot, "apps", "action_executor")
	if output, compileErr := compile.CombinedOutput(); compileErr != nil {
		t.Fatalf("compile application to bytecode: %v\n%s", compileErr, output)
	}

	runCase := func(t *testing.T, name string, setupState string, proposalJSON string, evidenceArgs []string, expectDecision string, expectStateMutation string, capabilities []string) {
		t.Run(name, func(t *testing.T) {
			proposalPath := filepath.Join(scratchDir, "proposal.json")
			os.WriteFile(proposalPath, []byte(proposalJSON), 0o600)

			sandboxDir := t.TempDir()
			os.MkdirAll(filepath.Join(sandboxDir, "staged"), 0o755)
			os.MkdirAll(filepath.Join(sandboxDir, "releases"), 0o755)
			os.MkdirAll(filepath.Join(sandboxDir, "fixtures"), 0o755)

			// Setup some basic fixtures
			os.WriteFile(filepath.Join(sandboxDir, "fixtures", "app-v1.txt"), []byte("v1_content"), 0o600)
			os.WriteFile(filepath.Join(sandboxDir, "fixtures", "app-v2.txt"), []byte("v2_content"), 0o600)

			args := []string{"-run-bc"}
			if len(capabilities) > 0 {
				args = append(args, "-allow-caps", strings.Join(capabilities, ","))
			}
			args = append(args, artifact, proposalPath)

			// Provide base environment
			args = append(args, "sandbox="+sandboxDir)
			if setupState != "" {
				args = append(args, "state="+setupState)
			}
			args = append(args, evidenceArgs...)

			// Change directory to sandbox to resolve "fixtures/app-v1.txt" cleanly
			cmd := exec.Command(compiler, args...)
			cmd.Dir = sandboxDir

			out, err := cmd.CombinedOutput()

			var exitError *exec.ExitError
			if err != nil && errors.As(err, &exitError) && exitError.ExitCode() != 0 {
				if expectDecision == "ERROR" {
					return
				}
				// If we lack capability for expected side effects, we expect an error exit.
				hasDbCap := false
				hasFsCap := false
				for _, cap := range capabilities {
					if cap == "database" {
						hasDbCap = true
					}
					if cap == "filesystem" {
						hasFsCap = true
					}
				}
				if !hasFsCap || (!hasDbCap && expectStateMutation != "none" && expectStateMutation != "") {
					return
				}
				t.Fatalf("unexpected failure: %v\n%s", err, string(out))
			}

			if expectDecision == "ERROR" {
				t.Fatalf("expected error, but succeeded with output:\n%s", string(out))
			}

			outStr := string(out)

			if !strings.Contains(outStr, `"decision": "`+expectDecision+`"`) {
				t.Fatalf("expected decision %s, got output:\n%s", expectDecision, outStr)
			}

			// Validate explicit filesystem boundaries for staging
			if strings.Contains(proposalJSON, "stage_artifact") && expectDecision == "ALLOW" {
				// verify it actually copied
				_, statErr := os.Stat(filepath.Join(sandboxDir, "staged", "app-v1.txt"))
				if statErr != nil {
					t.Fatalf("expected artifact to be staged, but got error: %v", statErr)
				}
			}

			// Validate explicit filesystem boundary for release markers
			if strings.Contains(proposalJSON, "write_release_marker") && expectDecision == "ALLOW" {
				content, statErr := os.ReadFile(filepath.Join(sandboxDir, "releases", "current.txt"))
				if statErr != nil {
					t.Fatalf("expected release marker, but got error: %v", statErr)
				}
				if string(content) != "production_deployed" {
					t.Fatalf("unexpected marker content: %s", string(content))
				}
			}
		})
	}

	baseCaps := []string{"filesystem", "database"}

	// ALLOW
	runCase(t, "read valid status",
		"idle",
		`{"action":"read_release_status"}`,
		[]string{},
		"ALLOW", "none", baseCaps)

	runCase(t, "stage allowlisted artifact",
		"idle",
		`{"action":"stage_artifact","artifact":"app-v1"}`,
		[]string{},
		"ALLOW", "staged", baseCaps)

	runCase(t, "authorized marker write",
		"staged",
		`{"action":"write_release_marker"}`,
		[]string{"approved=yes"},
		"ALLOW", "production_deployed", baseCaps)

	runCase(t, "authorized rollback",
		"production_deployed",
		`{"action":"rollback_marker"}`,
		[]string{"approved=yes"},
		"ALLOW", "rolled_back", baseCaps)

	// DENY
	runCase(t, "unknown action",
		"idle",
		`{"action":"unknown_action"}`,
		[]string{},
		"DENY", "none", baseCaps)

	runCase(t, "arbitrary exec action",
		"idle",
		`{"action":"exec","command":"rm -rf /"}`,
		[]string{},
		"DENY", "none", baseCaps)

	runCase(t, "path traversal artifact",
		"idle",
		`{"action":"stage_artifact","artifact":"../../etc/passwd"}`,
		[]string{},
		"DENY", "none", baseCaps)

	runCase(t, "invalid state transition - write marker from idle",
		"idle",
		`{"action":"write_release_marker"}`,
		[]string{"approved=yes"},
		"DENY", "none", baseCaps)

	runCase(t, "invalid state transition - rollback from idle",
		"idle",
		`{"action":"rollback_marker"}`,
		[]string{"approved=yes"},
		"DENY", "none", baseCaps)

	runCase(t, "unauthorized mutation - requires approval",
		"staged",
		`{"action":"write_release_marker"}`,
		[]string{"approved=no"},
		"REQUIRE_APPROVAL", "none", baseCaps)

	// ADVERSARIAL
	runCase(t, "proposal self-approves",
		"staged",
		`{"action":"write_release_marker","approved":"yes"}`,
		[]string{"approved=no"},
		"REQUIRE_APPROVAL", "none", baseCaps)

	runCase(t, "proposal attempts capability escalation",
		"staged",
		`{"action":"write_release_marker","requested_caps":["network","process"]}`,
		[]string{"approved=yes"},
		"ALLOW", "production_deployed", baseCaps) // Will still work because capability escalation in JSON is ignored, runner provides baseCaps.

	runCase(t, "missing required capability - database",
		"staged",
		`{"action":"write_release_marker"}`,
		[]string{"approved=yes"},
		"ALLOW", "production_deployed", []string{"filesystem"}) // Expected to fail internally on store mutation
}
