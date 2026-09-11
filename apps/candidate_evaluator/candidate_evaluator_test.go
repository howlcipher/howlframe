package candidate_evaluator_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type AssessmentResult struct {
	SchemaVersion string `json:"schema_version"`
	AssessmentID  string `json:"assessment_id"`
	CandidateID   string `json:"candidate_id"`
	Disposition   string `json:"disposition"`
	Confidence    string `json:"confidence"`
	Reason        string `json:"reason"`
	Authority     struct {
		Type       string `json:"type"`
		Executable bool   `json:"executable"`
	} `json:"authority"`
}

func TestCandidateEvaluator(t *testing.T) {
	repositoryRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	scratchDir := t.TempDir()
	compiler := filepath.Join(scratchDir, "howlframe")
	artifact := filepath.Join(scratchDir, "candidate_evaluator.hfbc")

	build := exec.Command("go", "build", "-o", compiler, "howlframe.go")
	build.Dir = repositoryRoot
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build HowlFrame scratch binary: %v\n%s", buildErr, output)
	}

	appSource := filepath.Join(repositoryRoot, "apps", "candidate_evaluator", "candidate_evaluator.howl")
	compile := exec.Command(compiler, "-compile-bc", appSource, "-o", artifact)
	compile.Dir = filepath.Join(repositoryRoot, "apps", "candidate_evaluator")
	if output, compileErr := compile.CombinedOutput(); compileErr != nil {
		t.Fatalf("compile application to bytecode: %v\n%s", compileErr, output)
	}

	runCase := func(t *testing.T, name string, candidateJSON string, expectDisposition string, expectAuthExecutable bool) {
		t.Run(name, func(t *testing.T) {
			candPath := filepath.Join(scratchDir, "candidate.json")
			if err := os.WriteFile(candPath, []byte(candidateJSON), 0o600); err != nil {
				t.Fatalf("write candidate file: %v", err)
			}

			args := []string{"-run-bc", "-allow-caps", "filesystem", artifact, candPath}
			cmd := exec.Command(compiler, args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("execution failed: %v\nOutput: %s", err, string(out))
			}

			var assessment AssessmentResult
			jsonStr := strings.TrimSpace(string(out))
			if err := json.Unmarshal([]byte(jsonStr), &assessment); err != nil {
				t.Fatalf("failed to unmarshal assessment JSON: %v\nRaw: %s", err, jsonStr)
			}

			if assessment.Disposition != expectDisposition {
				t.Errorf("expected disposition %q, got %q (reason: %s)", expectDisposition, assessment.Disposition, assessment.Reason)
			}
			if assessment.Authority.Executable != expectAuthExecutable {
				t.Errorf("expected authority.executable=%v, got %v", expectAuthExecutable, assessment.Authority.Executable)
			}
			if assessment.Authority.Type != "ADVISORY" {
				t.Errorf("expected authority.type='ADVISORY', got %q", assessment.Authority.Type)
			}
		})
	}

	// Case 1: Locally verified candidate -> ACCEPT_FOR_DEVELOPMENT
	cand1 := `{
		"candidate_id": "run-001/candidates/0/0",
		"status": "LOCALLY_VERIFIED",
		"trust": "UNVERIFIED",
		"authority": {"type": "ADVISORY", "executable": false}
	}`
	runCase(t, "LocallyVerifiedPromotesToDevelopment", cand1, "ACCEPT_FOR_DEVELOPMENT", false)

	// Case 2: Rejected candidate -> REJECT
	cand2 := `{
		"candidate_id": "run-001/candidates/0/1",
		"status": "REJECTED",
		"trust": "UNVERIFIED",
		"authority": {"type": "ADVISORY", "executable": false}
	}`
	runCase(t, "RejectedCandidateStaysRejected", cand2, "REJECT", false)

	// Case 3: Unresolved assumptions -> UNRESOLVED
	cand3 := `{
		"candidate_id": "run-001/candidates/0/2",
		"status": "UNRESOLVED",
		"trust": "UNVERIFIED",
		"authority": {"type": "ADVISORY", "executable": false}
	}`
	runCase(t, "UnresolvedRemainsUnresolved", cand3, "UNRESOLVED", false)

	// Case 4: Authority escalation attempt (executable=true) -> REJECT
	cand4 := `{
		"candidate_id": "run-001/candidates/0/3",
		"status": "LOCALLY_VERIFIED",
		"trust": "UNVERIFIED",
		"authority": {"type": "ADVISORY", "executable": true}
	}`
	runCase(t, "AuthorityEscalationAttemptRejected", cand4, "REJECT", false)

	// Case 5: Authority escalation attempt (type=EXECUTIVE) -> REJECT
	cand5 := `{
		"candidate_id": "run-001/candidates/0/4",
		"status": "LOCALLY_VERIFIED",
		"trust": "UNVERIFIED",
		"authority": {"type": "EXECUTIVE", "executable": false}
	}`
	runCase(t, "ExecutiveAuthorityTypeRejected", cand5, "REJECT", false)
}
