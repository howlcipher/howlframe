// Package m3freeze holds the Milestone 3 spec-freeze guards (milestone3/FREEZE.md).
// It has no non-test code; `go test ./...` runs it from this directory.
package m3freeze

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Pinned sha256 values (post-R7). FREEZE.md §5 records every pin except
// pinFreeze, FREEZE.md's own hash, which only this file holds. No pinned file
// contains its own pin.
const (
	specPath     = "M3_ACCEPTANCE_SPEC.md"
	sealedPath   = "heldout/SEALED.sha256"
	ownerAckPath = "OWNER_ACK_2026-10-20.md"
	freezePath   = "FREEZE.md"

	pinSpec     = "8a20258e80b18384842bca6b65b27869dce143669610e2236e3735b5b2f8782c"
	pinSealed   = "2bdb70870f55cbe029ecaab1889955ec878931671ab834aa5ce0518a0d3acf3b"
	pinSection7 = "7dbeb891f72c35c8416ea59e67ce4d1fb26877c93ca870a2f5e1f7aabdb3bd09"
	pinOwnerAck = "380370403e266310d43cc09abdbf67b27ac3617be2ff4a063a9cde8d9cbc082f"
	pinFreeze   = "bbdb9c6dc038c5494276014aba0738bb953e1e993a19153f5fac887794ef2a3f"
)

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func read(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return b
}

// section7 returns the bytes from the "## 7. " heading up to (not including)
// the "## 8. " heading, the same span as
// awk '/^## 7\. /{f=1} /^## 8\. /{f=0} f'.
func section7(t *testing.T, spec []byte) []byte {
	t.Helper()
	s := string(spec)
	start := regexp.MustCompile(`(?m)^## 7\. `).FindStringIndex(s)
	end := regexp.MustCompile(`(?m)^## 8\. `).FindStringIndex(s)
	if start == nil || end == nil || end[0] <= start[0] {
		t.Fatalf("spec: cannot find the ## 7. .. ## 8. span")
	}
	return []byte(s[start[0]:end[0]])
}

// (i) Frozen files and the sealed §7 span match their pins.
func TestM3SpecFreezePins(t *testing.T) {
	spec := read(t, specPath)
	for _, c := range []struct{ name, got, want string }{
		{specPath, sha(spec), pinSpec},
		{sealedPath, sha(read(t, sealedPath)), pinSealed},
		{specPath + " §7 span", sha(section7(t, spec)), pinSection7},
		{ownerAckPath, sha(read(t, ownerAckPath)), pinOwnerAck},
		{freezePath, sha(read(t, freezePath)), pinFreeze},
	} {
		if c.got != c.want {
			t.Errorf("%s sha256 = %s, pinned %s (a change needs a dated revision in milestone3/FREEZE.md §4 and a new pin)", c.name, c.got, c.want)
		}
	}
}

// Verifier source (FREEZE.md §5). Paths are relative to milestone3/.
//   - every file under eco/src/ (spec §1.2, E2: milestone3/eco/src/<tool>.howl) or verifier/;
//   - any non-Markdown file under eco/ outside requirements/, tests/, evidence/ and heldout/;
//   - any .howl, .hfbc or .go file anywhere under milestone3/ (eco/tests/ and heldout/ included)
//     that is not on sourceAllowlist.
var nonSource = []string{"eco/requirements/", "eco/tests/", "eco/evidence/", "eco/heldout/"}

var sourceExt = []string{".howl", ".hfbc", ".go"}

// sourceAllowlist is explicit and minimal; FREEZE.md §5 lists the same entries.
// Each entry maps a path to its pinned sha256, or to "" for this test file
// (it cannot pin itself). Adding an entry, for example an Auditor E-gate
// fixture .howl, needs a dated FREEZE.md §4 revision and a pin.
var sourceAllowlist = map[string]string{
	"spec_freeze_test.go": "",
}

