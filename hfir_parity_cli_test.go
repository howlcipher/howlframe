package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/experiments/falsifiable-v0/harness"
)

// Issue #159: every frozen HowlFrame benchmark solution must behave byte for
// byte the same on the AST interpreter (-run), governed AST bytecode
// (build --profile governed + -run-bc) and the experimental HFIR bytecode
// (-compile-hfir-bc + -run-bc). Each path runs in a fresh sandbox with the
// same grant; the observed outcome is stdout+stderr, the exit status and the
// full resulting file tree. Any divergence fails CI.

type parityCase struct {
	set, id, src string
	caps         string
	setup        func(dir string) error
}

type parityOutcome struct {
	Output string
	Exit   int
	Files  map[string]string
}

func parityHeldoutCases(t *testing.T, set, root, solutions string) []parityCase {
	var manifest struct {
		Tasks []struct {
			ID     string            `json:"id"`
			Inputs map[string]string `json:"inputs"`
		} `json:"tasks"`
	}
	data, err := os.ReadFile(filepath.Join(root, "tasks/manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	var cases []parityCase
	for _, task := range manifest.Tasks {
		src := filepath.Join(solutions, task.ID+".howl")
		if _, err := os.Stat(src); err != nil {
			continue // recorded gap
		}
		inputs := task.Inputs
		cases = append(cases, parityCase{set: set, id: task.ID, src: src, caps: "filesystem:read=$DIR/in,filesystem:write=$DIR/out", setup: func(dir string) error {
			if err := os.MkdirAll(filepath.Join(dir, "out"), 0o755); err != nil {
				return err
			}
			for p, content := range inputs {
				full := filepath.Join(dir, p)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					return err
				}
			}
			return nil
		}})
	}
	return cases
}

func parityV0Cases(t *testing.T) []parityCase {
	corpus, _ := filepath.Abs("experiments/falsifiable-v0")
	manifest, err := harness.LoadManifest(corpus)
	if err != nil {
		t.Fatal(err)
	}
	var cases []parityCase
	for _, task := range manifest.All() {
		src := filepath.Join("experiments/token-efficiency-v0/solutions/howl", task.ID+".howl")
		if _, err := os.Stat(src); err != nil {
			continue
		}
		// The sandbox is materialized per path in runParityPath.
		cases = append(cases, parityCase{set: "v0", id: task.ID, src: src, caps: strings.Join(task.AllowedCaps, ",")})
	}
	return cases
}

func allParityCases(t *testing.T) []parityCase {
	var cases []parityCase
	for _, s := range []struct{ set, root, sol string }{
		{"heldout-v0", "experiments/token-efficiency-heldout-v0", "experiments/token-efficiency-heldout-v0/solutions/howl"},
		{"heldout-v1", "experiments/token-efficiency-heldout-v1", "experiments/token-efficiency-heldout-v1/solutions/howl"},
		{"heldout-v2", "experiments/token-efficiency-heldout-v2", "experiments/token-efficiency-heldout-v2/solutions/howl"},
		{"heldout-v2-postcf", "experiments/token-efficiency-heldout-v2", "experiments/token-efficiency-heldout-v2/postcf/solutions/howl"},
		{"heldout-v3", "experiments/token-efficiency-heldout-v3", "experiments/token-efficiency-heldout-v3/solutions/howl"},
		{"heldout-v3-postrec", "experiments/token-efficiency-heldout-v3", "experiments/token-efficiency-heldout-v3/postrec/solutions/howl"},
	} {
		cases = append(cases, parityHeldoutCases(t, s.set, s.root, s.sol)...)
	}
	return append(cases, parityV0Cases(t)...)
}

func snapshotParityTree(dir string) map[string]string {
	files := map[string]string{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		b, _ := os.ReadFile(p)
		files[rel] = string(b)
		return nil
	})
	return files
}

