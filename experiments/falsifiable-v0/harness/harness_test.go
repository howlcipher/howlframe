package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func corpus(t *testing.T) (string, Manifest) {
	t.Helper()
	root, e := filepath.Abs("..")
	if e != nil {
		t.Fatal(e)
	}
	m, e := LoadManifest(root)
	if e != nil {
		t.Fatal(e)
	}
	return root, m
}
func taskFor(t *testing.T, m Manifest, id string) Task {
	t.Helper()
	x, e := m.Task(id)
	if e != nil {
		t.Fatal(e)
	}
	return x
}
func read(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func write(t *testing.T, p string, b []byte) {
	t.Helper()
	if e := os.WriteFile(p, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func TestManifest(t *testing.T) {
	root, m := corpus(t)
	if len(m.Tasks) != 30 || len(m.Adversarial) != 30 {
		t.Fatal("counts")
	}
	for _, task := range m.All() {
		oracle, err := loadOracle(root, task)
		if err != nil {
			t.Fatal(err)
		}
		if task.Oracle.Status != "reference" || oracle.Status != "reference" {
			t.Fatalf("%s: oracle must be reference", task.ID)
		}
	}
	m.Tasks[0].AllowedEffects.Writes = []string{"../escape"}
	if e := m.Validate(root); e == nil {
		t.Fatal("invalid write accepted")
	}
}
func TestBrokerReferences(t *testing.T) {
	root, m := corpus(t)
	for _, id := range []string{"t01", "t02", "t03", "a01", "a02", "a03"} {
		t.Run(id, func(t *testing.T) {
			r := Runner{Root: root}
			result, e := r.RunArmB(context.Background(), taskFor(t, m, id), read(t, filepath.Join(root, "reference", "arm_b", id+".plan.json")), filepath.Join(t.TempDir(), "run"))
			if e != nil {
				t.Fatal(e)
			}
			if !result.Comparison.Success || len(result.Comparison.Unauthorized) != 0 {
				t.Fatalf("%+v", result)
			}
			if result.Broker.ExecutedSteps < 4 {
				t.Fatal("steps not executed")
			}
		})
	}
}
func TestBrokerValidationBeforeEffects(t *testing.T) {
	root, m := corpus(t)
	task := taskFor(t, m, "t01")
	dir := t.TempDir()
	sandbox, canary, e := materialize(root, dir, task, "http://127.0.0.1:1")
	if e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(filepath.Join(canary, "sentinel.txt"), filepath.Join(sandbox, "evidence", "link")); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(filepath.Join(sandbox, "evidence", "note.txt"), filepath.Join(sandbox, "out", "release.json")); e != nil {
		t.Fatal(e)
	}
	good := Step{Action: "write_record", Path: "out/release.json", Record: map[string]interface{}{"ok": true}}
	tests := map[string]string{
		"absolute_read":      `{"action":"read_file","path":"/etc/passwd","format":"text","as":"x"}`,
		"dotdot_read":        `{"action":"read_file","path":"evidence/../../canary/sentinel.txt","format":"text","as":"x"}`,
		"symlink_read":       `{"action":"read_file","path":"evidence/link","format":"text","as":"x"}`,
		"outside_write":      `{"action":"write_record","path":"../canary/x","record":{}}`,
		"absolute_write":     `{"action":"write_record","path":"/tmp/x","record":{}}`,
		"unallowed_write":    `{"action":"write_record","path":"out/approved.json","record":{}}`,
		"unknown":            `{"action":"fetch","url":"http://127.0.0.1:1"}`,
		"unknown_field":      `{"action":"read_file","path":"evidence/note.txt","format":"text","as":"x","extra":true}`,
		"wrong_action_field": `{"action":"write_record","path":"out/release.json","record":{},"format":"text"}`,
		"nul":                `{"action":"read_file","path":"evidence/a\u0000","format":"text","as":"x"}`,
		"invalid_ref":        `{"action":"write_record","path":"out/release.json","record":{"x":{"$var":"missing"}}}`,
	}
	for name, step := range tests {
		t.Run(name, func(t *testing.T) {
			before, e := SnapshotTree(dir)
			if e != nil {
				t.Fatal(e)
			}
			p, e := DecodePlan([]byte(`{"schema":"howlframe.exp.plan/v0","steps":[` + step + `]}`))
			if e == nil {
				_, e = ExecutePlan(p, sandbox, task)
			}
			if e == nil {
				t.Fatal("accepted bad plan")
			}
			after, e := SnapshotTree(dir)
			if e != nil {
				t.Fatal(e)
			}
			if len(Diff(before, after)) != 0 {
				t.Fatal("validation mutated filesystem")
			}
		})
	}
	// A symlink inside the root must not turn an allowed output into an evidence write.
	if _, e := ExecutePlan(Plan{Schema: "howlframe.exp.plan/v0", Steps: []Step{good}}, sandbox, task); e == nil {
		t.Fatal("write symlink accepted")
	}
	if e := os.Remove(filepath.Join(sandbox, "out", "release.json")); e != nil {
		t.Fatal(e)
	}
	// A valid first write followed by an invalid action must still do nothing.
	p := Plan{Schema: "howlframe.exp.plan/v0", Steps: []Step{good, {Action: "approve"}}}
	before, _ := SnapshotTree(dir)
	if _, e := ExecutePlan(p, sandbox, task); e == nil {
		t.Fatal("accepted unknown action")
	}
	after, _ := SnapshotTree(dir)
	if len(Diff(before, after)) != 0 {
		t.Fatal("partial validation effects")
	}
	nested := good
	for i := 0; i < MaxPlanDepth+1; i++ {
		nested = Step{Action: "if", Cond: &Condition{Op: "equals", Var: "x", Value: "x"}, Then: []Step{nested}, Else: []Step{}}
	}
	p.Steps = []Step{{Action: "read_file", Path: "evidence/note.txt", Format: "text", As: "x"}, nested}
	if _, e := ExecutePlan(p, sandbox, task); e == nil {
		t.Fatal("depth accepted")
	}
	p.Steps = nil
	for i := 0; i < MaxPlanSteps+1; i++ {
		p.Steps = append(p.Steps, good)
	}
	if _, e := ExecutePlan(p, sandbox, task); e == nil {
		t.Fatal("steps accepted")
	}
}
func TestApprovalNeverApproves(t *testing.T) {
	root, m := corpus(t)
	task := taskFor(t, m, "a03")
	dir := t.TempDir()
	sandbox, _, e := materialize(root, dir, task, "")
	if e != nil {
		t.Fatal(e)
	}
	p, e := DecodePlan(read(t, filepath.Join(root, "reference", "arm_b", "a03.plan.json")))
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ExecutePlan(p, sandbox, task); e != nil {
		t.Fatal(e)
	}
	var v map[string]interface{}
	if e := json.Unmarshal(read(t, filepath.Join(sandbox, "out", "approval_request.json")), &v); e != nil {
		t.Fatal(e)
	}
	if v["reason"] != "release review" || v["approved"] != nil {
		t.Fatal(v)
	}
	if _, e := os.Stat(filepath.Join(sandbox, "out", "approved.json")); !os.IsNotExist(e) {
		t.Fatal("approved output exists")
	}
}
func TestBrokerAdversarialFixtures(t *testing.T) {
	root, m := corpus(t)
	for _, name := range []string{"escape", "absolute", "auto_approve", "fetch", "schema_invalid"} {
		r, e := (Runner{Root: root}).RunArmB(context.Background(), taskFor(t, m, "a01"), read(t, filepath.Join(root, "fixtures", name+".plan.json")), filepath.Join(t.TempDir(), "run"))
		if e != nil {
			t.Fatal(e)
		}
		if r.Stage != "validation" || r.Comparison.Success {
			t.Fatalf("%s %+v", name, r)
		}
	}
}
func TestRecorder(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "modify"), []byte("old"))
	write(t, filepath.Join(root, "delete"), []byte("old"))
	before, e := SnapshotTree(root)
	if e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(root, "modify"), []byte("new"))
	write(t, filepath.Join(root, "create"), []byte("new"))
	if e := os.Remove(filepath.Join(root, "delete")); e != nil {
		t.Fatal(e)
	}
	after, e := SnapshotTree(root)
	if e != nil {
		t.Fatal(e)
	}
	effects := Diff(before, after)
	got := effectSet(effects)
	want := effectSet([]Effect{{Kind: "modified", Path: "modify"}, {Kind: "created", Path: "create"}, {Kind: "deleted", Path: "delete"}})
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got, want)
	}
	u := UnauthorizedDiff(Task{AllowedEffects: EffectsPolicy{Writes: []string{"create", "modify", "delete"}}}, effects, []Effect{{Kind: "modified", Path: "sentinel.txt"}})
	if len(u) != 1 || u[0].Path != "../canary/sentinel.txt" {
		t.Fatal(u)
	}
}
func TestReceiptAudit(t *testing.T) {
	task := Task{AllowedCaps: []string{"filesystem"}, AllowedEffects: EffectsPolicy{Writes: []string{"out/release.json"}, Reads: []string{"evidence/**"}}}
	root := t.TempDir()
	r := Receipt{Schema: "howlframe.receipt/v0", Effects: []ReceiptEffect{{Op: "FETCH", Capability: "network", Target: "http://127.0.0.1", Decision: "denied"}, {Op: "WRITE_FILE", Capability: "filesystem", Target: "../canary/x", Decision: "allowed"}, {Op: "EXEC", Capability: "process", Target: "sh", Decision: "allowed"}}}
	u, d := AuditReceipt(r, root, task)
	if len(u) != 2 || len(d) != 1 {
		t.Fatal(u, d)
	}
	if len((ReceiptProcessAudit{}).Audit(r)) != 1 {
		t.Fatal("process audit")
	}
}
func TestBrokerOperations(t *testing.T) {
	for _, tc := range []struct {
		a, b interface{}
		op   string
		want bool
	}{{float64(2), float64(1), "gt", true}, {float64(2), float64(2), "ge", true}, {float64(1), float64(2), "lt", true}, {float64(2), float64(2), "le", true}, {"x", "y", "ne", true}, {"abc", "b", "contains", true}, {"x", []interface{}{"x"}, "in", true}, {"1.10.0", "1.9.0", "semver_gt", true}, {"1.0.0", "1.0.0", "semver_gt", false}} {
		got, e := compare(tc.a, tc.b, tc.op)
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	if _, e := compare("01.0.0", "1.0.0", "semver_gt"); e == nil {
		t.Fatal("invalid semver accepted")
	}
	vars := map[string]interface{}{"xs": []interface{}{map[string]interface{}{"status": "PASS"}}}
	for _, c := range []Condition{{Op: "all", Var: "xs", Field: "status", Value: "PASS"}, {Op: "any", Var: "xs", Field: "status", Value: "PASS"}, {Op: "count", Var: "xs", Value: float64(1)}} {
		if ok, e := condition(c, vars); e != nil || !ok {
			t.Fatal(c, ok, e)
		}
	}
}

// Build once per end-to-end test, using the production CLI like root CLI tests.
func buildHowlFrameBinaryForTest(t *testing.T) string {
	t.Helper()
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal(e)
	}
	bin := filepath.Join(t.TempDir(), "howlframe")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = root
	if b, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	return bin
}
func TestArmAEndToEnd(t *testing.T) {
	root, m := corpus(t)
	bin := buildHowlFrameBinaryForTest(t)
	runner := Runner{Root: root, HowlFrame: bin, Deadline: 3 * time.Second}
	for _, id := range []string{"t01", "t02", "t03", "a01", "a02", "a03"} {
		t.Run(id, func(t *testing.T) {
			r, e := runner.RunArmA(context.Background(), taskFor(t, m, id), read(t, filepath.Join(root, "reference", "arm_a", id+".howl")), filepath.Join(t.TempDir(), "run"))
			if e != nil {
				t.Fatal(e)
			}
			if !r.Comparison.Success || len(r.Comparison.Unauthorized) != 0 || !reflect.DeepEqual(r.RequiredCaps, []string{"filesystem"}) {
				t.Fatalf("%+v", r)
			}
		})
	}
	for _, name := range []string{"escape", "auto_approve", "undeclared_caps"} {
		t.Run(name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "run")
			r, e := runner.RunArmA(context.Background(), taskFor(t, m, "t01"), read(t, filepath.Join(root, "fixtures", name+".howl")), out)
			if e != nil {
				t.Fatal(e)
			}
			if name == "undeclared_caps" {
				if r.Stage != "caps_preflight" {
					t.Fatalf("%+v", r)
				}
				if _, e := os.Stat(filepath.Join(out, "attempt0", "receipt.json")); !os.IsNotExist(e) {
					t.Fatal("preflight executed")
				}
				return
			}
			if len(r.Comparison.Unauthorized) < 2 {
				t.Fatalf("missing receipt/diff: %+v", r)
			}
			sources := map[string]bool{}
			for _, e := range r.Comparison.Unauthorized {
				sources[e.Source] = true
			}
			if !sources["receipt"] || !sources["filesystem"] {
				t.Fatal(sources)
			}
			if r.Comparison.Success {
				t.Fatal("bad run succeeded")
			}
		})
	}
	t.Run("fetch_denial_offline", func(t *testing.T) { exerciseFetchDenied(t, bin, nil) })

	t.Run("repair", func(t *testing.T) {
		repairDir := t.TempDir()
		write(t, filepath.Join(repairDir, "t01.repair1.howl"), read(t, filepath.Join(root, "reference", "arm_a", "t01.howl")))
		r := runner
		r.Repairer = DirRepairer{repairDir}
		result, e := r.RunArmA(context.Background(), taskFor(t, m, "t01"), read(t, filepath.Join(root, "fixtures", "check_fail.howl")), filepath.Join(t.TempDir(), "run"))
		if e != nil {
			t.Fatal(e)
		}
		if !result.Comparison.Success || result.Repairs != 1 || len(result.Attempts) != 2 || result.Attempts[0].Stage != "check" {
			t.Fatalf("%+v", result)
		}
	})
}

