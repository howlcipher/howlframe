package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Milestone 2 gate H: the independently written developer-experience checker
// (milestone2/ticketdesk/dx) must pass against the current compiler. It covers
// unclosed and stray parentheses, a missing (call ...), a multi-form let body and
// mismatched if branches.
func TestDeveloperExperienceDiagnostics(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	binary := buildHowlFrameBinaryForTest(t)
	script, err := filepath.Abs("milestone2/ticketdesk/dx/check_dx.py")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "-I", script)
	cmd.Env = append(os.Environ(), "HF_BIN="+binary)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("DX checker failed: %v\n%s", err, out)
	}
}
