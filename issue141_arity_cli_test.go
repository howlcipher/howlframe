package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/construct"
)

// issue141Run runs the CLI with a timeout and empty stdin. It never executes
// a program that passes the arity check except through build-only commands.
func issue141Run(bin string, args ...string) (string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader("")
	raw, err := cmd.CombinedOutput()
	// ReportError emits JSON, which escapes <, > and &.
	out := strings.NewReplacer(`\u003c`, "<", `\u003e`, ">", `\u0026`, "&").Replace(string(raw))
	code := 0
	if err != nil {
		code = -1
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}
	return out, code
}

func issue141Panicked(out string) bool {
	return strings.Contains(out, "panic:") || strings.Contains(out, "goroutine ") || strings.Contains(out, "runtime error")
}

// issue141Form builds (head 0 0 ...) with n placeholder operands.
func issue141Form(head string, n int) string {
	args := strings.Repeat(" 0", n)
	return "(" + head + args + ")"
}

func issue141Source(form string) string {
	return "(cli_app\n " + form + ")\n"
}

// assertArityFailClosed builds src under both profiles and requires exit 1,
// no panic, no artifact, and a coded ARITY_ERROR (governed may reject a
// forbidden construct first with PROFILE_FORBIDDEN_CONSTRUCT).
func assertArityFailClosed(t *testing.T, bin, dir, name, src, want string) {
	t.Helper()
	source := filepath.Join(dir, name+".howl")
	if err := os.WriteFile(source, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"default", "governed"} {
		art := filepath.Join(dir, name+"."+profile+".hfbc")
		out, code := issue141Run(bin, "build", "--profile", profile, source, "-o", art)
		if code != 1 || issue141Panicked(out) {
			t.Errorf("%s [%s]: exit %d, want 1 without panic: %s", name, profile, code, out)
		}
		ok := strings.Contains(out, "ARITY_ERROR")
		if want != "" && profile == "default" {
			ok = strings.Contains(out, want)
		}
		if profile == "governed" && strings.Contains(out, "PROFILE_FORBIDDEN_CONSTRUCT") {
			ok = true
		}
		if !ok {
			t.Errorf("%s [%s]: want %q, got %s", name, profile, want, out)
		}
		if _, err := os.Stat(art); !os.IsNotExist(err) {
			t.Errorf("%s [%s]: artifact written", name, profile)
		}
	}
}

func TestIssue141MotokoArityRepros(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	cases := []struct{ name, form, want string }{
		{"read_file0", `(read_file)`, "ARITY_ERROR: read_file expects 1 arg, got 0"},
		{"write_file0", `(write_file)`, "ARITY_ERROR: write_file expects 2 args, got 0"},
		{"write_file1", `(write_file "a")`, "ARITY_ERROR: write_file expects 2 args, got 1"},
		{"mkdir0", `(mkdir)`, "ARITY_ERROR: mkdir expects"},
		{"env0", `(print (env))`, "ARITY_ERROR: env expects"},
		{"task0", `(task)`, "ARITY_ERROR: task expects"},
		{"res0", `(res)`, "ARITY_ERROR: res expects 3 args, got 0"},
		{"res1", `(res 200)`, "ARITY_ERROR: res expects 3 args, got 1"},
		{"res2", `(res 200 "text/plain")`, "ARITY_ERROR: res expects 3 args, got 2"},
		{"res_json0", `(res_json)`, "ARITY_ERROR: res_json expects 2 args, got 0"},
		{"res_json1", `(res_json 200)`, "ARITY_ERROR: res_json expects 2 args, got 1"},
		{"store_open0", `(store_open)`, "ARITY_ERROR: store_open expects"},
		{"defun0", `(defun)`, "ARITY_ERROR: defun expects at least 3 args, got 0"},
		{"defun1", `(defun f)`, "ARITY_ERROR: defun expects at least 3 args, got 1"},
		{"http_server0", `(http_server)`, "ARITY_ERROR: http_server expects at least 1 arg, got 0"},
		{"str_trim_in_print", `(print (str_trim))`, "ARITY_ERROR: str_trim expects 1 arg, got 0"},
		{"exec0", `(print (exec))`, "ARITY_ERROR: exec expects at least 1 arg, got 0"},
		{"spawn_agent0", `(spawn_agent)`, "ARITY_ERROR: spawn_agent expects 2 args, got 0"},
		{"spawn_agent1", `(spawn_agent "x")`, "ARITY_ERROR: spawn_agent expects 2 args, got 1"},
		{"llm_generate0", `(print (llm_generate))`, "ARITY_ERROR: llm_generate expects"},
		{"sql_query0", `(print (sql_query))`, "ARITY_ERROR: sql_query expects"},
		{"sql_query1", `(print (sql_query db))`, "ARITY_ERROR: sql_query expects"},
		{"db_connect0", `(db_connect)`, "ARITY_ERROR: db_connect expects 3 args, got 0"},
		{"db_connect1", `(db_connect db)`, "ARITY_ERROR: db_connect expects 3 args, got 1"},
		{"db_connect2", `(db_connect db "sqlite3")`, "ARITY_ERROR: db_connect expects 3 args, got 2"},
		{"lazy_synthesize0", `(lazy_synthesize)`, "ARITY_ERROR: lazy_synthesize expects 3 args, got 0"},
		{"lazy_synthesize1", `(lazy_synthesize f)`, "ARITY_ERROR: lazy_synthesize expects 3 args, got 1"},
		{"lazy_synthesize2", `(lazy_synthesize f ())`, "ARITY_ERROR: lazy_synthesize expects 3 args, got 2"},
		{"confidence0", `(print (confidence))`, "ARITY_ERROR: confidence expects"},
		{"fetch1", `(print (fetch "http://127.0.0.1:1/"))`, "ARITY_ERROR: fetch expects 2 to 3 args, got 1"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertArityFailClosed(t, bin, dir, tc.name, issue141Source(tc.form), tc.want)
		})
	}
}

