package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/agent"
	"github.com/howlcipher/howlframe/internal/verification"
)

func runVerificationReportCmd(t *testing.T, bin string, args ...string) (verification.Report, int, []byte) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		exitCode = exitErr.ExitCode()
	}
	var report verification.Report
	if decErr := json.Unmarshal(out, &report); decErr != nil {
		t.Fatalf("%v invalid JSON: %v\n%s", args, decErr, out)
	}
	return report, exitCode, out
}

func findReportCheck(report verification.Report, name string) (verification.Check, bool) {
	for _, c := range report.Checks {
		if c.Name == name {
			return c, true
		}
	}
	return verification.Check{}, false
}

func TestVerificationReportCheckAndInspectCLI(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()

	incPath := filepath.Join(dir, "part.howl")
	if err := os.WriteFile(incPath, []byte(`(print "hello")`), 0o600); err != nil {
		t.Fatal(err)
	}
	incSource := filepath.Join(dir, "inc.howl")
	if err := os.WriteFile(incSource, []byte(`(cli_app (include "part.howl"))`), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name        string
		profile     string
		target      string
		source      string
		sourcePath  string
		wantPass    bool
		wantPartial bool
		wantExit    int
		checkName   string
		checkStatus string
		wantCodes   []string
	}{
		{
			name:        "check_default_skips_artifact",
			source:      `(cli_app (print (+ 1 2)))`,
			wantPass:    true,
			wantPartial: true,
			wantExit:    0,
			checkName:   "artifact_validation",
			checkStatus: verification.StatusSkipped,
		},
		{
			name:        "check_bytecode_full_pass",
			target:      "bytecode",
			source:      `(cli_app (print (+ 1 2)))`,
			wantPass:    true,
			wantPartial: false,
			wantExit:    0,
			checkName:   "artifact_validation",
			checkStatus: verification.StatusPass,
		},
		{
			name:        "check_include_keeps_stdout_clean_json",
			target:      "bytecode",
			sourcePath:  incSource,
			wantPass:    true,
			wantPartial: false,
			wantExit:    0,
			checkName:   "parser",
			checkStatus: verification.StatusPass,
		},
		{
			name:        "check_governed_forbidden",
			profile:     "governed",
			target:      "bytecode",
			source:      `(cli_app (print (exec "echo hi")))`,
			wantPass:    false,
			wantPartial: true,
			wantExit:    1,
			checkName:   "profile_policy",
			checkStatus: verification.StatusFail,
			wantCodes:   []string{"PROFILE_FORBIDDEN_CONSTRUCT"},
		},
		{
			name:        "check_parse_error",
			target:      "bytecode",
			source:      `(cli_app (print`,
			wantPass:    false,
			wantPartial: true,
			wantExit:    1,
			checkName:   "parser",
			checkStatus: verification.StatusFail,
		},
		{
			name:        "check_unsupported_go_value_form",
			target:      "go",
			source:      `(cli_app (print (try_let (x (parse_json Any "{}")) (catch e "err") x)))`,
			wantPass:    false,
			wantPartial: true,
			wantExit:    1,
			checkName:   "checker",
			checkStatus: verification.StatusFail,
			wantCodes:   []string{"UNSUPPORTED_ON_TARGET"},
		},
		{
			name:        "check_hfir_unsupported_bytecode_construct",
			target:      "bytecode",
			source:      `(cli_app (test "s" (assert true)))`,
			wantPass:    false,
			wantPartial: true,
			wantExit:    1,
			checkName:   "hfir_gate",
			checkStatus: verification.StatusFail,
			wantCodes:   []string{"HFIR_TARGET_INFEASIBLE"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.sourcePath
			if src == "" {
				src = filepath.Join(dir, tc.name+".howl")
				if err := os.WriteFile(src, []byte(tc.source), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"check", "--json"}
			if tc.profile != "" {
				args = append(args, "--profile", tc.profile)
			}
			if tc.target != "" {
				args = append(args, "--target", tc.target)
			}
			args = append(args, src)

			rep, code, raw1 := runVerificationReportCmd(t, bin, args...)
			if rep.Schema != verification.ReportSchema {
				t.Fatalf("schema = %q, want %q", rep.Schema, verification.ReportSchema)
			}
			if code != tc.wantExit || rep.Passed != tc.wantPass || rep.Partial != tc.wantPartial {
				t.Fatalf("status mismatch: code=%d passed=%v partial=%v report=%+v", code, rep.Passed, rep.Partial, rep)
			}
			if rep.Versions.Compiler != Version || rep.Versions.HFBCFormat != HFBCFormatVersion || rep.Versions.ConstructRegistry <= 0 {
				t.Fatalf("versions incomplete: %+v", rep.Versions)
			}
			if !slices.Contains(rep.HFIRBlockingCodes, "HFIR_TARGET_INFEASIBLE") {
				t.Fatalf("missing blocking code list: %v", rep.HFIRBlockingCodes)
			}
			if len(rep.Limitations) < 2 {
				t.Fatalf("expected explicit limitations, got %v", rep.Limitations)
			}
			chk, ok := findReportCheck(rep, tc.checkName)
			if !ok || chk.Status != tc.checkStatus {
				t.Fatalf("check %q = (%+v, %v), want status %q", tc.checkName, chk, ok, tc.checkStatus)
			}
			for _, wantCode := range tc.wantCodes {
				if !slices.Contains(chk.Codes, wantCode) {
					t.Fatalf("check %q codes = %v, missing %q", tc.checkName, chk.Codes, wantCode)
				}
			}
			if tc.name == "check_bytecode_full_pass" {
				if rep.ArtifactSHA256 == "" {
					t.Fatal("expected ArtifactSHA256 for in-memory bytecode check")
				}
				if _, _, raw2 := runVerificationReportCmd(t, bin, args...); string(raw1) != string(raw2) {
					t.Fatalf("non-deterministic JSON report:\n%s\nvs\n%s", raw1, raw2)
				}
			}
		})
	}

	// Build a valid artifact and inspect --report vs corrupted bytes and agent check --json.
	pureSrc := filepath.Join(dir, "pure.howl")
	if err := os.WriteFile(pureSrc, []byte(`(cli_app (print (+ 20 22)))`), 0o600); err != nil {
		t.Fatal(err)
	}
	artPath := filepath.Join(dir, "pure.hfbc")
	if out, err := exec.Command(bin, "build", "--target", "bytecode", "-o", artPath, pureSrc).CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v %s", err, out)
	}
	artBytes, err := os.ReadFile(artPath)
	if err != nil {
		t.Fatal(err)
	}
	wantSHA := fmt.Sprintf("%x", sha256.Sum256(artBytes))

	inspectRep, inspectCode, _ := runVerificationReportCmd(t, bin, "inspect", "--report", artPath)
	if inspectCode != 0 || !inspectRep.Passed || !inspectRep.Partial || inspectRep.ArtifactSHA256 != wantSHA {
		t.Fatalf("inspect --report valid artifact mismatch: code=%d %+v", inspectCode, inspectRep)
	}
	artCheck, _ := findReportCheck(inspectRep, "artifact_validation")
	if artCheck.Status != verification.StatusPass {
		t.Fatalf("expected artifact_validation pass, got %+v", artCheck)
	}

	corruptPath := filepath.Join(dir, "corrupt.hfbc")
	if err := os.WriteFile(corruptPath, []byte("not-an-hfbc-artifact"), 0o600); err != nil {
		t.Fatal(err)
	}
	badRep, badCode, _ := runVerificationReportCmd(t, bin, "inspect", "--report", corruptPath)
	if badCode != 1 || badRep.Passed || !badRep.Partial {
		t.Fatalf("corrupt inspect --report must fail closed: code=%d %+v", badCode, badRep)
	}
	badArtCheck, _ := findReportCheck(badRep, "artifact_validation")
	if badArtCheck.Status != verification.StatusFail {
		t.Fatalf("expected corrupt artifact_validation fail, got %+v", badArtCheck)
	}

	agentOut, err := exec.Command(bin, "agent", "check", "--json", pureSrc).CombinedOutput()
	if err != nil {
		t.Fatalf("agent check --json failed: %v\n%s", err, agentOut)
	}
	var agentResp agent.Response
	if err := json.Unmarshal(agentOut, &agentResp); err != nil {
		t.Fatalf("agent check decode error: %v", err)
	}
	if !agentResp.OK || agentResp.VerificationReport == nil || agentResp.VerificationReport.Schema != verification.ReportSchema || !agentResp.VerificationReport.Passed {
		t.Fatalf("agent check missing structured verification_report: %+v", agentResp)
	}
	if !strings.Contains(string(agentOut), verification.ReportSchema) {
		t.Fatalf("expected %q in agent check JSON output", verification.ReportSchema)
	}
}
