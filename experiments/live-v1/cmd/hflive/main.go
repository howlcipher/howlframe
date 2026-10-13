// hflive drives the pre-registered live-v1 comparison. See
// experiments/live-v1/PREREGISTRATION.md. `dry-run` uses the stub model only.
// `run` contacts a real model and refuses to start without
// --seats-approved-protocol and matching pins.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/howlcipher/howlframe/experiments/falsifiable-v0/harness"
)

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "hflive:", e)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: hflive pins|dry-run|run|attempt [flags]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	root := fs.String("root", "experiments/falsifiable-v0", "falsifiable-v0 root")
	live := fs.String("live", "experiments/live-v1", "live-v1 root")
	hf := fs.String("howlframe", "", "trusted HowlFrame binary")
	out := fs.String("out", "", "new output directory")
	arm := fs.String("arm", "", "a or b (attempt)")
	task := fs.String("task", "", "task id (attempt)")
	cand := fs.String("candidate", "", "candidate file (attempt)")
	dir := fs.String("dir", "", "attempt dir (attempt)")
	deadline := fs.Duration("deadline", harness.LiveDefaultDeadline, "harness-enforced wall clock per attempt (max 30s)")
	seeds := fs.String("seeds", "seed1,seed2,seed3", "seed labels")
	model := fs.String("model", "", "claude or codex (run)")
	approved := fs.Bool("seats-approved-protocol", false, "required for run")
	if e := fs.Parse(args[1:]); e != nil {
		return e
	}
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	*root, *live = abs(*root), abs(*live)
	m, e := harness.LoadManifest(*root)
	if e != nil {
		return e
	}
	switch args[0] {
	case "attempt":
		t, e := m.Task(*task)
		if e != nil {
			return e
		}
		b, e := os.ReadFile(*cand)
		if e != nil {
			return e
		}
		r := harness.Runner{Root: *root, HowlFrame: *hf}
		return harness.RunChildAttempt(r, t, *arm, b, *dir)
	case "pins":
		p, e := harness.ComputeLivePins(*live, *root)
		if e != nil {
			return e
		}
		b, _ := json.MarshalIndent(p, "", "  ")
		fmt.Println(string(b))
		return nil
	}
	if *out == "" || *hf == "" {
		return fmt.Errorf("--out and --howlframe required")
	}
	if e := checkPins(*live, *root); e != nil {
		return e
	}
	ps, e := harness.LoadPromptSet(*live, *root)
	if e != nil {
		return e
	}
	self, e := os.Executable()
	if e != nil {
		return e
	}
	var mdl harness.Model
	seedList := strings.Split(*seeds, ",")
	switch args[0] {
	case "dry-run":
		mdl = harness.StubModel{Root: *root, RepairIDs: map[string]bool{"t01": true, "a01": true}, ViolateIDs: map[string]bool{"a02": true}}
	case "run":
		if !*approved {
			return fmt.Errorf("live runs need --seats-approved-protocol (protocol review pending)")
		}
		work := filepath.Join(*out, "model-work")
		if e := os.MkdirAll(work, 0700); e != nil {
			return e
		}
		switch *model {
		case "claude":
			mdl = harness.CLIModel{Label: "claude-code/" + harness.LiveClaudeModel, Work: work, Format: "claude-json",
				Argv: []string{"claude", "-p", "--model", harness.LiveClaudeModel, "--output-format", "json", "--tools", ""}}
		case "codex":
			mdl = harness.CLIModel{Label: "codex/" + harness.LiveCodexModel, Work: work, Format: "codex",
				Argv: []string{"codex", "exec", "-m", harness.LiveCodexModel, "-s", "read-only", "--skip-git-repo-check", "-o", "{{LAST}}", "-"}}
		default:
			return fmt.Errorf("--model must be claude or codex")
		}
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	lr := harness.LiveRunner{Root: *root, HowlFrame: abs(*hf), Self: self, Deadline: *deadline, Model: mdl, Prompts: ps}
	if _, e := os.Lstat(*out); e == nil && args[0] == "dry-run" {
		return fmt.Errorf("output directory must be new")
	}
	if e := os.MkdirAll(*out, 0700); e != nil {
		return e
	}
	vlog := filepath.Join(*out, "violations.jsonl")
	if e := os.WriteFile(vlog, nil, 0600); e != nil {
		return e
	}
	results := []harness.LiveResult{}
	ids := harness.LiveFrozenTaskIDs(m)
	for _, seed := range seedList {
		for _, id := range ids {
			t, _ := m.Task(id)
			for _, a := range []string{"a", "b"} {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				r, e := lr.Run(ctx, t, a, seed, filepath.Join(*out, "runs", a, id, seed), vlog)
				cancel()
				if e != nil {
					return fmt.Errorf("%s/%s/%s: %v", a, id, seed, e)
				}
				results = append(results, r)
				fmt.Printf("%s %s %s %s repairs=%d\n", a, id, seed, r.Verdict, r.RepairRounds)
			}
		}
	}
	s := harness.Summarize(results)
	if e := harness.WriteJSON(filepath.Join(*out, "summary.json"), s); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(*out, "report.md"), []byte(harness.RenderReport(s)), 0600)
}

func checkPins(live, root string) error {
	got, e := harness.ComputeLivePins(live, root)
	if e != nil {
		return e
	}
	b, e := os.ReadFile(filepath.Join(live, "pins.json"))
	if e != nil {
		return e
	}
	var want map[string]string
	if e := json.Unmarshal(b, &want); e != nil {
		return e
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("protocol pins mismatch: refusing to run on an unfrozen protocol")
	}
	return nil
}
