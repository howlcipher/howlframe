// Command heldoutverify replays every held-out reference in a fresh sandbox
// and compares its single output with the frozen expected value.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

type Task struct {
	ID       string            `json:"id"`
	Category string            `json:"category"`
	Inputs   map[string]string `json:"inputs"`
	Output   string            `json:"output"`
	Kind     string            `json:"kind"`
	Expected json.RawMessage   `json:"expected"`
}

type Gap struct {
	Task   string `json:"task"`
	Lang   string `json:"lang"`
	Reason string `json:"reason"`
}

type Result struct {
	Task   string `json:"task"`
	Lang   string `json:"lang"`
	Pass   bool   `json:"pass"`
	SHA256 string `json:"sha256"`
	Detail string `json:"detail,omitempty"`
}

var exts = map[string]string{"howl": "howl", "python": "py", "js": "js", "go": "go"}

func main() {
	root := flag.String("root", "experiments/token-efficiency-heldout-v1", "benchmark root")
	howl := flag.String("howlframe", "/tmp/howlframe", "HowlFrame binary")
	flag.Parse()
	var manifest struct{ Tasks []Task }
	must(readJSON(filepath.Join(*root, "tasks/manifest.json"), &manifest))
	var gaps []Gap
	must(readJSON(filepath.Join(*root, "gaps.json"), &gaps))
	gapped := map[string]bool{}
	for _, g := range gaps {
		gapped[g.Task+"/"+g.Lang] = true
	}
	work, err := os.MkdirTemp("", "hf-heldout-verify-")
	must(err)
	fmt.Println("Replay artifacts:", work)
	var results []Result
	failed := false
	for _, lang := range []string{"howl", "python", "js", "go"} {
		passed, total := 0, 0
		for _, t := range manifest.Tasks {
			if gapped[t.ID+"/"+lang] {
				continue
			}
			total++
			src, _ := filepath.Abs(filepath.Join(*root, "solutions", lang, t.ID+"."+exts[lang]))
			data, readErr := os.ReadFile(src)
			r := Result{Task: t.ID, Lang: lang, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))}
			if readErr != nil {
				r.Detail = readErr.Error()
			} else {
				r.Detail = verify(filepath.Join(work, lang+"-"+t.ID), src, lang, *howl, t)
				r.Pass = r.Detail == ""
			}
			if r.Pass {
				passed++
			} else {
				failed = true
			}
			fmt.Printf("%s %s pass=%v %s\n", lang, t.ID, r.Pass, r.Detail)
			results = append(results, r)
		}
		fmt.Printf("%s: %d/%d passed\n", lang, passed, total)
	}
	out, _ := json.MarshalIndent(results, "", "  ")
	must(os.MkdirAll(filepath.Join(*root, "results"), 0o755))
	must(os.WriteFile(filepath.Join(*root, "results/verification.json"), append(out, '\n'), 0o644))
	if failed {
		os.Exit(1)
	}
}

func verify(dir, src, lang, howl string, t Task) string {
	if err := os.MkdirAll(filepath.Join(dir, "out"), 0o755); err != nil {
		return err.Error()
	}
	for p, content := range t.Inputs {
		full := filepath.Join(dir, p)
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err.Error()
		}
	}
	absDir, _ := filepath.Abs(dir)
	var cmd []string
	switch lang {
	case "howl":
		art := filepath.Join(filepath.Dir(absDir), filepath.Base(absDir)+".hfbc")
		if o, err := run(absDir, 30*time.Second, howl, "build", "--profile", "governed", src, "-o", art); err != nil {
			return "build: " + o
		}
		cmd = []string{howl, "-run-bc", "-allow-caps", "filesystem:read=" + filepath.Join(absDir, "in") + ",filesystem:write=" + filepath.Join(absDir, "out"), art}
	case "python":
		cmd = []string{"python3", src}
	case "js":
		cmd = []string{"node", src}
	case "go":
		bin := absDir + ".bin"
		if o, err := run(filepath.Dir(absDir), 60*time.Second, "go", "build", "-o", bin, src); err != nil {
			return "go build: " + o
		}
		cmd = []string{bin}
	}
	if o, err := run(absDir, 10*time.Second, cmd[0], cmd[1:]...); err != nil {
		return "run: " + o
	}
	// Exactly the inputs (unchanged) plus the one output may exist.
	var files []string
	filepath.WalkDir(absDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(absDir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	want := []string{t.Output}
	for p := range t.Inputs {
		want = append(want, p)
	}
	sort.Strings(files)
	sort.Strings(want)
	if !reflect.DeepEqual(files, want) {
		return fmt.Sprintf("file set %v, want %v", files, want)
	}
	for p, content := range t.Inputs {
		got, _ := os.ReadFile(filepath.Join(absDir, p))
		if string(got) != content {
			return "input modified: " + p
		}
	}
	got, err := os.ReadFile(filepath.Join(absDir, t.Output))
	if err != nil {
		return err.Error()
	}
	return compare(got, t)
}

func compare(got []byte, t Task) string {
	if t.Kind == "text" {
		var want string
		json.Unmarshal(t.Expected, &want)
		g := strings.TrimRight(string(got), "\n")
		if g != strings.TrimRight(want, "\n") {
			return fmt.Sprintf("text mismatch: got %q want %q", g, want)
		}
		return ""
	}
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		return "invalid JSON output: " + err.Error()
	}
	json.Unmarshal(t.Expected, &w)
	if !reflect.DeepEqual(g, w) {
		return fmt.Sprintf("json mismatch: got %s want %s", bytes.TrimSpace(got), t.Expected)
	}
	return ""
}

func run(dir string, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, name, args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if ctx.Err() != nil {
		return "timeout", ctx.Err()
	}
	s := string(out)
	if len(s) > 400 {
		s = s[:400]
	}
	return s, err
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
