package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/agent"
)

func TestAgentCLI(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "howlframe")
	if out, err := exec.Command("go", "build", "-o", binary, "howlframe.go").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	run := func(args ...string) (agent.Response, error) {
		t.Helper()
		out, err := exec.Command(binary, append([]string{"agent"}, args...)...).CombinedOutput()
		var r agent.Response
		if e := json.Unmarshal(out, &r); e != nil {
			t.Fatalf("invalid JSON: %s: %v", out, e)
		}
		return r, err
	}
	source := filepath.Join(dir, "-source.howl")
	if err := os.WriteFile(source, []byte(agent.ExampleSource), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := run("check", "--json", "--", source)
	if err != nil || !r.OK || len(r.Diagnostics) != 0 || r.Requirements == nil || len(r.Requirements.Capabilities) != 0 || len(r.ActualGrants) != 0 {
		t.Fatalf("pure check: %v %+v", err, r)
	}

	// A relative leading-hyphen filename is accepted only after the option terminator.
	hyphenCmd := exec.Command(binary, "agent", "check", "--json", "--", "-source.howl")
	hyphenCmd.Dir = dir
	if out, e := hyphenCmd.CombinedOutput(); e != nil {
		t.Fatalf("relative hyphen source: %v %s", e, out)
	}
	explained, e := run("explain", "ARITY_ERROR")
	if e != nil || !explained.OK || explained.Explanation == "" {
		t.Fatalf("explanation: %v %+v", e, explained)
	}
	// Put owned compilation scopes below a private parent so both success and failure cleanup are observable.
	tempRoot := filepath.Join(dir, "temporary-scopes")
	if e := os.Mkdir(tempRoot, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("TMPDIR", tempRoot)

	// Compile and run the actual advertised records, so catalog/example drift fails.
	catalog, catalogErr := run("capabilities")
	if catalogErr != nil || len(catalog.Examples) == 0 {
		t.Fatalf("missing executable examples: %v %+v", catalogErr, catalog)
	}
	for i, example := range catalog.Examples {
		if example.Target != "bytecode" {
			t.Fatalf("unverified advertised target: %+v", example)
		}
		examplePath := filepath.Join(dir, fmt.Sprintf("example-%d.howl", i))
		artifact := filepath.Join(dir, fmt.Sprintf("example-%d.hfbc", i))
		if e := os.WriteFile(examplePath, []byte(example.Source), 0600); e != nil {
			t.Fatal(e)
		}
		if out, e := exec.Command(binary, "build", "--target", example.Target, "-o", artifact, examplePath).CombinedOutput(); e != nil {
			t.Fatalf("advertised example build: %v %s", e, out)
		}
		if out, e := exec.Command(binary, "run", artifact).CombinedOutput(); e != nil || string(out) != example.Stdout {
			t.Fatalf("advertised example run: %v %q expected %q", e, out, example.Stdout)
		}
	}
	assertRepeat := func(path string) {
		t.Helper()
		var previous []byte
		for i := 0; i < 2; i++ {
			out, e := exec.Command(binary, "agent", "check", "--json", path).CombinedOutput()
			if e != nil {
				t.Fatalf("repeat check: %v %s", e, out)
			}
			if i > 0 && !bytes.Equal(previous, out) {
				t.Fatalf("nondeterministic check: %s vs %s", previous, out)
			}
			previous = out
		}
	}
	assertRepeat(source)

	for _, tc := range []struct{ name, source, code string }{
		{"arity", "(cli_app (append (list 1)))", "ARITY_ERROR"},
		{"parse", "(cli_app (print", ""},
		{"unsupported", `(cli_app (test "sample" (assert true)))`, "HFIR_TARGET_INFEASIBLE"},
		{"type", `(cli_app (print (if true 1 "x")))`, "AGENT_CHECK_FAILED"},
		{"authority", `(cli_app (write_file "` + filepath.Join(dir, "must-not-exist") + `" "bad"))`, ""},
	} {
		path := filepath.Join(dir, tc.name+".howl")
		os.WriteFile(path, []byte(tc.source), 0600)
		r, err := run("check", "--json", path)
		if tc.name == "authority" {
			if err != nil || r.Requirements == nil || len(r.Requirements.Capabilities) != 1 || r.Requirements.Capabilities[0] != "filesystem" {
				t.Fatalf("authority inventory: %v %+v", err, r)
			}
			if _, e := os.Stat(filepath.Join(dir, "must-not-exist")); !os.IsNotExist(e) {
				t.Fatal("check executed source")
			}
			assertRepeat(path)
			continue
		}
		if err == nil || len(r.Diagnostics) == 0 {
			t.Fatalf("accepted %s: %+v", tc.name, r)
		}
		if tc.code != "" && r.Diagnostics[0].Code != tc.code {
			t.Fatal(r)
		}
		// Compare the wrapper's message and coordinates with the production compiler,
		// rather than maintaining a parallel guessed parser/checker location contract.
		raw, rawErr := exec.Command(binary, "build", "--target", "bytecode", "-o", filepath.Join(dir, "rejected.hfbc"), path).CombinedOutput()
		if rawErr == nil {
			t.Fatalf("production unexpectedly accepted %s", tc.name)
		}
		var legacy struct {
			Reason string `json:"reason"`
			Line   int    `json:"line"`
			Column int    `json:"column"`
		}
		if json.Unmarshal(raw, &legacy) == nil && legacy.Reason != "" {
			d := r.Diagnostics[0]
			if d.Message != legacy.Reason || d.Line != legacy.Line || d.Column != legacy.Column {
				t.Fatalf("lost production diagnostic: %+v vs %s", d, raw)
			}
		} else {
			var structured []struct {
				Code     string `json:"code"`
				Message  string `json:"message"`
				Location struct {
					Filename string `json:"filename"`
					Line     int    `json:"line"`
					Column   int    `json:"column"`
				} `json:"location"`
			}
			if e := json.Unmarshal(raw, &structured); e != nil || len(structured) != len(r.Diagnostics) {
				t.Fatalf("unexpected production diagnostics: %s", raw)
			}
			for i, d := range structured {
				got := r.Diagnostics[i]
				if got.Code != d.Code || got.Message != d.Message || got.Filename != d.Location.Filename || got.Line != d.Location.Line || got.Column != d.Location.Column {
					t.Fatalf("lost HFIR diagnostic: %+v vs %+v", got, d)
				}
			}
		}

	}

	// Include warnings remain visible on success; mixed warning/JSON failure is
	// explicitly a fallback, rather than silently discarding a compiler message.
	includePart := filepath.Join(dir, "included.howl")
	os.WriteFile(includePart, []byte(`(print "included")`), 0600)
	includeRoot := filepath.Join(dir, "include-root.howl")
	os.WriteFile(includeRoot, []byte(`(cli_app (include "included.howl"))`), 0600)
	included, includeErr := run("check", "--json", includeRoot)
	if includeErr != nil || len(included.CompilerNotes) != 1 || !strings.Contains(included.CompilerNotes[0], "deprecated") {
		t.Fatalf("lost include warning: %v %+v", includeErr, included)
	}
	os.WriteFile(includePart, []byte(`(append (list 1))`), 0600)
	included, includeErr = run("check", "--json", includeRoot)
	if includeErr == nil || len(included.Diagnostics) != 1 || included.Diagnostics[0].Code != "AGENT_CHECK_FAILED" || !strings.Contains(included.Diagnostics[0].Message, "deprecated") {
		t.Fatalf("mixed diagnostics silently normalized: %v %+v", includeErr, included)
	}
	// Legacy compiler diagnostics cannot identify an imported module filename.
	module := filepath.Join(dir, "bad-module.howl")
	os.WriteFile(module, []byte(`(module (export (defun bad () (return (append (list 1))))))`), 0600)
	moduleRoot := filepath.Join(dir, "module-root.howl")
	os.WriteFile(moduleRoot, []byte(`(cli_app (use "bad-module.howl" as bad) (print (call bad/bad)))`), 0600)
	imported, importErr := run("check", "--json", moduleRoot)
	if importErr == nil || len(imported.Diagnostics) != 1 || imported.Diagnostics[0].Code != "ARITY_ERROR" || imported.Diagnostics[0].Filename != "" {
		t.Fatalf("false imported provenance: %v %+v", importErr, imported)
	}
	if entries, e := os.ReadDir(tempRoot); e != nil || len(entries) != 0 {
		t.Fatalf("temporary compilation artifacts remain: %v %v", entries, e)
	}
	files, _ := os.ReadDir(dir)
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".hfbc") && !strings.HasPrefix(f.Name(), "example-") {
			t.Fatal("check leaked artifact")
		}
	}
}