func runParityPath(t *testing.T, bin string, c parityCase, path string) parityOutcome {
	t.Helper()
	work := t.TempDir()
	dir := filepath.Join(work, "sandbox")
	if c.set == "v0" {
		corpus, _ := filepath.Abs("experiments/falsifiable-v0")
		manifest, err := harness.LoadManifest(corpus)
		if err != nil {
			t.Fatal(err)
		}
		var task harness.Task
		for _, tk := range manifest.All() {
			if tk.ID == c.id {
				task = tk
			}
		}
		run := filepath.Join(work, "run")
		if err := os.Mkdir(run, 0o700); err != nil {
			t.Fatal(err)
		}
		if dir, _, err = harness.MaterializeReferenceSandbox(corpus, run, task); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := c.setup(dir); err != nil {
			t.Fatal(err)
		}
	}
	caps := strings.ReplaceAll(c.caps, "$DIR", dir)
	src, _ := filepath.Abs(c.src)
	art := filepath.Join(work, "prog.hfbc")
	command := func(args ...string) (string, int) {
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + work, "LANG=C", "TZ=UTC"}
		out, err := cmd.CombinedOutput()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if err != nil {
			code = -1
		}
		return strings.ReplaceAll(string(out), work, "$WORK"), code
	}
	var out string
	var code int
	switch path {
	case "interpreter":
		out, code = command("-run", "-allow-caps", caps, src)
	case "bytecode":
		if o, e := command("build", "--profile", "governed", src, "-o", art); e != 0 {
			return parityOutcome{Output: "build failed: " + o, Exit: e}
		}
		out, code = command("-run-bc", "-allow-caps", caps, art)
	case "hfir":
		if o, e := command("-compile-hfir-bc", src, "-o", art); e != 0 {
			return parityOutcome{Output: "hfir compile failed: " + o, Exit: e}
		}
		out, code = command("-run-bc", "-allow-caps", caps, art)
	}
	return parityOutcome{Output: out, Exit: code, Files: snapshotParityTree(dir)}
}

func TestHFIRBenchmarkSolutionParity(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs every benchmark solution on three paths")
	}
	bin := c1c2Binary(t)
	cases := allParityCases(t)
	if len(cases) < 260 {
		t.Fatalf("parity corpus has %d programs, want every benchmark solution", len(cases))
	}
	for _, c := range cases {
		c := c
		t.Run(c.set+"/"+c.id, func(t *testing.T) {
			t.Parallel()
			interp := runParityPath(t, bin, c, "interpreter")
			for _, p := range []string{"bytecode", "hfir"} {
				got := runParityPath(t, bin, c, p)
				if fmt.Sprint(got) != fmt.Sprint(interp) {
					t.Errorf("%s differs from interpreter:\n%s: %+v\ninterpreter: %+v", p, p, got, interp)
				}
			}
		})
	}
}

// #159 root cause 1 (the x03 false denial): the stray parse_json body left on
// the stack was popped by WRITE_FILE as its path. The fix must not widen the
// grant: inside the write root every path succeeds, and with no write root or
// a different one every path is still denied and writes nothing.
func TestHFIRParseJSONWriteFileStaysFailClosed(t *testing.T) {
	bin := c1c2Binary(t)
	program := `(do (write_file "out/r.json" (encode_json (map_get (parse_json Any (read_text "in/d.json")) "a"))))`
	src := filepath.Join(t.TempDir(), "p.howl")
	if err := os.WriteFile(src, []byte(program), 0o644); err != nil {
		t.Fatal(err)
	}
	setup := func(dir string) error {
		os.MkdirAll(filepath.Join(dir, "in"), 0o755)
		os.MkdirAll(filepath.Join(dir, "out"), 0o755)
		os.MkdirAll(filepath.Join(dir, "other"), 0o755)
		return os.WriteFile(filepath.Join(dir, "in/d.json"), []byte(`{"a": {"y": [1, 2]}}`), 0o644)
	}
	grants := map[string]string{
		"granted":     "filesystem:read=$DIR/in,filesystem:write=$DIR/out",
		"read_only":   "filesystem:read=$DIR/in",
		"other_write": "filesystem:read=$DIR/in,filesystem:write=$DIR/other",
	}
	for name, caps := range grants {
		c := parityCase{set: "focused", id: name, src: src, caps: caps, setup: setup}
		for _, p := range []string{"interpreter", "bytecode", "hfir"} {
			got := runParityPath(t, bin, c, p)
			written, wrote := got.Files["out/r.json"]
			if name == "granted" {
				if got.Exit != 0 || written != `{"y":[1,2]}` {
					t.Errorf("%s/%s: exit %d output %q file %q", name, p, got.Exit, got.Output, written)
				}
				continue
			}
			if got.Exit == 0 || wrote || !strings.Contains(got.Output, "denied") {
				t.Errorf("%s/%s must fail closed: exit %d output %q wrote=%v", name, p, got.Exit, got.Output, wrote)
			}
		}
	}
}
