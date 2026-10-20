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

// Pinned sha256 values, recorded in milestone3/FREEZE.md §5 (post-R4 values).
// None of the pinned files contains its own pin.
const (
	specPath     = "M3_ACCEPTANCE_SPEC.md"
	sealedPath   = "heldout/SEALED.sha256"
	ownerAckPath = "OWNER_ACK_2026-10-20.md"
	freezePath   = "FREEZE.md"

	pinSpec     = "3a3165bfcc4c41c2f8ef8cf977d0850a5386ada1960d5b7890250153ceb28a23"
	pinSealed   = "2bdb70870f55cbe029ecaab1889955ec878931671ab834aa5ce0518a0d3acf3b"
	pinSection7 = "7dbeb891f72c35c8416ea59e67ce4d1fb26877c93ca870a2f5e1f7aabdb3bd09"
	pinOwnerAck = "5b8a7e7a04b0b887ba85ef2da946513f80674f8de9be03f242fad181611a7ce3"
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
	} {
		if c.got != c.want {
			t.Errorf("%s sha256 = %s, pinned %s (a change needs a dated revision in milestone3/FREEZE.md §4 and a new pin)", c.name, c.got, c.want)
		}
	}
}

// Verifier source, per spec §1.2 (E2: milestone3/eco/src/<tool>.howl): every
// file under eco/src/, plus any non-Markdown file under eco/ outside the
// requirements, tests, evidence and heldout trees (written by non-builders).
var nonSource = []string{"eco/requirements/", "eco/tests/", "eco/evidence/", "eco/heldout/"}

func verifierSources(t *testing.T) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir("eco", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && p == "eco" {
				return fs.SkipDir
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		p = filepath.ToSlash(p)
		if strings.HasPrefix(p, "eco/src/") {
			out = append(out, p)
			return nil
		}
		for _, pre := range nonSource {
			if strings.HasPrefix(p, pre) {
				return nil
			}
		}
		if !strings.HasSuffix(p, ".md") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("walk eco: %v", err)
	}
	return out
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

// (iii) The verifier's reviewer (eco/DECISION.md "Reviewer: <agent id>") never
// appears as an "Agent: <agent id>" git trailer on a commit touching verifier
// source (FREEZE.md §3 condition 3). Needs full history: fails closed on a
// shallow or missing git history.
func TestM3VerifierReviewerNotBuilder(t *testing.T) {
	dec, err := os.ReadFile("eco/DECISION.md")
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	m := reviewerLine.FindSubmatch(dec)
	if m == nil {
		t.Fatal("milestone3/eco/DECISION.md exists but has no `Reviewer: <agent id>` line")
	}
	reviewer := string(m[1])

	shallow, err := exec.Command("git", "rev-parse", "--is-shallow-repository").Output()
	if err != nil {
		t.Fatalf("git history unavailable (%v): the reviewer/builder check cannot run; run with full history", err)
	}
	if strings.TrimSpace(string(shallow)) != "false" {
		t.Fatal("shallow clone: the reviewer/builder check needs full history (e.g. actions/checkout fetch-depth: 0); failing closed")
	}
	args := []string{"log", "--format=%H%x00%(trailers:key=Agent,valueonly,separator=%x01)%x00", "--",
		"eco", ":(exclude)eco/requirements", ":(exclude)eco/tests", ":(exclude)eco/evidence", ":(exclude)eco/heldout", ":(exclude)eco/*.md"}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		t.Fatalf("git log: %v", err)
	}
	f := strings.Split(string(out), "\x00")
	for i := 0; i+1 < len(f); i += 2 {
		commit := strings.TrimSpace(f[i])
		for _, a := range strings.Split(f[i+1], "\x01") {
			if strings.TrimSpace(a) == reviewer {
				t.Errorf("commit %s touches verifier source with `Agent: %s`, the recorded reviewer", commit, reviewer)
			}
		}
	}
}
