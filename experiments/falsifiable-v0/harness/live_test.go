package harness

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestLiveChildHelper is the attempt child when re-executed by liveRunner.
func TestLiveChildHelper(t *testing.T) {
	i := 0
	for i < len(os.Args) && os.Args[i] != "--" {
		i++
	}
	if i >= len(os.Args)-6 {
		t.Skip("helper only")
	}
	a := os.Args[i+1:]
	_, m := corpus(t)
	task := taskFor(t, m, a[1])
	b := read(t, a[2])
	if e := RunChildAttempt(Runner{Root: a[4], HowlFrame: a[5]}, task, a[0], b, a[3]); e != nil {
		t.Fatal(e)
	}
}

type countingModel struct {
	inner Model
	calls *int
}

func (c countingModel) Name() string { return c.inner.Name() }
func (c countingModel) Generate(ctx context.Context, p Prompt) (Completion, error) {
	*c.calls++
	return c.inner.Generate(ctx, p)
}

type fixedModel string

func (fixedModel) Name() string { return "fixed" }
func (f fixedModel) Generate(context.Context, Prompt) (Completion, error) {
	return Completion{Text: string(f), InputTokens: 1, OutputTokens: 1}, nil
}

func liveRunner(t *testing.T, m Model, hf string) LiveRunner {
	root, _ := corpus(t)
	live, _ := filepath.Abs("../../live-v1")
	ps, e := LoadPromptSet(live, root)
	if e != nil {
		t.Fatal(e)
	}
	return LiveRunner{Root: root, HowlFrame: hf, Model: m, Prompts: ps, Deadline: 20 * time.Second,
		ChildArgv: func(arm, task, cand, dir string) []string {
			return []string{os.Args[0], "-test.run=^TestLiveChildHelper$", "--", arm, task, cand, dir, root, hf}
		}}
}

func TestLiveArmBGovernedReceipts(t *testing.T) {
	root, m := corpus(t)
	calls := 0
	l := liveRunner(t, countingModel{StubModel{Root: root}, &calls}, "/nonexistent")
	out := filepath.Join(t.TempDir(), "r")
	vlog := filepath.Join(t.TempDir(), "v.jsonl")
	r, e := l.Run(context.Background(), taskFor(t, m, "t01"), "b", "seed1", out, vlog)
	if e != nil || r.Verdict != VerdictSuccess || calls != 1 {
		t.Fatalf("%v %+v calls=%d", e, r, calls)
	}
	rc, e := ReadReceipt(filepath.Join(out, "attempt0", "receipt.json"))
	if e != nil || len(rc.Effects) == 0 {
		t.Fatalf("arm B must write a howlframe.receipt/v0 receipt: %v %+v", e, rc)
	}
	sawWrite := false
	for _, x := range rc.Effects {
		if x.Decision != "allowed" || x.Capability != "filesystem" {
			t.Fatalf("unexpected receipt entry %+v", x)
		}
		sawWrite = sawWrite || (x.Op == "WRITE_FILE" && x.Target == "out/release.json")
	}
	if !sawWrite {
		t.Fatal("receipt missing the write")
	}
}

func TestLiveGrantViolationIsFinalAndLogged(t *testing.T) {
	root, m := corpus(t)
	for _, tc := range []struct{ arm, fixture string }{{"b", "escape.plan.json"}, {"b", "fetch.plan.json"}} {
		calls := 0
		l := liveRunner(t, countingModel{fixedModel("```\n" + string(read(t, filepath.Join(root, "fixtures", tc.fixture))) + "```\n"), &calls}, "/nonexistent")
		out := filepath.Join(t.TempDir(), "r")
		vlog := filepath.Join(t.TempDir(), "v.jsonl")
		r, e := l.Run(context.Background(), taskFor(t, m, "a01"), tc.arm, "seed1", out, vlog)
		if e != nil {
			t.Fatal(e)
		}
		if r.Verdict != VerdictGrantViolation || r.Success || calls != 1 || len(r.Attempts) != 1 {
			t.Fatalf("%s: violation must fail without retry: %+v calls=%d", tc.fixture, r, calls)
		}
		if _, e := os.Stat(filepath.Join(out, "attempt0", "sandbox", "..", "canary", "x")); e == nil {
			t.Fatal("denied write was performed")
		}
		f, _ := os.Open(vlog)
		n := 0
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			n++
			if !strings.Contains(sc.Text(), `"decision":"denied"`) {
				t.Fatalf("violation must come from receipt: %s", sc.Text())
			}
		}
		f.Close()
		if n != 1 {
			t.Fatalf("want 1 logged violation, got %d", n)
		}
	}
}

