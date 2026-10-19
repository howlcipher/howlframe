package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

const (
	round6StormScript = "milestone2/ticketdesk/evidence/b6_kill_storm.py"
	round6Decision    = "milestone2/MILESTONE2_DECISION.md"
)

var round6ScriptPin = regexp.MustCompile("\\*\\*Script sha256:\\*\\* `([0-9a-f]{64})`")

// TestRound6StormScriptPinned enforces the Round 6 pre-registration: the
// kill-storm script must hash to the sha256 pinned in MILESTONE2_DECISION.md.
// Any edit to the script (or to the pin) without the other fails CI.
func TestRound6StormScriptPinned(t *testing.T) {
	doc, err := os.ReadFile(round6Decision)
	if err != nil {
		t.Fatal(err)
	}
	m := round6ScriptPin.FindSubmatch(doc)
	if m == nil {
		t.Fatalf("%s: no Round 6 '**Script sha256:** `<hex>`' pin found", round6Decision)
	}
	script, err := os.ReadFile(round6StormScript)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(script)
	if got, want := hex.EncodeToString(sum[:]), string(m[1]); got != want {
		t.Fatalf("%s sha256 = %s, but Round 6 pins %s; the script is frozen for Round 6", round6StormScript, got, want)
	}
}

// TestRound6StormScriptSigstopSelfTest runs the script's SIGSTOP self-test: a
// stopped process must still yield wchan/status/task diagnostics, and the
// SIGQUIT step must fall back to SIGKILL.
func TestRound6StormScriptSigstopSelfTest(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs /proc")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	cmd := exec.Command("python3", round6StormScript, "--self-test-sigstop", "--artifacts", t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "SELFTEST_SIGSTOP PASS") {
		t.Fatalf("self-test failed: %v\n%s", err, out)
	}
	for _, want := range []string{"DIAG wchan=", "DIAG status:", "DIAG task ", "falling back to SIGKILL", "SCRIPT_SHA256 "} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("self-test output missing %q:\n%s", want, out)
		}
	}
}