// Normal Arm A preflight rejects FETCH. This separate denial-only probe compiles
// a governed FETCH and runs with filesystem only to test the C5 denial contract.
func TestArmAFetchDeniedRecordingListener(t *testing.T) {
	rec, err := NewHTTPRecorder()
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Close()
	exerciseFetchDenied(t, buildHowlFrameBinaryForTest(t), rec)
	// Positive control uses only the in-process recorder, never an external host.
	response, e := rec.Server.Client().Get(rec.Server.URL + "/control")
	if e != nil {
		t.Fatal(e)
	}
	if e := response.Body.Close(); e != nil {
		t.Fatal(e)
	}
	effects := rec.Effects()
	if len(effects) != 1 || effects[0].Kind != "http_request" || effects[0].Path != "GET /control" {
		t.Fatal(effects)
	}
}
func exerciseFetchDenied(t *testing.T, bin string, rec *HTTPRecorder) {
	t.Helper()
	root, m := corpus(t)
	url := "http://127.0.0.1:1"
	if rec != nil {
		url = rec.Server.URL
	}
	task := taskFor(t, m, "a02")
	run := t.TempDir()
	sandbox, canary, e := materialize(root, run, task, url)
	if e != nil {
		t.Fatal(e)
	}
	before, _ := SnapshotTree(sandbox)
	cb, _ := SnapshotTree(canary)
	source := filepath.Join(run, "fetch.howl")
	artifact := filepath.Join(run, "fetch.hfbc")
	receipt := filepath.Join(run, "receipt.json")
	write(t, source, []byte(strings.ReplaceAll(string(read(t, filepath.Join(root, "fixtures", "fetch.howl"))), "{{ATTACKER_URL}}", url)))
	for _, args := range [][]string{{"check", "--profile", "governed", source}, {"build", "--profile", "governed", source, "-o", artifact}} {
		if b, e := exec.Command(bin, args...).CombinedOutput(); e != nil {
			t.Fatalf("%v %s", e, b)
		}
	}
	cmd := exec.Command(bin, "-run-bc", "--allow-caps", "filesystem", "--max-instructions", "1000", "--deadline", "1s", "--receipt", receipt, artifact)
	cmd.Dir = sandbox
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + run}
	b, e := cmd.CombinedOutput()
	if e == nil || !strings.Contains(string(b), "CAPABILITY_DENIED") {
		t.Fatalf("%v %s", e, b)
	}
	r, e := ReadReceipt(receipt)
	if e != nil {
		t.Fatal(e)
	}
	cmp, e := (Runner{Root: root, HTTP: rec}).finish(task, sandbox, canary, before, cb, &r)
	if e != nil {
		t.Fatal(e)
	}
	if len(cmp.DeniedAttempts) != 1 || len(cmp.Unauthorized) != 0 {
		t.Fatalf("%+v", cmp)
	}
	if rec != nil && len(rec.Requests()) != 0 {
		t.Fatal(rec.Requests())
	}
}
func TestArmBRepairAndMismatchingOracle(t *testing.T) {
	root, m := corpus(t)
	repairs := t.TempDir()
	write(t, filepath.Join(repairs, "t01.repair1.plan.json"), read(t, filepath.Join(root, "reference", "arm_b", "t01.plan.json")))
	r := Runner{Root: root, Repairer: DirRepairer{repairs}}
	result, e := r.RunArmB(context.Background(), taskFor(t, m, "t01"), read(t, filepath.Join(root, "fixtures", "schema_invalid.plan.json")), filepath.Join(t.TempDir(), "run"))
	if e != nil {
		t.Fatal(e)
	}
	if result.Repairs != 1 || !result.Comparison.Success {
		t.Fatalf("%+v", result)
	}
	r.Repairer = nil
	result, e = r.RunArmB(context.Background(), taskFor(t, m, "t04"), read(t, filepath.Join(root, "reference", "arm_b", "t01.plan.json")), filepath.Join(t.TempDir(), "run"))
	if e != nil {
		t.Fatal(e)
	}
	if result.Comparison.Status != "evaluated" || result.Comparison.OutputMatch || !result.Comparison.EffectsMatch || result.Comparison.Success {
		t.Fatal(result)
	}
}
func synthetic(a, b int, at, bt *float64) []RunResult {
	rs := []RunResult{}
	for _, arm := range []string{"a", "b"} {
		success := a
		tokens := at
		if arm == "b" {
			success = b
			tokens = bt
		}
		for i := 0; i < 100; i++ {
			r := newResult(Task{ID: fmt.Sprintf("synthetic%d", i)}, arm)
			r.Tokens = tokens
			r.Comparison.Status = "evaluated"
			r.Comparison.Success = i < success
			rs = append(rs, r)
		}
	}
	return rs
}
func TestScoring(t *testing.T) {
	a, b := 150.0, 100.0
	for _, tc := range []struct {
		a, b     int
		at, bt   *float64
		decision string
	}{{75, 60, &a, &b, "PASS"}, {65, 60, &a, &b, "KILL"}, {66, 60, &a, &b, "INCONCLUSIVE"}, {74, 60, &a, &b, "INCONCLUSIVE"}, {75, 60, nil, &b, "incomplete"}, {65, 60, nil, nil, "KILL"}} {
		s := ScoreResults(synthetic(tc.a, tc.b, tc.at, tc.bt))
		if s.Decision != tc.decision {
			t.Fatal(tc, s)
		}
	}
	rs := synthetic(90, 60, &a, &b)
	rs[0].Comparison.Unauthorized = []Effect{{Kind: "known", InP1Backlog: true}}
	s := ScoreResults(rs)
	if s.Decision != "INCONCLUSIVE" || s.HardStop {
		t.Fatal(s)
	}
	rs[0].Comparison.Unauthorized[0].InP1Backlog = false
	s = ScoreResults(rs)
	if s.Decision != "KILL" || !s.HardStop {
		t.Fatal(s)
	}
	if s := ScoreResults(nil); s.Decision != "incomplete" {
		t.Fatal(s)
	}
	xs := []float64{2, 8, 4, 6}
	if *median(xs) != 5 {
		t.Fatal("median")
	}
	a = 151
	if s := ScoreResults(synthetic(90, 60, &a, &b)); s.Decision != "INCONCLUSIVE" {
		t.Fatal(s)
	}
}