// isVerifierSource matches case-insensitively (R7): .GO, .Howl and ECO/SRC/ all count. HFIR
// .json is source under eco/src/, verifier/ and non-allowlisted eco/ paths (all their files are).
func isVerifierSource(p string) bool {
	if _, ok := sourceAllowlist[p]; ok {
		return false
	}
	p = strings.ToLower(p)
	if strings.HasPrefix(p, "eco/src/") || strings.HasPrefix(p, "verifier/") {
		return true
	}
	for _, e := range sourceExt {
		if strings.HasSuffix(p, e) {
			return true
		}
	}
	if !strings.HasPrefix(p, "eco/") || strings.HasSuffix(p, ".md") {
		return false
	}
	for _, pre := range nonSource {
		if strings.HasPrefix(p, pre) {
			return false
		}
	}
	return true
}

// isSuitePath: files of the Auditor's E-gate suite and fixtures (FREEZE.md §3 condition 3).
func isSuitePath(p string) bool { return strings.HasPrefix(strings.ToLower(p), "eco/tests/") }

func verifierSources(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if p = filepath.ToSlash(p); isVerifierSource(p) {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk milestone3: %v", err)
	}
	return out
}

// Allowlisted files other than this test match their pins.
func TestM3SourceAllowlistPins(t *testing.T) {
	for p, want := range sourceAllowlist {
		if want == "" {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("allowlisted %s: %v", p, err)
			continue
		}
		if got := sha(b); got != want {
			t.Errorf("allowlisted %s sha256 = %s, pinned %s", p, got, want)
		}
	}
}

// sealed-v1 P-ECO bind marker: a row for sealed-v1-eco-bind in the §7.3
// commitments table (spec §7.1 step 3: "its hashes are committed in a dated
// revision of §7.3").
var ecoBindRow = regexp.MustCompile("(?m)^\\|\\s*`?sealed-v1-eco-bind`?\\s*\\|")

// (ii) No verifier source before the freeze, the sealed-v1 bind and the E-gate
// suite (FREEZE.md §3 conditions 1-2).
func TestM3NoVerifierBeforeFreezeConditions(t *testing.T) {
	src := verifierSources(t)
	if len(src) == 0 {
		return
	}
	var missing []string
	if _, err := os.Stat(freezePath); err != nil {
		missing = append(missing, "milestone3/FREEZE.md")
	}
	if _, err := os.Stat("eco/requirements/FREEZE.md"); err != nil {
		missing = append(missing, "milestone3/eco/requirements/FREEZE.md (P-ECO requirements freeze, spec §1.1)")
	}
	if !ecoBindRow.Match(section7(t, read(t, specPath))) {
		missing = append(missing, "sealed-v1-eco-bind row in spec §7.3")
	}
	if fi, err := os.Stat("eco/tests/acceptance"); err != nil || !fi.IsDir() {
		missing = append(missing, "milestone3/eco/tests/acceptance/ (Auditor's E-gate suite)")
	}
	if len(missing) > 0 {
		t.Fatalf("P-ECO verifier source present (%v) before freeze conditions are met; missing: %s", src, strings.Join(missing, "; "))
	}
}

var reviewerLine = regexp.MustCompile("(?m)^\\s*(?:[-*]\\s*)?\\**Reviewer\\**\\s*:\\**\\s*`?([A-Za-z0-9._@-]+)`?")

type commitInfo struct {
	hash   string
	agents []string
	files  []string
}

