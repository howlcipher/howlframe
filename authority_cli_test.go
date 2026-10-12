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

	"github.com/howlcipher/howlframe/internal/vm"
)

// Authority hardening: exact-argv process allow-lists and S9 attenuation for
// spawn_agent children. Every case runs on the interpreter (-run), the
// production bytecode path (-compile-bc + -run-bc) and the experimental HFIR
// lowering (-compile-hfir-bc + -run-bc).

type authorityRun struct {
	code           int
	stdout, stderr string
	receipt        *vm.ExecutionReceipt
}

func authorityPaths() []string { return []string{"interpreter", "bytecode", "hfir"} }

func runAuthority(t *testing.T, binary, path, source, grant string) authorityRun {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "prog.howl")
	if err := os.WriteFile(src, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	var args []string
	receiptPath := ""
	switch path {
	case "interpreter":
		args = []string{"-allow-caps", grant, "-run", src}
	case "bytecode", "hfir":
		flag := "-compile-bc"
		if path == "hfir" {
			flag = "-compile-hfir-bc"
		}
		artifact := filepath.Join(dir, "prog.hfbc")
		if out, err := exec.Command(binary, flag, src, "-o", artifact).CombinedOutput(); err != nil {
			return authorityRun{code: -1, stdout: string(out)}
		}
		receiptPath = filepath.Join(dir, "receipt.json")
		args = []string{"-allow-caps", grant, "-receipt", receiptPath, "-run-bc", artifact}
	}
	if grant == "" {
		args = args[2:]
	}
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := authorityRun{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		e, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		r.code = e.ExitCode()
	}
	if receiptPath != "" {
		if data, err := os.ReadFile(receiptPath); err == nil {
			var rec vm.ExecutionReceipt
			if err := json.Unmarshal(data, &rec); err != nil {
				t.Fatalf("receipt: %v %s", err, data)
			}
			r.receipt = &rec
		}
	}
	return r
}

// requireDenied asserts a typed CAPABILITY_DENIED failure. On the bytecode
// paths the receipt must record the denied effect for the given opcode.
func requireDenied(t *testing.T, path string, r authorityRun, opcode string, forbid ...string) {
	t.Helper()
	all := r.stdout + r.stderr
	if r.code == 0 || !strings.Contains(all, "CAPABILITY_DENIED") {
		t.Fatalf("%s: want CAPABILITY_DENIED, got code %d stdout %q stderr %q", path, r.code, r.stdout, r.stderr)
	}
	for _, f := range forbid {
		if strings.Contains(r.stdout, f) {
			t.Fatalf("%s: denied run printed %q: %q", path, f, r.stdout)
		}
	}
	if path == "interpreter" {
		return // interpreter receipts are unsupported (--receipt requires -run-bc)
	}
	if r.receipt == nil {
		t.Fatalf("%s: no receipt written", path)
	}
	if r.receipt.Exit.Error == nil || r.receipt.Exit.Error.Code != "CAPABILITY_DENIED" || r.receipt.Exit.Error.Opcode != opcode {
		t.Fatalf("%s: receipt exit = %+v, want CAPABILITY_DENIED at %s", path, r.receipt.Exit, opcode)
	}
	found := false
	for _, e := range r.receipt.Effects {
		if e.Op == opcode && e.Capability == "process" && e.Decision == "denied" {
			found = true
		}
	}
	if !found {
		t.Fatalf("%s: receipt effects %+v lack a denied %s process effect", path, r.receipt.Effects, opcode)
	}
}

func canonicalProgram(t *testing.T, name string) string {
	t.Helper()
	for _, dir := range []string{"/usr/bin", "/bin"} {
		p := filepath.Join(dir, name)
		if resolved, err := filepath.EvalSymlinks(p); err == nil && resolved == p {
			return p
		}
	}
	t.Skipf("no canonical %s", name)
	return ""
}

