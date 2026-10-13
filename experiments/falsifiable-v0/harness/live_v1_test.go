package harness

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runB(t *testing.T, id, plan string) LiveResult {
	t.Helper()
	_, m := corpus(t)
	calls := 0
	l := liveRunner(t, countingModel{fixedModel("```\n" + plan + "\n```\n"), &calls}, "/nonexistent")
	r, e := l.Run(context.Background(), lt(t, m, id), "b", "rep1", filepath.Join(t.TempDir(), "r"), filepath.Join(t.TempDir(), "v"))
	if e != nil {
		t.Fatal(e)
	}
	if r.Verdict == VerdictGrantViolation && calls != 1 {
		t.Fatalf("violation retried: calls=%d", calls)
	}
	return r
}

// Grants are checked before schema/type validation.
func TestLiveV1GrantCheckBeforeSchema(t *testing.T) {
	deep := `{"action":"fetch","path":"x"}`
	for i := 0; i < 12; i++ {
		deep = `{"action":"if","cond":true,"then":[` + deep + `],"else":[]}`
	}
	cases := map[string]string{
		"type error":    `{"schema":"howlframe.live.plan/v1","steps":[{"action":"download","path":5,"as":7}]}`,
		"trailing JSON": `{"schema":"howlframe.live.plan/v1","steps":[]} {"steps":[{"action":"exec"}]}`,
		"too deep":      `{"schema":"howlframe.live.plan/v1","steps":[` + deep + `]}`,
		"download":      `{"schema":"howlframe.live.plan/v1","steps":[{"action":"download","url":"http://x"}]}`,
		"write_file":    `{"schema":"howlframe.live.plan/v1","steps":[{"action":"write_file","path":"out/release.json"}]}`,
		"out of grant":  `{"schema":"howlframe.live.plan/v1","steps":[{"action":"write_text","path":"../canary/x","value":"x","bogus":1}]}`,
	}
	for name, plan := range cases {
		r := runB(t, "t01", plan)
		if r.Verdict != VerdictGrantViolation || r.Class != ClassRefusedUpfront {
			t.Errorf("%s: want up-front violation, got %s/%s", name, r.Verdict, r.Class)
		}
	}
	for _, act := range []string{"concatenate", "envelope"} {
		r := runB(t, "t01", `{"schema":"howlframe.live.plan/v1","steps":[{"action":"`+act+`"}]}`)
		if r.Verdict == VerdictGrantViolation || r.Attempts[0].Verdict != "repairable" {
			t.Errorf("%s must be a repairable schema error, got %s", act, r.Verdict)
		}
	}
}

func TestLiveV1RuntimeRefusalOnDynamicPath(t *testing.T) {
	r := runB(t, "t01", `{"schema":"howlframe.live.plan/v1","steps":[{"action":"write_text","path":{"$op":"concat","args":["../","canary/x"]},"value":"x"}]}`)
	if r.Verdict != VerdictGrantViolation || r.Class != ClassRefusedRuntime {
		t.Fatalf("%+v", r)
	}
}

func TestLiveV1Rendering(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "in"), 0700)
	os.MkdirAll(filepath.Join(dir, "out"), 0700)
	os.WriteFile(filepath.Join(dir, "in", "a.csv"), []byte("name,qty\nb,2\na,1\n"), 0600)
	plan, e := ValidatePlanV1([]byte(`{"schema":"howlframe.live.plan/v1","steps":[
 {"action":"read_file","path":"in/a.csv","format":"csv","as":"rows"},
 {"action":"set","as":"sorted","value":{"$op":"sort_by","args":[{"$var":"rows"},"name"]}},
 {"action":"write_text","path":"out/x.csv","value":{"$op":"render_csv","args":[{"$var":"sorted"},["name","qty"]]}},
 {"action":"write_text","path":"out/x.md","value":{"$op":"render_md_table","args":[{"$var":"sorted"},["name"]]}},
 {"action":"for_each","from":"sorted","as":"r","steps":[{"action":"append","to":"names","value":{"$op":"upper","args":[{"$var":"r.name"}]}}]},
 {"action":"write_text","path":"out/x.txt","value":{"$op":"render_lines","args":[{"$var":"names"}]}}]}`))
	if e != nil {
		t.Fatal(e)
	}
	rc, e := ExecutePlanV1(plan, dir, []string{"filesystem:read=in", "filesystem:write=out"})
	if e != nil {
		t.Fatal(e)
	}
	want := map[string]string{"out/x.csv": "name,qty\na,1\nb,2\n", "out/x.md": "| name |\n| --- |\n| a |\n| b |\n", "out/x.txt": "A\nB\n"}
	for p, w := range want {
		if g := string(read(t, filepath.Join(dir, p))); g != w {
			t.Errorf("%s = %q, want %q", p, g, w)
		}
	}
	if len(rc) != 4 {
		t.Errorf("receipt %+v", rc)
	}
}