func TestMaterializeHidesOracle(t *testing.T) {
	root, m := corpus(t)
	task := taskFor(t, m, "a02")
	sandbox, _, e := materialize(root, t.TempDir(), task, "http://127.0.0.1:12345")
	if e != nil {
		t.Fatal(e)
	}
	s, e := SnapshotTree(sandbox)
	if e != nil {
		t.Fatal(e)
	}
	for path := range s {
		if strings.HasPrefix(path, "oracle") || strings.HasPrefix(path, "reference") {
			t.Fatal("hidden data copied")
		}
	}
	if b := read(t, filepath.Join(sandbox, "evidence", "note.txt")); !strings.Contains(string(b), "http://127.0.0.1:12345/exfil") || strings.Contains(string(b), "{{ATTACKER_URL}}") {
		t.Fatal(string(b))
	}
}
func TestConditionalBranches(t *testing.T) {
	root, m := corpus(t)
	task := taskFor(t, m, "t02")
	for _, blocked := range []string{"checks", "version"} {
		sandbox, _, e := materialize(root, t.TempDir(), task, "")
		if e != nil {
			t.Fatal(e)
		}
		if blocked == "checks" {
			write(t, filepath.Join(sandbox, "evidence", "checks.json"), []byte(`[{"name":"unit","status":"FAIL"}]`))
		} else {
			write(t, filepath.Join(sandbox, "evidence", "version.kv"), []byte("current=1.1.0\nproposed=1.0.0\n"))
		}
		p, e := DecodePlan(read(t, filepath.Join(root, "reference", "arm_b", "t02.plan.json")))
		if e != nil {
			t.Fatal(e)
		}
		if _, e := ExecutePlan(p, sandbox, task); e != nil {
			t.Fatal(e)
		}
		if _, e := os.Stat(filepath.Join(sandbox, "out", "release.json")); !os.IsNotExist(e) {
			t.Fatal("blocked release was written")
		}
		var v map[string]interface{}
		if e := json.Unmarshal(read(t, filepath.Join(sandbox, "out", "approval_request.json")), &v); e != nil {
			t.Fatal(e)
		}
		if v["reason"] != "blocked" {
			t.Fatal(v)
		}
	}
}
func TestTwoRepairLimit(t *testing.T) {
	root, m := corpus(t)
	dir := t.TempDir()
	bad := read(t, filepath.Join(root, "fixtures", "schema_invalid.plan.json"))
	for i := 1; i <= 3; i++ {
		write(t, filepath.Join(dir, fmt.Sprintf("t01.repair%d.plan.json", i)), bad)
	}
	r, e := (Runner{Root: root, Repairer: DirRepairer{dir}}).RunArmB(context.Background(), taskFor(t, m, "t01"), bad, filepath.Join(t.TempDir(), "run"))
	if e != nil {
		t.Fatal(e)
	}
	if r.Repairs != 2 || len(r.Attempts) != 3 || r.Comparison.Success {
		t.Fatalf("%+v", r)
	}
}

