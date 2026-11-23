package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// process:exec_inherit_env=<program> passes the runner's environment,
// unchanged, to exactly that program; every other exec still starts empty,
// child VMs can never receive it, and exec still needs its argv grant.
func TestAuthorityExecInheritEnv(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	envProg := canonicalProgram(t, "env")
	sh := canonicalProgram(t, "sh")
	src := fmt.Sprintf(`(cli_app (print (bytes_to_string (exec %q))))`, envProg)
	inherit := "process:exec=" + envProg + ",process:exec_inherit_env=" + envProg
	for _, path := range authorityPaths() {
		t.Run(path, func(t *testing.T) {
			r := runAuthorityEnv(t, binary, path, src, inherit, "HOWL_INHERIT_PROBE=operator-value")
			if r.code != 0 || !strings.Contains(r.stdout, "HOWL_INHERIT_PROBE=operator-value") || !strings.Contains(r.stdout, "PATH=") {
				t.Fatalf("inherited env: code %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
			}
			// Another program is not covered by the grant: it still starts empty.
			other := fmt.Sprintf(`(cli_app (print (bytes_to_string (exec %q "-c" "echo [$HOWL_INHERIT_PROBE][$PATH]"))))`, sh)
			r = runAuthorityEnv(t, binary, path, other, inherit+",process:exec="+sh+" -c *", "HOWL_INHERIT_PROBE=operator-value")
			if r.code != 0 || strings.Contains(r.stdout, "operator-value") {
				t.Fatalf("other program inherited: code %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
			}
			// The inherit grant alone authorizes nothing.
			requireDenied(t, path, runAuthority(t, binary, path, src, "process:exec_inherit_env="+envProg), "EXEC", "PATH=")
			if path == "bytecode" {
				if r.receipt == nil {
					t.Fatal("no receipt")
				}
				r = runAuthorityEnv(t, binary, path, src, inherit, "HOWL_INHERIT_PROBE=operator-value")
				found := false
				for _, e := range r.receipt.Effects {
					if e.Op == "EXEC" && e.Decision == "allowed" && e.EnvironmentInherited {
						found = true
					}
				}
				if !found {
					t.Fatalf("receipt does not mark the inherited environment: %+v", r.receipt.Effects)
				}
			}
		})
	}
}

// Child VMs never receive the runner's environment: spawn and spawn_agent
// requests for the grant are refused before the body runs.
func TestAuthorityExecInheritEnvIsRunnerOnly(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	envProg := canonicalProgram(t, "env")
	grants := "process,process:exec=" + envProg + ",process:exec_inherit_env=" + envProg
	spawn := fmt.Sprintf(`(cli_app (spawn (grants "process:exec=%s" "process:exec_inherit_env=%s") (lambda () (print "child-ran"))) (sleep 200) (print "parent-done"))`, envProg, envProg)
	r := runAuthority(t, binary, "bytecode", spawn, grants)
	if strings.Contains(r.stdout, "child-ran") {
		t.Fatalf("spawn received a runner-only grant: %q", r.stdout)
	}
	requireDenied(t, "bytecode", r, "SPAWN", "child-ran")
	agent := fmt.Sprintf(`(cli_app (spawn_agent "Child" (grants "process:exec_inherit_env=%s") (task "t" (print "child-ran"))))`, envProg)
	r = runAuthority(t, binary, "bytecode", agent, grants)
	if strings.Contains(r.stdout, "child-ran") {
		t.Fatalf("spawn_agent received a runner-only grant: %q", r.stdout)
	}
	// A child granted only the exec entry still gets an empty environment.
	plain := fmt.Sprintf(`(cli_app (spawn (grants "process:exec=%s") (lambda () (print (bytes_to_string (exec %q))))) (sleep 300) (print "parent-done"))`, envProg, envProg)
	t.Setenv("HOWL_INHERIT_PROBE", "operator-value")
	if r := runAuthority(t, binary, "bytecode", plain, grants); strings.Contains(r.stdout, "operator-value") || !strings.Contains(r.stdout, "parent-done") {
		t.Fatalf("spawned exec inherited: code %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
}

func TestAuthorityExecInheritEnvGrantSyntax(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "p.howl")
	if err := os.WriteFile(src, []byte(`(cli_app (print "ran"))`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"process:exec_inherit_env=", "process:exec_inherit_env=env", "process:exec_inherit_env=/usr/bin/../bin/env", "process:exec_inherit_env=/usr/bin/env extra"} {
		out, err := exec.Command(binary, "run", "--allow-caps", bad, src).CombinedOutput()
		if err == nil || strings.Contains(string(out), "ran") {
			t.Fatalf("%q accepted: %s", bad, out)
		}
	}
}
