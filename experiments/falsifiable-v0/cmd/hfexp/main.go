// hfexp replays saved candidates offline. It contains no model adapter.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/howlcipher/howlframe/experiments/falsifiable-v0/harness"
)

func main() {
	if e := run(os.Args[1:]); e != nil {
		b, _ := json.Marshal(map[string]string{"code": "HFEXP_ERROR", "message": e.Error()})
		fmt.Fprintln(os.Stderr, string(b))
		os.Exit(1)
	}
}
func printJSON(v interface{}) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e == nil {
		fmt.Println(string(b))
	}
	return e
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: hfexp validate|arm-a|arm-b|suite|score [flags]")
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	root := f.String("root", "experiments/falsifiable-v0", "scaffold root")
	task := f.String("task", "", "task ID")
	program := f.String("program", "", "HowlFrame candidate")
	plan := f.String("plan", "", "JSON candidate")
	repairs := f.String("repairs", "", "offline repair directory")
	binary := f.String("howlframe", "", "HowlFrame binary path")
	out := f.String("out", "", "new output directory")
	candidates := f.String("candidates", "", "saved candidates root")
	results := f.String("results", "", "result directory")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if f.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if args[0] == "score" {
		if *results == "" {
			return fmt.Errorf("--results required")
		}
		rs, e := harness.LoadResults(*results)
		if e != nil {
			return e
		}
		return printJSON(harness.ScoreResults(rs))
	}
	abs, e := filepath.Abs(*root)
	if e != nil {
		return e
	}
	m, e := harness.LoadManifest(abs)
	if e != nil {
		return e
	}
	if args[0] == "validate" {
		return printJSON(map[string]interface{}{"status": "valid", "tasks": len(m.Tasks), "adversarial": len(m.Adversarial), "experiment_status": m.Status})
	}
	r := harness.Runner{Root: abs}
	if *binary != "" {
		r.HowlFrame, e = filepath.Abs(*binary)
		if e != nil {
			return e
		}
	}
	if *repairs != "" {
		r.Repairer = harness.DirRepairer{Dir: *repairs}
	}
	if *out == "" {
		return fmt.Errorf("--out required")
	}
	switch args[0] {
	case "arm-a", "arm-b":
		t, e := m.Task(*task)
		if e != nil {
			return e
		}
		path := *plan
		if args[0] == "arm-a" {
			path = *program
			if r.HowlFrame == "" {
				return fmt.Errorf("--howlframe required")
			}
		}
		if path == "" {
			return fmt.Errorf("candidate file required")
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		var result harness.RunResult
		if args[0] == "arm-a" {
			result, e = r.RunArmA(context.Background(), t, b, *out)
		} else {
			result, e = r.RunArmB(context.Background(), t, b, *out)
		}
		if e != nil {
			return e
		}
		return printJSON(result)
	case "suite":
		if *candidates == "" || r.HowlFrame == "" {
			return fmt.Errorf("--candidates and --howlframe required")
		}
		if _, e := os.Lstat(*out); !os.IsNotExist(e) {
			return fmt.Errorf("suite --out must be new")
		}
		count := 0
		for _, t := range m.All() {
			for _, arm := range []string{"a", "b"} {
				ext := ".howl"
				if arm == "b" {
					ext = ".plan.json"
				}
				paths, e := filepath.Glob(filepath.Join(*candidates, "arm_"+arm, t.ID, "seed*"+ext))
				if e != nil {
					return e
				}
				for _, p := range paths {
					seed := strings.TrimSuffix(filepath.Base(p), ext)
					if !validSeed(seed) {
						return fmt.Errorf("invalid seed filename %s", p)
					}
					b, e := os.ReadFile(p)
					if e != nil {
						return e
					}
					runner := r
					if *repairs != "" {
						runner.Repairer = harness.DirRepairer{Dir: filepath.Join(*repairs, "arm_"+arm, t.ID, seed)}
					}
					dest := filepath.Join(*out, "arm_"+arm, t.ID, seed)
					var result harness.RunResult
					if arm == "a" {
						result, e = runner.RunArmA(context.Background(), t, b, dest)
					} else {
						result, e = runner.RunArmB(context.Background(), t, b, dest)
					}
					if e != nil {
						return e
					}
					result.Seed = seed
					if e := harness.WriteJSON(filepath.Join(dest, "result.json"), result); e != nil {
						return e
					}
					count++
				}
			}
		}
		if count == 0 {
			return fmt.Errorf("no candidates found")
		}
		return printJSON(map[string]interface{}{"status": "replayed", "provided_candidates": count, "expected_full_suite": 360, "complete_candidate_count": count == 360})
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}
func validSeed(s string) bool {
	if !strings.HasPrefix(s, "seed") || len(s) == 4 {
		return false
	}
	for _, c := range s[4:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