// history returns every commit touching milestone3/, with its Agent: trailers and
// the files it touched (relative to milestone3/), from `git log` run here.
func history(t *testing.T) []commitInfo {
	t.Helper()
	out, err := exec.Command("git", "log", "--relative", "--name-only",
		"--format=%x00%H%x01%(trailers:key=Agent,valueonly,separator=%x02)%x01", "--", ".").Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	var cs []commitInfo
	for _, rec := range strings.Split(string(out), "\x00")[1:] {
		parts := strings.SplitN(rec, "\x01", 3)
		if len(parts) != 3 {
			t.Fatalf("git log: unexpected record %q", rec)
		}
		c := commitInfo{hash: parts[0]}
		for _, a := range strings.Split(parts[1], "\x02") {
			if a = strings.TrimSpace(a); a != "" {
				c.agents = append(c.agents, a)
			}
		}
		for _, f := range strings.Split(parts[2], "\n") {
			if f = strings.TrimSpace(f); f != "" {
				c.files = append(c.files, f)
			}
		}
		cs = append(cs, c)
	}
	return cs
}

// skipHistoryMsg is the Owner-decided skip for shallow clones (CI).
const skipHistoryMsg = "history checks are manual-only per Owner decision 2026-10-20 16:48 ET; run on a full clone and save evidence (see milestone3/FREEZE.md R6)"

// requireFullHistory skips on a shallow clone (R6) and fails if git history is unavailable.
func requireFullHistory(t *testing.T) {
	t.Helper()
	out, err := exec.Command("git", "rev-parse", "--is-shallow-repository").Output()
	if err != nil {
		t.Fatalf("git history unavailable (%v): the history checks cannot run; failing closed", err)
	}
	if strings.TrimSpace(string(out)) != "false" {
		t.Skip(skipHistoryMsg)
	}
}

// decisionReviewer returns the "Reviewer: <agent id>" of eco/DECISION.md, or "" if the
// file does not exist. A DECISION.md without a Reviewer line fails.
func decisionReviewer(t *testing.T) string {
	t.Helper()
	dec, err := os.ReadFile("eco/DECISION.md")
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	m := reviewerLine.FindSubmatch(dec)
	if m == nil {
		t.Fatal("milestone3/eco/DECISION.md exists but has no `Reviewer: <agent id>` line")
	}
	return string(m[1])
}

// Non-history: eco/DECISION.md, once present, names its reviewer. Runs at any clone depth.
func TestM3DecisionNamesReviewer(t *testing.T) { decisionReviewer(t) }

// (iii) History checks, FREEZE.md §3 condition 3 (MANUAL-ONLY per R6; evidence at
// milestone3/eco/evidence/freeze_history_<head-sha>.txt):
//   - every commit touching verifier source or the E-gate suite (eco/tests/) carries an Agent: trailer;
//   - builders (Agent: trailers on verifier-source commits) never include the reviewer named in
//     eco/DECISION.md, nor any author of the E-gate suite (Agent: trailers on eco/tests/ commits).
//
// On a shallow clone it skips (R6); with no git history it fails. On a full clone it always
// scans, so source added and later deleted is still caught. It holds no non-history check.
func TestM3VerifierReviewerNotBuilder(t *testing.T) {
	requireFullHistory(t)

	builders := map[string][]string{}
	authors := map[string][]string{}
	for _, c := range history(t) {
		var src, suite bool
		for _, f := range c.files {
			src = src || isVerifierSource(f)
			suite = suite || isSuitePath(f)
		}
		if (src || suite) && len(c.agents) == 0 {
			t.Errorf("commit %s touches verifier source or the E-gate suite and has no `Agent:` trailer (fail closed)", c.hash)
		}
		for _, a := range c.agents {
			if src {
				builders[a] = append(builders[a], c.hash)
			}
			if suite {
				authors[a] = append(authors[a], c.hash)
			}
		}
	}
	for a, hs := range builders {
		if ah, ok := authors[a]; ok {
			t.Errorf("`Agent: %s` is both a verifier builder (%v) and an E-gate suite author (%v)", a, hs, ah)
		}
	}
	if r := decisionReviewer(t); r != "" {
		if hs, ok := builders[r]; ok {
			t.Errorf("the recorded reviewer `%s` is a verifier builder (`Agent:` trailer on %v)", r, hs)
		}
	}
}