// issue141TopLevel heads are program roots, not statements inside cli_app.
var issue141TopLevel = map[string]bool{"cli_app": true, "web_app": true, "wasm_app": true}

func issue141ProgramFor(head string, n int) string {
	if issue141TopLevel[head] {
		return issue141Form(head, n) + "\n"
	}
	return issue141Source(issue141Form(head, n))
}

func issue141SortedTable() ([]string, map[string]construct.Arity) {
	table := construct.ArityTable()
	names := make([]string, 0, len(table))
	for n := range table {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, table
}

// TestIssue141EveryArityEntry drives one too-few and one too-many call for
// every table entry through both build profiles.
func TestIssue141EveryArityEntry(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	names, table := issue141SortedTable()
	checked := 0
	for _, name := range names {
		a := table[name]
		var bad []int
		if a.Min > 0 {
			bad = append(bad, a.Min-1)
		}
		if a.Max != construct.Variadic {
			bad = append(bad, a.Max+1)
		}
		for _, n := range bad {
			name, n := name, n
			checked++
			want := fmt.Sprintf("ARITY_ERROR: %s expects", name)
			t.Run(fmt.Sprintf("%s/%d", name, n), func(t *testing.T) {
				t.Parallel()
				id := fmt.Sprintf("e%x_%d", []byte(name), n)
				assertArityFailClosed(t, bin, dir, id, issue141ProgramFor(name, n), want)
			})
		}
	}
	if checked < len(table) {
		t.Fatalf("only %d wrong-arity cases for %d entries", checked, len(table))
	}
}

// TestIssue141ArityFuzzNoPanic tries arities 0..N for every registry
// built-in on every build path and asserts the CLI never panics. Only build
// commands run, so no program is executed (no live LLM, network or exec);
// -run is exercised only with arities the table rejects.
func TestIssue141ArityFuzzNoPanic(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	var names []string
	for _, e := range construct.Table() {
		names = append(names, e.Name)
	}
	table := construct.ArityTable()
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []string
	runs := 0
	for _, name := range names {
		a, known := table[name]
		top := 4
		if known && a.Min+1 > top {
			top = a.Min + 1
		}
		for n := 0; n <= top; n++ {
			name, n := name, n
			id := fmt.Sprintf("f%x_%d", []byte(name), n)
			source := filepath.Join(dir, id+".howl")
			if err := os.WriteFile(source, []byte(issue141ProgramFor(name, n)), 0o600); err != nil {
				t.Fatal(err)
			}
			cmds := [][]string{
				{"build", source, "-o", filepath.Join(dir, id+".bc")},
				{"build", "--profile", "governed", source, "-o", filepath.Join(dir, id+".gbc")},
				{"-compile-hfir-bc", source, "-o", filepath.Join(dir, id+".hbc")},
				{"build", "--target=go", source, "-o", filepath.Join(dir, id+"-go")},
				{"build", "--target=js", source, "-o", filepath.Join(dir, id+"-js")},
			}
			if known && (n < a.Min || (a.Max != construct.Variadic && n > a.Max)) {
				cmds = append(cmds, []string{"-run", source})
			}
			for _, args := range cmds {
				args := args
				runs++
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()
					out, code := issue141Run(bin, args...)
					if issue141Panicked(out) || code == -1 || code == 2 {
						mu.Lock()
						failures = append(failures, fmt.Sprintf("%s n=%d %v: exit %d %s", name, n, args[:len(args)-1], code, firstLine(out)))
						mu.Unlock()
					}
				}()
			}
		}
	}
	wg.Wait()
	sort.Strings(failures)
	for _, f := range failures {
		t.Error(f)
	}
	t.Logf("%d CLI runs over %d registry built-ins", runs, len(names))
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			if len(l) > 200 {
				return l[:200]
			}
			return l
		}
	}
	return ""
}
