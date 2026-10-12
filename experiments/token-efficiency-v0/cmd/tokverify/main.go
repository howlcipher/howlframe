// tokverify runs trusted offline references and retains every outcome.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/howlcipher/howlframe/experiments/falsifiable-v0/harness"
	"github.com/howlcipher/howlframe/experiments/internal/pathparity"
)

type Result struct {
	Task        string             `json:"task"`
	Lang        string             `json:"lang"`
	Pass        bool               `json:"pass"`
	SHA256      string             `json:"sha256"`
	Diagnostics string             `json:"diagnostics"`
	Comparison  harness.Comparison `json:"comparison"`
}

func command(dir string, timeout time.Duration, minimal bool, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	if minimal {
		cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + filepath.Dir(dir), "LANG=C", "TZ=UTC"}
	}
	b, err := cmd.CombinedOutput()
	return string(b), err
}

func verify(root, work, source, lang, hfexp, howlframe string, task harness.Task) (harness.Comparison, string, error) {
	if lang == "howl" {
		output, err := command(root, 30*time.Second, false, hfexp, "arm-a", "--root", filepath.Join(root, "experiments/falsifiable-v0"), "--task", task.ID, "--program", source, "--howlframe", howlframe, "--out", work)
		if err != nil {
			return harness.Comparison{}, output, err
		}
		b, err := os.ReadFile(filepath.Join(work, "result.json"))
		if err != nil {
			return harness.Comparison{}, output, err
		}
		var run harness.RunResult
		if err = json.Unmarshal(b, &run); err != nil {
			return harness.Comparison{}, output, err
		}
		return run.Comparison, fmt.Sprint(run.Diagnostics), nil
	}
	if err := os.Mkdir(work, 0700); err != nil {
		return harness.Comparison{}, "", err
	}
	corpus := filepath.Join(root, "experiments/falsifiable-v0")
	sandbox, canary, err := harness.MaterializeReferenceSandbox(corpus, work, task)
	if err != nil {
		return harness.Comparison{}, "", err
	}
	before, err := harness.SnapshotTree(sandbox)
	if err != nil {
		return harness.Comparison{}, "", err
	}
	cb, err := harness.SnapshotTree(canary)
	if err != nil {
		return harness.Comparison{}, "", err
	}
	var name string
	var args []string
	switch lang {
	case "python":
		name, args = "python3", []string{source}
	case "js":
		name, args = "node", []string{source}
	case "go":
		name = filepath.Join(work, "program")
		output, err := command(root, 60*time.Second, false, "go", "build", "-o", name, source)
		if err != nil {
			return harness.Comparison{}, output, err
		}
	}
	// Resolve before replacing PATH in the child environment.
	name, err = exec.LookPath(name)
	if err != nil {
		return harness.Comparison{}, "", err
	}
	output, runErr := command(sandbox, 5*time.Second, true, name, args...)
	after, err := harness.SnapshotTree(sandbox)
	if err != nil {
		return harness.Comparison{}, output, err
	}
	ca, err := harness.SnapshotTree(canary)
	if err != nil {
		return harness.Comparison{}, output, err
	}
	diff := harness.Diff(before, after)
	cmp, err := harness.CompareOracle(corpus, sandbox, task, diff, harness.UnauthorizedDiff(task, diff, harness.Diff(cb, ca)), nil)
	if runErr != nil {
		cmp.Success = false
		return cmp, output, runErr
	}
	return cmp, output, err
}

func main() {
	rootFlag := flag.String("root", ".", "repository root")
	resultsFlag := flag.String("results", "experiments/token-efficiency-v0/results/verification.json", "result file relative to root")
	hfexpFlag := flag.String("hfexp", "/tmp/hfexp", "built harness CLI")
	howlFlag := flag.String("howlframe", "/tmp/howlframe", "built HowlFrame CLI")
	flag.Parse()
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		panic(err)
	}
	manifest, err := harness.LoadManifest(filepath.Join(root, "experiments/falsifiable-v0"))
	if err != nil {
		panic(err)
	}
	work, err := os.MkdirTemp("", "hf-token-verify-")
	if err != nil {
		panic(err)
	}
	fmt.Println("Replay artifacts:", work)
	results := []Result{}
	extensions := map[string]string{"howl": "howl", "python": "py", "js": "js", "go": "go"}
	failed := false
	for _, lang := range []string{"howl", "python", "js", "go"} {
		passed := 0
		for _, task := range manifest.All() {
			source := filepath.Join(root, "experiments/token-efficiency-v0/solutions", lang, task.ID+"."+extensions[lang])
			data, readErr := os.ReadFile(source)
			result := Result{Task: task.ID, Lang: lang, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
			if readErr != nil {
				err = readErr
			} else {
				result.Comparison, result.Diagnostics, err = verify(root, filepath.Join(work, lang+"-"+task.ID), source, lang, *hfexpFlag, *howlFlag, task)
				if err == nil && lang == "howl" && result.Comparison.Success {
					// #159: interpreter, governed bytecode and HFIR bytecode
					// must agree byte for byte in identical sandboxes.
					if detail := pathparity.Check(*howlFlag, source, filepath.Join(work, "parity-"+task.ID), v0Sandbox(root, task)); detail != "" {
						err = fmt.Errorf("%s", detail)
					}
				}
			}
			if err != nil {
				result.Diagnostics += " " + err.Error()
			}
			result.Pass = err == nil && result.Comparison.Success
			if result.Pass {
				passed++
			} else {
				failed = true
			}
			fmt.Printf("%s %s pass=%t %s\n", lang, task.ID, result.Pass, result.Diagnostics)
			results = append(results, result)
		}
		fmt.Printf("%s: %d/%d passed\n", lang, passed, len(manifest.All()))
	}
	path := *resultsFlag
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	if err := harness.WriteJSON(path, results); err != nil {
		panic(err)
	}
	if failed {
		os.Exit(1)
	}
}

// v0Sandbox materializes the same reference sandbox the other languages use
// and grants the task's declared capabilities.
func v0Sandbox(root string, task harness.Task) pathparity.Sandbox {
	return func(run string) (string, string, error) {
		dir := filepath.Join(run, "ref")
		if err := os.Mkdir(dir, 0o700); err != nil {
			return "", "", err
		}
		sandbox, _, err := harness.MaterializeReferenceSandbox(filepath.Join(root, "experiments/falsifiable-v0"), dir, task)
		return sandbox, strings.Join(task.AllowedCaps, ","), err
	}
}
