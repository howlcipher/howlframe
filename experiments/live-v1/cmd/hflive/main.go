// hflive drives the pre-registered live-v1 comparison. See
// experiments/live-v1/PREREGISTRATION.md. `dry-run` uses the stub model only.
// `run` contacts the pinned model and refuses to start unless every
// pre-registered condition holds.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/howlcipher/howlframe/experiments/falsifiable-v0/harness"
	"github.com/howlcipher/howlframe/internal/vm"
)

// Pre-registered run parameters. `run` refuses any other value.
const (
	RunRepetitions = "rep1"
	ArmOrderSeed   = 20261013
	TiktokenPython = "/workspace/tokvenv/bin/python"
)

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "hflive:", e)
		os.Exit(1)
	}
}

func fileSHA(p string) (string, error) {
	b, e := os.ReadFile(p)
	if e != nil {
		return "", e
	}
	return fmt.Sprintf("%x", sha256.Sum256(b)), nil
}

func git(dir string, args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = dir
	b, e := c.Output()
	return strings.TrimSpace(string(b)), e
}

func loadSets(root, blindRoot, sets string, m harness.Manifest) ([]harness.LiveTask, error) {
	out := []harness.LiveTask{}
	for _, s := range strings.Split(sets, ",") {
		switch s {
		case "tuning":
			for _, id := range harness.LiveFrozenTaskIDs(m) {
				t, _ := m.Task(id)
				out = append(out, harness.LiveTask{Task: t, Set: "tuning"})
			}
		case "blind":
			bs, e := harness.LoadBlindSet(blindRoot, harness.LiveBlindManifestSHA256)
			if e != nil {
				return nil, fmt.Errorf("blind set: %v", e)
			}
			out = append(out, bs...)
		default:
			return nil, fmt.Errorf("unknown set %q", s)
		}
	}
	return out, nil
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: hflive pins|dry-run|run|attempt [flags]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	root := fs.String("root", "experiments/falsifiable-v0", "falsifiable-v0 root")
	blindRoot := fs.String("blind-root", "experiments/live-v1-blind", "live-v1-blind root (howlframe#180)")
	live := fs.String("live", "experiments/live-v1", "live-v1 root")
	sets := fs.String("set", "tuning,blind", "task sets")
	hf := fs.String("howlframe", "", "trusted HowlFrame binary")
	out := fs.String("out", "", "new output directory")
	arm := fs.String("arm", "", "a or b (attempt)")
	task := fs.String("task", "", "task id (attempt)")
	cand := fs.String("candidate", "", "candidate file (attempt)")
	dir := fs.String("dir", "", "attempt dir (attempt)")
	deadline := fs.Duration("deadline", harness.LiveDefaultDeadline, "harness-enforced wall clock per attempt; fixed for run")
	reps := fs.String("repetitions", RunRepetitions, "repetition labels; fixed for run")
	approved := fs.Bool("seats-approved-protocol", false, "required for run")
	expectCommit := fs.String("expect-commit", "", "run: the seat-approved commit")
	expectHF := fs.String("expect-howlframe-sha256", "", "run: sha256 of the approved howlframe binary")
	// #175: runner-owned model-adapter budget, validated and capped (30 s) by
	// vm.AdapterDeadlineFlag. Recorded in provenance; fixed for run.
	var policy vm.ExecutionPolicy
	fs.Var(vm.AdapterDeadlineFlag{Policy: &policy}, "adapter-deadline", "model-adapter deadline (default 1s, cap 30s)")
	if e := fs.Parse(args[1:]); e != nil {
		return e
	}
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	*root, *live, *blindRoot = abs(*root), abs(*live), abs(*blindRoot)
	m, e := harness.LoadManifest(*root)
	if e != nil {
		return e
	}
	switch args[0] {
	case "attempt":
		tasks, e := loadSets(*root, *blindRoot, *sets, m)
		if e != nil {
			return e
		}
		for _, t := range tasks {
			if t.ID == *task {
				b, e := os.ReadFile(*cand)
				if e != nil {
					return e
				}
				return harness.RunChildAttempt(harness.Runner{Root: *root, HowlFrame: *hf}, t, *arm, b, *dir)
			}
		}
		return fmt.Errorf("unknown task %q", *task)
	case "pins":
		p, e := harness.ComputeLivePins(*live, *root)
		if e != nil {
			return e
		}
		b, _ := json.MarshalIndent(p, "", "  ")
		fmt.Println(string(b))
		return nil
	case "dry-run", "run":
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	if *out == "" || *hf == "" {
		return fmt.Errorf("--out and --howlframe required")
	}
	if _, e := os.Lstat(*out); e == nil {
		return fmt.Errorf("output directory must be new (reruns are refused): %s", *out)
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
	adapterDeadline := vm.DefaultAdapterDeadline
	if policy.AdapterDeadline != 0 {
		adapterDeadline = policy.AdapterDeadline
	}
	prov := harness.Provenance{Status: "running", Deadline: deadline.String(), AdapterDeadline: adapterDeadline.String(), Repetitions: strings.Split(*reps, ","), ArmOrderSeed: ArmOrderSeed,
		BootstrapResamples: harness.BootstrapResamples, BootstrapSeed: harness.BootstrapSeed}
	repo := filepath.Dir(filepath.Dir(*live))
	prov.Commit, _ = git(repo, "rev-parse", "HEAD")
	prov.HarnessTree, _ = git(repo, "rev-parse", "HEAD:experiments/falsifiable-v0/harness")
	prov.CodePins, _ = harness.LiveCodePins(*root)
	prov.HowlFrameSHA256, _ = fileSHA(abs(*hf))
	prov.HFLiveSHA256, _ = fileSHA(self)
	prov.PreregSHA256, _ = fileSHA(filepath.Join(*live, "PREREGISTRATION.md"))
	prov.PinsSHA256, _ = fileSHA(filepath.Join(*live, "pins.json"))
	jail := ""
	if p, e := exec.LookPath("bwrap"); e == nil && exec.Command(p, "--ro-bind", "/", "/", "--dev", "/dev", "--unshare-net", "true").Run() == nil {
		jail = p
	}
	prov.Jail = "none"
	if jail != "" {
		prov.Jail = "bwrap read-only root, attempt dir writable, no network"
	}
	counter := harness.TiktokenCounter{Python: TiktokenPython}
	prov.ReviewCounter, _ = counter.Version()
	var mdl harness.Model
	switch args[0] {
	case "dry-run":
		mdl = harness.StubModel{Root: *root, RepairIDs: map[string]bool{"t01": true, "a01": true}, ViolateIDs: map[string]bool{"a02": true}}
		prov.Model, prov.ClientVersion = mdl.Name(), "stub"
	case "run":
		var bad []string
		if !*approved {
			bad = append(bad, "--seats-approved-protocol missing")
		}
		if *reps != RunRepetitions {
			bad = append(bad, "--repetitions is fixed at "+RunRepetitions)
		}
		if *deadline != harness.LiveDefaultDeadline {
			bad = append(bad, "--deadline is fixed at "+harness.LiveDefaultDeadline.String())
		}
		if adapterDeadline != vm.DefaultAdapterDeadline {
			bad = append(bad, "-adapter-deadline is fixed at "+vm.DefaultAdapterDeadline.String())
		}
		if *sets != "tuning,blind" {
			bad = append(bad, "--set is fixed at tuning,blind")
		}
		if *expectCommit == "" || prov.Commit != *expectCommit {
			bad = append(bad, "HEAD "+prov.Commit+" is not --expect-commit")
		}
		if st, _ := git(repo, "status", "--porcelain", "--", "experiments"); st != "" {
			bad = append(bad, "experiments/ has uncommitted changes")
		}
		if *expectHF == "" || prov.HowlFrameSHA256 != *expectHF {
			bad = append(bad, "howlframe binary sha256 is not --expect-howlframe-sha256")
		}
		if v, _ := exec.Command("claude", "--version").Output(); strings.TrimSpace(string(v)) != harness.LiveClaudeCLIVersion {
			bad = append(bad, "claude CLI version is not "+harness.LiveClaudeCLIVersion)
		}
		if jail == "" {
			bad = append(bad, "bwrap OS detector unavailable")
		}
		if prov.ReviewCounter == "" {
			bad = append(bad, "tiktoken cl100k counter unavailable")
		}
		if len(bad) > 0 {
			return fmt.Errorf("refusing live run: %s", strings.Join(bad, "; "))
		}
		work := filepath.Join(*out, "model-work")
		if e := os.MkdirAll(work, 0700); e != nil {
			return e
		}
		mdl = harness.ClaudeModel{Work: work, Allowed: harness.LiveAllowedModelIDs}
		prov.Model, prov.ClientVersion = mdl.Name(), harness.LiveClaudeCLIVersion
	}
	tasks, e := loadSets(*root, *blindRoot, *sets, m)
	if e != nil {
		return e
	}
	prov.BlindManifestSHA = harness.LiveBlindManifestSHA256
	prov.ExpectedCells = len(tasks) * 2 * len(prov.Repetitions)
	lr := harness.LiveRunner{Root: *root, BlindRoot: *blindRoot, HowlFrame: abs(*hf), Self: self, Deadline: *deadline, Model: mdl, Prompts: ps, Jail: jail}
	if e := os.MkdirAll(*out, 0700); e != nil {
		return e
	}
	vlog, clog := filepath.Join(*out, "violations.jsonl"), filepath.Join(*out, "cells.jsonl")
	results := []harness.LiveResult{}
	rng := rand.New(rand.NewSource(ArmOrderSeed))
	var runErr error
cells:
	for _, rep := range prov.Repetitions {
		for _, t := range tasks {
			order := []string{"a", "b"}
			if rng.Intn(2) == 1 {
				order = []string{"b", "a"}
			}
			for i, a := range order {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				r, e := lr.Run(ctx, t, a, rep, filepath.Join(*out, "runs", t.Set, a, t.ID, rep), vlog)
				cancel()
				r.ArmOrder = i
				if e != nil {
					runErr = fmt.Errorf("%s/%s/%s/%s: %v", t.Set, a, t.ID, rep, e)
					break cells
				}
				results = append(results, r)
				if e := harness.AppendJSONL(clog, r); e != nil {
					runErr = e
					break cells
				}
				fmt.Printf("%s %s %s %s %s repairs=%d\n", t.Set, a, t.ID, rep, r.Verdict, r.RepairRounds)
			}
		}
	}
	prov.Status = "complete"
	if runErr != nil || len(results) != prov.ExpectedCells {
		prov.Status = "aborted"
		if runErr == nil {
			runErr = errors.New("coverage short of expected cells")
		}
		prov.AbortError = runErr.Error()
	}
	if prov.ReviewCounter != "" {
		texts := make([]string, len(results))
		for i, r := range results {
			texts[i] = r.FinalCandidate
		}
		if counts, e := counter.Count(texts); e == nil {
			for i := range results {
				c := counts[i]
				results[i].Review.CL100K = &c
			}
		}
	}
	s := harness.Summarize(results, prov)
	if e := harness.WriteJSON(filepath.Join(*out, "summary.json"), s); e != nil {
		return e
	}
	if e := os.WriteFile(filepath.Join(*out, "report.md"), []byte(harness.RenderReport(s)), 0600); e != nil {
		return e
	}
	return runErr
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
	if e := json.Unmarshal(bytes.TrimSpace(b), &want); e != nil {
		return e
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("protocol pins mismatch: refusing to run on an unfrozen protocol")
	}
	return nil
}
