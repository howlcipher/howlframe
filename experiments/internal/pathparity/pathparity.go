// Package pathparity replays one HowlFrame program on every execution path the
// benchmark verifiers trust - the AST interpreter (-run), governed AST
// bytecode (build --profile governed, then -run-bc) and the experimental HFIR
// bytecode (-compile-hfir-bc, then -run-bc) - each in a fresh sandbox with the
// same grant, and reports any byte-level divergence (issue #159).
package pathparity

import (
	"context"
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

// Paths lists the compared execution paths in report order.
var Paths = []string{"interpreter", "bytecode", "hfir"}

// Outcome is everything a path is allowed to make observable: its combined
// stdout/stderr, its exit status and the full resulting sandbox tree.
type Outcome struct {
	Output string
	Exit   int
	Files  map[string]string
}

// Sandbox prepares a fresh, private sandbox under run and returns its path
// together with the -allow-caps grant for it.
type Sandbox func(run string) (dir, caps string, err error)

// Run executes src on path in a sandbox prepared by prepare under work.
func Run(howl, src, path, work string, prepare Sandbox) (Outcome, error) {
	if err := os.MkdirAll(work, 0o700); err != nil {
		return Outcome{}, err
	}
	dir, caps, err := prepare(work)
	if err != nil {
		return Outcome{}, err
	}
	src, _ = filepath.Abs(src)
	art := filepath.Join(work, "prog.hfbc")
	command := func(args ...string) (string, int) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, howl, args...)
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
		if o, c := command("build", "--profile", "governed", src, "-o", art); c != 0 {
			return Outcome{Output: "build failed: " + o, Exit: c}, nil
		}
		out, code = command("-run-bc", "-allow-caps", caps, art)
	case "hfir":
		if o, c := command("-compile-hfir-bc", src, "-o", art); c != 0 {
			return Outcome{Output: "hfir compile failed: " + o, Exit: c}, nil
		}
		out, code = command("-run-bc", "-allow-caps", caps, art)
	default:
		return Outcome{}, fmt.Errorf("unknown path %q", path)
	}
	return Outcome{Output: out, Exit: code, Files: snapshot(dir)}, nil
}

// Check runs src on every path and returns "" when all outcomes are
// identical, or a description of the first divergence from the interpreter.
func Check(howl, src, work string, prepare Sandbox) string {
	var base Outcome
	for i, p := range Paths {
		got, err := Run(howl, src, p, filepath.Join(work, p), prepare)
		if err != nil {
			return p + ": " + err.Error()
		}
		if i == 0 {
			base = got
			continue
		}
		if !reflect.DeepEqual(got, base) {
			return fmt.Sprintf("path parity: %s differs from interpreter: %s", p, describe(got, base))
		}
	}
	return ""
}

func describe(got, want Outcome) string {
	if got.Exit != want.Exit || got.Output != want.Output {
		return fmt.Sprintf("exit %d output %q, interpreter exit %d output %q", got.Exit, got.Output, want.Exit, want.Output)
	}
	var names []string
	for k := range got.Files {
		names = append(names, k)
	}
	for k := range want.Files {
		if _, ok := got.Files[k]; !ok {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	for _, k := range names {
		g, gok := got.Files[k]
		w, wok := want.Files[k]
		if gok != wok || g != w {
			return fmt.Sprintf("file %s: %q (present=%v), interpreter %q (present=%v)", k, g, gok, w, wok)
		}
	}
	return "outcomes differ"
}

func snapshot(dir string) map[string]string {
	files := map[string]string{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		b, _ := os.ReadFile(p)
		files[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	return files
}
