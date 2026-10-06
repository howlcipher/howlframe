package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/howlcipher/howlframe/experiments/falsifiable-v0/harness"
)

func TestOfflineCLI(t *testing.T) {
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	if e := run([]string{"validate", "--root", root}); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "single")
	plan := filepath.Join(root, "reference", "arm_b", "t01.plan.json")
	if e := run([]string{"arm-b", "--root", root, "--task", "t01", "--plan", plan, "--out", out}); e != nil {
		t.Fatal(e)
	}
	rs, e := harness.LoadResults(out)
	if e != nil || len(rs) != 1 || !rs[0].Comparison.Success {
		t.Fatal(rs, e)
	}
	if e := run([]string{"arm-b", "--root", root, "--task", "t01", "--plan", plan, "--out", out}); e == nil {
		t.Fatal("reused run directory")
	}
	if e := run([]string{"score", "--results", out}); e != nil {
		t.Fatal(e)
	}
	candidates := filepath.Join(dir, "candidates")
	folder := filepath.Join(candidates, "arm_b", "t01")
	if e := os.MkdirAll(folder, 0700); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(plan)
	if e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(filepath.Join(folder, "seed1.plan.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	suite := filepath.Join(dir, "suite")
	if e := run([]string{"suite", "--root", root, "--candidates", candidates, "--howlframe", filepath.Join(dir, "unused-binary"), "--out", suite}); e != nil {
		t.Fatal(e)
	}
	b, e = os.ReadFile(filepath.Join(suite, "arm_b", "t01", "seed1", "result.json"))
	if e != nil {
		t.Fatal(e)
	}
	var result harness.RunResult
	if e := json.Unmarshal(b, &result); e != nil {
		t.Fatal(e)
	}
	if result.Seed != "seed1" || !result.Comparison.Success {
		t.Fatal(result)
	}
	if s := harness.ScoreResults([]harness.RunResult{result}); s.Decision != "incomplete" {
		t.Fatal(s)
	}
}
func TestCLIRejectsMissingInputs(t *testing.T) {
	for _, args := range [][]string{nil, {"score"}, {"bogus"}, {"arm-a"}, {"suite"}} {
		if e := run(args); e == nil {
			t.Fatal(args)
		}
	}
}
