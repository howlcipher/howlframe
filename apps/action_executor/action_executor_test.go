package action_executor_test

import (
	"encoding/json"
	"errors"
	"io"
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

			markerPath := filepath.Join(sandboxDir, "releases", "current.txt")
			if err := os.WriteFile(markerPath, []byte("kept"), 0o600); err != nil {
				t.Fatal(err)
			}

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
					if !strings.Contains(string(out), "CAPABILITY_DENIED") {
						t.Fatalf("expected capability denial: %s", out)
					}
					if _, err := os.Stat(filepath.Join(sandboxDir, "staged", "app-v1.txt")); !os.IsNotExist(err) {
						t.Fatalf("denied action created staged file: %v", err)
					}
					content, readErr := os.ReadFile(markerPath)
					if readErr != nil || string(content) != "kept" {
						t.Fatalf("denied action changed marker: %q, %v", content, readErr)
					}
					return
				}
				t.Fatalf("unexpected failure: %v\n%s", err, string(out))
			}

			if expectDecision == "ERROR" {
				t.Fatalf("expected error, but succeeded with output:\n%s", string(out))
			}

			outStr := string(out)

			reader := strings.NewReader(outStr)
			decoder := json.NewDecoder(reader)
			var decision map[string]string
			if err := decoder.Decode(&decision); err != nil {
				t.Fatalf("decode decision: %v; output=%s", err, outStr)
			}
			if decision["decision"] != expectDecision {
				t.Fatalf("expected decision %s, got %v", expectDecision, decision)
			}
			rest, err := io.ReadAll(io.MultiReader(decoder.Buffered(), reader))
			if err != nil {
				t.Fatal(err)
			}
			// Follow-up status/health lines remain outside the decision JSON.
			trailing := strings.TrimSpace(string(rest))
			if strings.Contains(proposalJSON, "read_release_status") && trailing != "status=idle" {
				t.Fatalf("status output = %q", trailing)
			}
			if strings.Contains(proposalJSON, "run_health_check") && trailing != "health=ok_marker_kept" {
				t.Fatalf("health output = %q", trailing)
			}
			if !strings.Contains(proposalJSON, "read_release_status") && !strings.Contains(proposalJSON, "run_health_check") && trailing != "" {
				t.Fatalf("extra output: %q", trailing)
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
	for _, caps := range [][]string{{"filesystem"}, {"database"}} {
		runCase(t, "stage preflight "+caps[0], "idle", `{"action":"stage_artifact","artifact":"app-v1"}`, nil, "ALLOW", "staged", caps)
		runCase(t, "marker preflight "+caps[0], "staged", `{"action":"write_release_marker"}`, []string{"approved=yes"}, "ALLOW", "production_deployed", caps)
		runCase(t, "rollback preflight "+caps[0], "production_deployed", `{"action":"rollback_marker"}`, []string{"approved=yes"}, "ALLOW", "rolled_back", caps)
	}
	runCase(t, "health follow-up", "idle", `{"action":"run_health_check"}`, nil, "ALLOW", "none", baseCaps)

}
