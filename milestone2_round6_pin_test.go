package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
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

// TestRound6ExpectHeadPinRules exercises --expect-head in a scratch repository:
// only HEAD == pin, or a single commit directly on the pin that changes exactly
// the Code-under-test line of MILESTONE2_DECISION.md (numstat 1 1, renames off),
// is accepted.
func TestRound6ExpectHeadPinRules(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	script, err := filepath.Abs(round6StormScript)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(round6Decision)
	if err != nil {
		t.Fatal(err)
	}
	const pinPrefix = "  * **Code-under-test SHA:** `"
	const thresholdMark = "the single invocation prints `SUMMARY runs=75 failures=0 ...` and exits 0;"
	if !strings.Contains(string(doc), "\n"+pinPrefix) || !strings.Contains(string(doc), thresholdMark) {
		t.Fatalf("%s lacks the pin line or the threshold line this test edits", round6Decision)
	}
	// The scratch repo always starts from the placeholder pin line, so this test keeps
	// working after the real decision doc has been pinned for Round 6.
	{
		lines := strings.Split(string(doc), "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, pinPrefix) {
				lines[i] = pinPrefix + "TO BE PINNED BEFORE THE RUN (main including #191 and this script)`"
			}
		}
		doc = []byte(strings.Join(lines, "\n"))
	}
	repo := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := func(pin string) (string, bool) {
		t.Helper()
		cmd := exec.Command("python3", script, "--check-head-only", "--expect-head", pin, "--artifacts", t.TempDir())
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		return string(out), err == nil && strings.Contains(string(out), "CHECK_HEAD PASS pinned="+pin)
	}
	run("init", "-q", "-b", "main")
	write("milestone2/MILESTONE2_DECISION.md", string(doc))
	write("product.go", "package product\n")
	run("add", "-A")
	run("commit", "-q", "-m", "code under test")
	pin := run("rev-parse", "HEAD")
	pinned := func(text string) string {
		lines := strings.Split(text, "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, pinPrefix) {
				lines[i] = pinPrefix + pin + "`"
			}
		}
		return strings.Join(lines, "\n")
	}

	// (a) exact match.
	if out, ok := check(pin); !ok {
		t.Fatalf("exact match rejected:\n%s", out)
	}

	// (b) one docs-only commit that fills the pin line: accepted.
	run("checkout", "-q", "-b", "pinline", pin)
	write("milestone2/MILESTONE2_DECISION.md", pinned(string(doc)))
	run("commit", "-q", "-am", "pin code under test")
	if out, ok := check(pin); !ok {
		t.Fatalf("pin-line commit rejected:\n%s", out)
	}

	// (c) rename of a .go file to .md: rejected.
	run("checkout", "-q", "-b", "rename", pin)
	run("mv", "product.go", "product.md")
	run("commit", "-q", "-m", "rename go to md")
	if out, ok := check(pin); ok {
		t.Fatalf(".go -> .md rename accepted:\n%s", out)
	}

	// (d1) a later .md rewrite of the Round 6 threshold on top of the pin line: rejected.
	run("checkout", "-q", "pinline")
	run("checkout", "-q", "-b", "later", "pinline")
	write("milestone2/MILESTONE2_DECISION.md", strings.Replace(pinned(string(doc)), thresholdMark, "the invocation prints any SUMMARY;", 1))
	run("commit", "-q", "-am", "relax threshold")
	if out, ok := check(pin); ok {
		t.Fatalf("later threshold rewrite accepted:\n%s", out)
	}

	// (d2) the threshold rewritten (1 line) directly on the pin instead of the pin line: rejected.
	run("checkout", "-q", "-b", "threshold", pin)
	write("milestone2/MILESTONE2_DECISION.md", strings.Replace(string(doc), thresholdMark, "the invocation prints any SUMMARY;", 1))
	run("commit", "-q", "-am", "relax threshold only")
	if out, ok := check(pin); ok {
		t.Fatalf("one-line threshold rewrite accepted:\n%s", out)
	}

	// Substring attacks on the pin line (Motoko, #193 at c89bef2f): rejected.
	pinLine := pinPrefix + pin + "`"
	for name, line := range map[string]string{
		"other-sha-mentioning-pin": pinPrefix + strings.Repeat("0", 40) + " (not " + pin + ")`",
		"pin-plus-trailing-text":   pinLine + " (threshold relaxed)",
	} {
		run("checkout", "-q", "-b", "attack-"+name, pin)
		lines := strings.Split(string(doc), "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, pinPrefix) {
				lines[i] = line
			}
		}
		write("milestone2/MILESTONE2_DECISION.md", strings.Join(lines, "\n"))
		run("commit", "-q", "-am", "attack "+name)
		if out, ok := check(pin); ok {
			t.Fatalf("pin line %q accepted:\n%s", line, out)
		}
	}

	// Parent check: two commits whose net diff is only the pin line: rejected.
	run("checkout", "-q", "-b", "twostep", pin)
	write("product.go", "package product\n\nvar Changed = true\n")
	run("commit", "-q", "-am", "change product")
	write("product.go", "package product\n")
	write("milestone2/MILESTONE2_DECISION.md", pinned(string(doc)))
	run("commit", "-q", "-am", "revert product and pin")
	if out, ok := check(pin); ok {
		t.Fatalf("two-commit pin accepted:\n%s", out)
	}

	// The pin commit must not be a merge, even one whose net diff is the pin line: rejected.
	run("checkout", "-q", "-b", "merged", pin)
	run("merge", "-q", "--no-ff", "-m", "merge pin line", "pinline")
	if out, ok := check(pin); ok {
		t.Fatalf("merge pin commit accepted:\n%s", out)
	}

	// Placeholder check: re-pinning a line that was already filled in: rejected.
	run("checkout", "-q", "-b", "prefilled", pin)
	prefilled := strings.Replace(pinned(string(doc)), pin, strings.Repeat("1", 40), 1)
	write("milestone2/MILESTONE2_DECISION.md", prefilled)
	run("commit", "-q", "-am", "already pinned elsewhere")
	pin2 := run("rev-parse", "HEAD")
	write("milestone2/MILESTONE2_DECISION.md", strings.Replace(prefilled, strings.Repeat("1", 40), pin2, 1))
	run("commit", "-q", "-am", "re-pin")
	if out, ok := check(pin2); ok {
		t.Fatalf("re-pin of a non-placeholder line accepted:\n%s", out)
	}

	// (e) pin line naming a different commit: rejected.
	run("checkout", "-q", "-b", "wrongsha", pin)
	write("milestone2/MILESTONE2_DECISION.md", strings.Replace(pinned(string(doc)), pin, strings.Repeat("0", 40), 1))
	run("commit", "-q", "-am", "pin wrong sha")
	if out, ok := check(pin); ok {
		t.Fatalf("pin line naming another sha accepted:\n%s", out)
	}
}