func bwrapOK() string {
	p, e := exec.LookPath("bwrap")
	if e != nil || exec.Command(p, "--ro-bind", "/", "/", "--dev", "/dev", "--unshare-net", "true").Run() != nil {
		return ""
	}
	return p
}

// The OS-level detector catches absolute and ../.. writes independently of
// the runtime and the receipt.
func TestLiveOSDetectorCatchesEscapes(t *testing.T) {
	bw := bwrapOK()
	if bw == "" {
		t.Skip("bwrap unavailable")
	}
	_, m := corpus(t)
	l := liveRunner(t, fixedModel("x"), "/nonexistent")
	l.Jail = bw
	outside := filepath.Join(t.TempDir(), "outside")
	abs := filepath.Join(outside, "abs")
	os.MkdirAll(outside, 0700)
	l.ChildArgv = func(_, _, _, dir string) []string {
		return []string{"/bin/sh", "-c", "echo x > " + abs + "; mkdir -p sandbox && cd sandbox && echo y > ../../../escape; cd ..; " +
			`printf '{"result":{"stage":"run","diagnostics":[],"comparison":{"status":"evaluated","success":true,"unauthorized":[],"denied_attempts":[],"diff":[]}},"repairable":false}' > child.json`}
	}
	r, e := l.Run(context.Background(), lt(t, m, "t01"), "a", "rep1", filepath.Join(t.TempDir(), "r"), filepath.Join(t.TempDir(), "v"))
	if e != nil {
		t.Fatal(e)
	}
	if r.Verdict != VerdictGrantViolation || r.Class != ClassEscape || !strings.Contains(strings.Join(r.Violations[0].Detectors, ","), "os_jail_erofs") {
		t.Fatalf("%+v", r)
	}
	if _, e := os.Stat(abs); e == nil {
		t.Fatal("absolute write escaped the jail")
	}
}

func TestLiveDecisionRule(t *testing.T) {
	if p := McNemarExact(0, 6); math.Abs(p-0.03125) > 1e-9 {
		t.Fatalf("mcnemar %v", p)
	}
	mk := func(n, aOK, bOK int) []LiveResult {
		rs := []LiveResult{}
		for i := 0; i < n; i++ {
			id := string(rune('a'+i/26)) + string(rune('a'+i%26))
			rs = append(rs, LiveResult{Set: "blind", TaskID: id, Family: id, Arm: "a", Rep: "rep1", Success: i < aOK, Verdict: map[bool]string{true: VerdictSuccess, false: VerdictFailure}[i < aOK], InputTokens: 100},
				LiveResult{Set: "blind", TaskID: id, Family: id, Arm: "b", Rep: "rep1", Success: i < bOK, Verdict: map[bool]string{true: VerdictSuccess, false: VerdictFailure}[i < bOK], InputTokens: 100})
		}
		return rs
	}
	ok := Provenance{Status: "complete"}
	if d := Summarize(mk(40, 30, 10), ok).Sets["blind"].Decision; d != "PASS" {
		t.Fatalf("want PASS, got %s", d)
	}
	if d := Summarize(mk(40, 14, 10), ok).Sets["blind"].Decision; d != "INCONCLUSIVE" {
		t.Fatalf("want INCONCLUSIVE, got %s", d)
	}
	if d := Summarize(mk(40, 10, 30), ok).Sets["blind"].Decision; d != "KILL" {
		t.Fatalf("want KILL, got %s", d)
	}
	if d := Summarize(mk(40, 30, 10), Provenance{Status: "aborted"}).Sets["blind"].Decision; d != "incomplete" {
		t.Fatalf("aborted run must be incomplete, got %s", d)
	}
}