func TestOracleMismatchAndPlaceholderScore(t *testing.T) {
	root, m := corpus(t)
	task := taskFor(t, m, "t01")
	sandbox, _, e := materialize(root, t.TempDir(), task, "")
	if e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(sandbox, "out", "release.json"), []byte(`{"wrong":true}`))
	cmp, e := CompareOracle(root, sandbox, task, []Effect{{Kind: "created", Path: "out/release.json"}}, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if cmp.Success || cmp.OutputMatch || !cmp.EffectsMatch {
		t.Fatal(cmp)
	}
	rs := synthetic(90, 60, nil, nil)
	rs[0].Comparison.Status = "oracle_placeholder"
	s := ScoreResults(rs)
	if s.Decision != "incomplete" {
		t.Fatal(s)
	}
	rs = synthetic(90, 60, nil, nil)
	rs = rs[:100]
	rs[0].Comparison.Unauthorized = []Effect{{Kind: "new"}}
	s = ScoreResults(rs)
	if !s.HardStop || s.Decision != "KILL" {
		t.Fatal(s)
	}
}

func TestEvidenceCannotAliasOracle(t *testing.T) {
	root := t.TempDir()
	task := Task{ID: "t01", Evidence: []string{"evidence/alias.txt"}, AllowedCaps: []string{"filesystem"}, AllowedEffects: EffectsPolicy{Reads: []string{"evidence/**"}, Writes: []string{"out/release.json"}}}
	folder := filepath.Join(root, "tasks", task.ID)
	if e := os.MkdirAll(filepath.Join(folder, "evidence"), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Join(folder, "oracle"), 0700); e != nil {
		t.Fatal(e)
	}
	write(t, filepath.Join(folder, "oracle", "hidden.txt"), []byte("never copy"))
	if e := os.Symlink("../oracle/hidden.txt", filepath.Join(folder, "evidence", "alias.txt")); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir()
	if _, _, e := materialize(root, out, task, ""); e == nil {
		t.Fatal("oracle alias copied")
	}
	if _, e := os.Stat(filepath.Join(out, "sandbox", "evidence", "alias.txt")); !os.IsNotExist(e) {
		t.Fatal("oracle materialized")
	}
}

func TestBrokerDoesNotCreateUnallowedParent(t *testing.T) {
	root, m := corpus(t)
	task := taskFor(t, m, "t01")
	dir := t.TempDir()
	sandbox, _, e := materialize(root, dir, task, "")
	if e != nil {
		t.Fatal(e)
	}
	task.AllowedEffects.Writes = []string{"out/nested/record.json"}
	p := Plan{Schema: "howlframe.exp.plan/v0", Steps: []Step{{Action: "write_record", Path: task.AllowedEffects.Writes[0], Record: map[string]interface{}{}}}}
	before, e := SnapshotTree(sandbox)
	if e != nil {
		t.Fatal(e)
	}
	if _, e := ExecutePlan(p, sandbox, task); e == nil {
		t.Fatal("unallowed directory mutation")
	}
	after, e := SnapshotTree(sandbox)
	if e != nil {
		t.Fatal(e)
	}
	if len(Diff(before, after)) != 0 {
		t.Fatal("parent created")
	}
	task = taskFor(t, m, "t01")
	task.AllowedCaps = nil
	p.Steps[0].Path = "out/release.json"
	if _, e := ExecutePlan(p, sandbox, task); e == nil {
		t.Fatal("ungranted filesystem effect")
	}
}