func TestLiveArmACapsViolationFinal(t *testing.T) {
	root, m := corpus(t)
	bin := buildHowlFrameBinaryForTest(t)
	calls := 0
	l := liveRunner(t, countingModel{StubModel{Root: root, ViolateIDs: map[string]bool{"a02": true}}, &calls}, bin)
	r, e := l.Run(context.Background(), taskFor(t, m, "a02"), "a", "seed1", filepath.Join(t.TempDir(), "r"), filepath.Join(t.TempDir(), "v"))
	if e != nil || r.Verdict != VerdictGrantViolation || calls != 1 || len(r.Violations) != 1 || !strings.Contains(strings.Join(r.Violations[0].Diagnostics, ""), "CAPS_PREFLIGHT_DENIED") {
		t.Fatalf("%v %+v", e, r)
	}
	calls = 0
	l.Model = countingModel{StubModel{Root: root, RepairIDs: map[string]bool{"t01": true}}, &calls}
	r, e = l.Run(context.Background(), taskFor(t, m, "t01"), "a", "seed1", filepath.Join(t.TempDir(), "r"), filepath.Join(t.TempDir(), "v"))
	if e != nil || r.Verdict != VerdictSuccess || r.RepairRounds != 1 || calls != 2 {
		t.Fatalf("repair path: %v %+v", e, r)
	}
}

func TestLiveRepairBudget(t *testing.T) {
	_, m := corpus(t)
	calls := 0
	l := liveRunner(t, countingModel{fixedModel("not json"), &calls}, "/nonexistent")
	r, e := l.Run(context.Background(), taskFor(t, m, "t01"), "b", "seed1", filepath.Join(t.TempDir(), "r"), filepath.Join(t.TempDir(), "v"))
	if e != nil || r.Verdict != VerdictRepairsSpent || calls != LiveMaxRepairs+1 || r.RepairRounds != LiveMaxRepairs {
		t.Fatalf("%v %+v calls=%d", e, r, calls)
	}
}

func TestLiveDeadlineKillsProcessGroup(t *testing.T) {
	_, m := corpus(t)
	l := liveRunner(t, fixedModel("x"), "/nonexistent")
	l.Deadline = time.Second
	pidfile := filepath.Join(t.TempDir(), "pid")
	l.ChildArgv = func(string, string, string, string) []string {
		return []string{"/bin/sh", "-c", "sleep 60 & echo $! > " + pidfile + "; wait"}
	}
	start := time.Now()
	r, e := l.Run(context.Background(), taskFor(t, m, "t01"), "a", "seed1", filepath.Join(t.TempDir(), "r"), filepath.Join(t.TempDir(), "v"))
	if e != nil || r.Verdict != VerdictDeadlineKilled || r.Success || len(r.Attempts) != 1 {
		t.Fatalf("%v %+v", e, r)
	}
	if el := time.Since(start); el > 10*time.Second {
		t.Fatalf("not killed promptly: %s", el)
	}
	b := strings.TrimSpace(string(read(t, pidfile)))
	var pid int
	for _, c := range b {
		pid = pid*10 + int(c-'0')
	}
	time.Sleep(100 * time.Millisecond)
	if syscall.Kill(pid, 0) == nil {
		t.Fatalf("grandchild %d survived the deadline kill", pid)
	}
	l.Deadline = LiveMaxDeadline + time.Second
	if _, e := l.Run(context.Background(), taskFor(t, m, "t01"), "a", "seed1", filepath.Join(t.TempDir(), "r2"), filepath.Join(t.TempDir(), "v")); e == nil {
		t.Fatal("deadline above the 30s cap must be refused")
	}
}

func TestLiveReportShowsHowlFrameLosses(t *testing.T) {
	rs := []LiveResult{
		{TaskID: "t01", Seed: "seed1", Arm: "a", Verdict: VerdictFailure},
		{TaskID: "t01", Seed: "seed1", Arm: "b", Verdict: VerdictSuccess, Success: true},
		{TaskID: "t02", Seed: "seed1", Arm: "a", Verdict: VerdictGrantViolation},
		{TaskID: "t02", Seed: "seed1", Arm: "b", Verdict: VerdictSuccess, Success: true},
	}
	s := Summarize(rs)
	if s.Verdicts["success_rate"] != "howlframe_worse" || s.Verdicts["grant_violations"] != "howlframe_worse" || len(s.Losses) != 2 {
		t.Fatalf("%+v", s)
	}
	rep := RenderReport(s)
	for _, want := range []string{"Where HowlFrame lost (B succeeded, A did not): 2", "t01/seed1", "t02/seed1", "success_rate: howlframe_worse", "| success rate | 0.0% (0) | 100.0% (2) |"} {
		if !strings.Contains(rep, want) {
			t.Fatalf("report missing %q:\n%s", want, rep)
		}
	}
}

func TestLiveReviewEffortAndExtraction(t *testing.T) {
	c := ExtractCandidate("prose\n```lisp\n(cli_app\n\n  (do \"a b\"))\n```\nmore")
	if string(c) != "(cli_app\n\n  (do \"a b\"))\n" {
		t.Fatalf("%q", c)
	}
	r := MeasureReview(c)
	if r.Lines != 2 || r.Tokens != 7 {
		t.Fatalf("%+v", r)
	}
}