func TestAuthorityExecAllowList(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	printf := canonicalProgram(t, "printf")
	marker := "authority-exec-marker"
	src := fmt.Sprintf(`(cli_app (print (bytes_to_string (exec %q "%%s" %q))))`, printf, marker)
	for _, path := range authorityPaths() {
		t.Run(path, func(t *testing.T) {
			// Exact allow-list entry runs.
			if r := runAuthority(t, binary, path, src, "process:exec="+printf+" %s "+marker); r.code != 0 || strings.TrimSpace(r.stdout) != marker {
				t.Fatalf("granted: code %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
			}
			// A bare process grant has no allow-list and fails closed.
			requireDenied(t, path, runAuthority(t, binary, path, src, "process"), "EXEC", marker)
			// The wrong program, a different argument, and no grant all deny.
			requireDenied(t, path, runAuthority(t, binary, path, src, "process:exec=/usr/bin/true **"), "EXEC", marker)
			requireDenied(t, path, runAuthority(t, binary, path, src, "process:exec="+printf+" %s other"), "EXEC", marker)
			r := runAuthority(t, binary, path, src, "")
			if r.code == 0 || !strings.Contains(r.stdout+r.stderr, "CAPABILITY_DENIED") && !strings.Contains(r.stdout+r.stderr, "capability denied") {
				t.Fatalf("no grant: code %d %q %q", r.code, r.stdout, r.stderr)
			}
		})
	}
}

func TestAuthorityExecArgvInjection(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	printf := canonicalProgram(t, "printf")
	grant := "process:exec=" + printf + " %s safe"
	for _, path := range authorityPaths() {
		t.Run(path, func(t *testing.T) {
			pwned := filepath.Join(t.TempDir(), "pwned")
			for name, src := range map[string]string{
				// Shell metacharacters are never interpreted and do not match the literal.
				"metachar": fmt.Sprintf(`(cli_app (print (bytes_to_string (exec %q "%%s" "safe; touch %s"))))`, printf, pwned),
				// An extra trailing argument is not covered by an exact pattern.
				"extra": fmt.Sprintf(`(cli_app (print (bytes_to_string (exec %q "%%s" "safe" "extra"))))`, printf),
				// Argument smuggling inside the program string does not split.
				"program-args": fmt.Sprintf(`(cli_app (print (bytes_to_string (exec "%s %%s safe"))))`, printf),
				// A missing argument does not match either.
				"short": fmt.Sprintf(`(cli_app (print (bytes_to_string (exec %q "%%s"))))`, printf),
			} {
				t.Run(name, func(t *testing.T) {
					requireDenied(t, path, runAuthority(t, binary, path, src, grant), "EXEC", "safe")
					if _, err := os.Stat(pwned); !os.IsNotExist(err) {
						t.Fatalf("injection created %s", pwned)
					}
				})
			}
		})
	}
}

func TestAuthorityExecProgramPath(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	printf := canonicalProgram(t, "printf")
	link := filepath.Join(t.TempDir(), "printf-link")
	if err := os.Symlink(printf, link); err != nil {
		t.Skip(err)
	}
	marker := "authority-path-marker"
	call := func(program string) string {
		return fmt.Sprintf(`(cli_app (print (bytes_to_string (exec %q %q))))`, program, marker)
	}
	for _, path := range authorityPaths() {
		t.Run(path, func(t *testing.T) {
			for name, tc := range map[string]struct{ program, grant string }{
				// Bare names never go through a PATH lookup.
				"bare name": {"printf", "process:exec=" + printf + " **"},
				// A relative path is refused even when it resolves to the program.
				"relative": {"./printf", "process:exec=" + printf + " **"},
				// A non-clean path that names the program is refused.
				"dotdot": {filepath.Join(filepath.Dir(printf), "..", filepath.Base(filepath.Dir(printf))) + "/./printf", "process:exec=" + printf + " **"},
				// Calling through a symlink to the granted program is refused.
				"symlink to granted": {link, "process:exec=" + printf + " **"},
				// Granting the symlink itself is refused: links can be swapped.
				"granted symlink": {link, "process:exec=" + link + " **"},
			} {
				t.Run(name, func(t *testing.T) {
					requireDenied(t, path, runAuthority(t, binary, path, call(tc.program), tc.grant), "EXEC", marker)
				})
			}
		})
	}
}

// spawn_agent runs only on the bytecode VM. The interpreter and the HFIR
// lowering refuse it outright, which is fail-closed; the cases still run
// there to prove no child body executes.
func TestAuthoritySpawnAgentAttenuation(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	printf := canonicalProgram(t, "printf")
	execGrant := "process:exec=" + printf + " **"
	cases := []struct {
		name, grant, source, want string
		denied, receiptDeny       bool
		failedChild               string
	}{
		{
			name:   "default no grants",
			grant:  "process,environment",
			source: `(cli_app (spawn_agent "Child" (task "t" (print "child-ran") (env "HOME"))) (print "parent-after"))`,
			// The child body starts but its env is denied: it inherited nothing.
			want:        "child-ran\nparent-after",
			failedChild: "CAPABILITY_DENIED: capability denied: environment",
		},
		{
			name:   "explicit subset",
			grant:  "process,environment",
			source: `(cli_app (spawn_agent "Child" (grants "environment") (task "t" (print (is_nil (env "HOWL_AUTHORITY_UNSET"))))))`,
			want:   "false",
		},
		{
			name:   "child requests more than parent",
			grant:  "process",
			source: `(cli_app (spawn_agent "Child" (grants "environment") (task "t" (print "child-ran"))))`,
			denied: true,
		},
		{
			name:   "child requests coarse filesystem from scoped parent",
			grant:  "process,filesystem:read=/tmp",
			source: `(cli_app (spawn_agent "Child" (grants "filesystem") (task "t" (print "child-ran"))))`,
			denied: true,
		},
		{
			name:   "child requests a wider exec pattern",
			grant:  "process," + "process:exec=" + printf + " x",
			source: fmt.Sprintf(`(cli_app (spawn_agent "Child" (grants %q) (task "t" (print "child-ran"))))`, execGrant),
			denied: true,
		},
		{
			name:  "nested escalation",
			grant: "process,environment",
			// Outer holds only process; Inner asks for environment, which the
			// root has but Outer was never handed.
			// The refusal is a typed error inside Outer, isolated like any child failure.
			source:      `(cli_app (spawn_agent "Outer" (grants "process") (task "o" (spawn_agent "Inner" (grants "environment") (task "i" (print "child-ran"))))) (print "parent-after"))`,
			want:        "parent-after",
			failedChild: `CAPABILITY_DENIED: capability denied: spawn_agent: child requested "environment", which the parent does not hold`,
			receiptDeny: true,
		},
		{
			name:  "nested spawn without process",
			grant: "process,environment",
			// Outer received nothing, so it cannot spawn at all.
			source:      `(cli_app (spawn_agent "Outer" (task "o" (spawn_agent "Inner" (task "i" (print "child-ran"))))) (print "parent-after"))`,
			want:        "parent-after",
			failedChild: "CAPABILITY_DENIED: capability denied: process",
		},
		{
			name:   "exec delegated exactly",
			grant:  "process," + execGrant,
			source: fmt.Sprintf(`(cli_app (spawn_agent "Child" (grants "process:exec=%s **") (task "t" (print (bytes_to_string (exec %q "delegated"))))))`, printf, printf),
			want:   "delegated",
		},
		{
			name:        "exec not delegated",
			grant:       "process," + execGrant,
			source:      fmt.Sprintf(`(cli_app (spawn_agent "Child" (grants "process") (task "t" (print (bytes_to_string (exec %q "child-ran"))))))`, printf),
			failedChild: "CAPABILITY_DENIED: capability denied: process: exec argv is not in the allow-list",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, path := range authorityPaths() {
				r := runAuthority(t, binary, path, tc.source, tc.grant)
				if strings.Contains(r.stdout, "child-ran") && (path != "bytecode" || tc.denied) {
					t.Fatalf("%s: child body ran: %q", path, r.stdout)
				}
				if path != "bytecode" {
					if r.code == 0 {
						t.Fatalf("%s: spawn_agent must be refused, got code 0 stdout %q", path, r.stdout)
					}
					continue
				}
				if tc.denied {
					requireDenied(t, path, r, "SPAWN_AGENT", "child-ran")
					continue
				}
				var lines []string
				for _, l := range strings.Split(strings.TrimSpace(r.stdout), "\n") {
					if !strings.HasPrefix(l, "[Swarm VM]") {
						lines = append(lines, l)
					}
				}
				if r.code != 0 || strings.Join(lines, "\n") != tc.want {
					t.Fatalf("code %d stdout %q stderr %q, want %q", r.code, r.stdout, r.stderr, tc.want)
				}
				if tc.failedChild != "" && !strings.Contains(r.stderr, tc.failedChild) {
					t.Fatalf("stderr %q, want %q", r.stderr, tc.failedChild)
				}
				if tc.receiptDeny {
					found := false
					for _, e := range r.receipt.Effects {
						found = found || e.Op == "SPAWN_AGENT" && e.Decision == "denied"
					}
					if !found {
						t.Fatalf("receipt effects %+v lack a denied SPAWN_AGENT", r.receipt.Effects)
					}
				}
				if tc.failedChild == "" && r.stderr != "" {
					t.Fatalf("unexpected stderr %q", r.stderr)
				}
			}
		})
	}
}

func TestAuthorityGrantsMustBeLiteral(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	for name, src := range map[string]string{
		"expression": `(cli_app (let (g "process") (spawn_agent "C" (grants g) (task "t" (print 1)))))`,
		"unknown":    `(cli_app (spawn_agent "C" (grants "root") (task "t" (print 1))))`,
		"relative":   `(cli_app (spawn_agent "C" (grants "process:exec=printf") (task "t" (print 1))))`,
		"not grants": `(cli_app (spawn_agent "C" (list "process") (task "t" (print 1))))`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "p.howl")
			if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(binary, "-compile-bc", path, "-o", filepath.Join(dir, "p.hfbc")).CombinedOutput(); err == nil {
				t.Fatalf("compiled: %s", out)
			}
		})
	}
}
