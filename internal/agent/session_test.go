package agent

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func sessionFixture(t *testing.T) (string, string, string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "project")
	os.Mkdir(root, 0700)
	c := exec.Command("git", "init", "-q", root)
	if b, e := c.CombinedOutput(); e != nil {
		t.Fatalf("git: %s %v", b, e)
	}
	for _, n := range []string{"main.howl", "go.sum", "config.json", "behavior_test.go"} {
		os.WriteFile(filepath.Join(root, n), []byte("original"), 0600)
	}
	c = exec.Command("git", "-C", root, "add", ".")
	if e := c.Run(); e != nil {
		t.Fatal(e)
	}
	c = exec.Command("git", "-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	if b, e := c.CombinedOutput(); e != nil {
		t.Fatalf("commit: %s %v", b, e)
	}
	compiler := filepath.Join(base, "compiler")
	os.WriteFile(compiler, []byte("compiler"), 0700)
	return root, filepath.Join(base, "bundle"), filepath.Join(base, "trust"), compiler
}
func invokeSession(t *testing.T, exe string, args ...string) (sessionResponse, int, []byte) {
	t.Helper()
	var b bytes.Buffer
	status := runSession(args, exe, &b, func(string, string, string) ([]byte, error) {
		return []byte(`{"ok":true,"scope":"compiler-check"}`), nil
	})
	var r sessionResponse
	if e := json.Unmarshal(b.Bytes(), &r); e != nil {
		t.Fatal(e)
	}
	return r, status, b.Bytes()
}
func snapshotArgs(root, bundle, trust string) []string {
	return []string{"snapshot", "--task", "task-1", "--root", root, "--bundle", bundle, "--state-dir", trust, "--source", "main.howl", "--claim", "owner claim", "--next", "review"}
}
func statusArgs(root, bundle, trust string) []string {
	return []string{"status", "--verified", "--task", "task-1", "--root", root, "--bundle", bundle, "--state-dir", trust}
}
func TestSessionRecoveryAndClaims(t *testing.T) {
	root, bundle, trust, exe := sessionFixture(t)
	r, s, _ := invokeSession(t, exe, snapshotArgs(root, bundle, trust)...)
	if s != 0 || !r.Verified {
		t.Fatalf("%+v", r)
	}
	args := statusArgs(root, bundle, trust)
	a, s, first := invokeSession(t, exe, args...)
	_, s2, second := invokeSession(t, exe, args...)
	if s != 0 || s2 != 0 || !bytes.Equal(first, second) || !a.Verified {
		t.Fatal(a)
	}
	args[0] = "context"
	args = append(args[:1], args[2:]...)
	r, s, _ = invokeSession(t, exe, args...)
	if s != 0 || r.Manifest == nil || len(r.Manifest.Claims) != 1 || len(r.Manifest.Next) != 1 {
		t.Fatal(r)
	}
	if _, s, _ := invokeSession(t, exe, snapshotArgs(root, bundle, trust)...); s == 0 {
		t.Fatal("overwrote evidence")
	}
}
func TestSessionStaleInventory(t *testing.T) {
	for _, name := range []string{"main.howl", "go.sum", "config.json", "behavior_test.go", "untracked.howl"} {
		t.Run(name, func(t *testing.T) {
			root, bundle, trust, exe := sessionFixture(t)
			if _, s, _ := invokeSession(t, exe, snapshotArgs(root, bundle, trust)...); s != 0 {
				t.Fatal("snapshot")
			}
			os.WriteFile(filepath.Join(root, name), []byte("changed"), 0600)
			if r, s, _ := invokeSession(t, exe, statusArgs(root, bundle, trust)...); s == 0 || r.Verified {
				t.Fatal("accepted stale input")
			}
		})
	}
}
func TestSessionTampering(t *testing.T) {
	for _, name := range []string{"manifest.json", "check.json", "authentication.json"} {
		t.Run(name, func(t *testing.T) {
			root, bundle, trust, exe := sessionFixture(t)
			invokeSession(t, exe, snapshotArgs(root, bundle, trust)...)
			os.WriteFile(filepath.Join(bundle, name), []byte(`{}`), 0600)
			if _, s, _ := invokeSession(t, exe, statusArgs(root, bundle, trust)...); s == 0 {
				t.Fatal("accepted tampering")
			}
		})
	}
}
func TestSessionTrustCompilerEnvironmentAndTask(t *testing.T) {
	for _, kind := range []string{"key", "compiler", "env", "task", "missing-evidence", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root, bundle, trust, exe := sessionFixture(t)
			invokeSession(t, exe, snapshotArgs(root, bundle, trust)...)
			args := statusArgs(root, bundle, trust)
			switch kind {
			case "key":
				os.Remove(filepath.Join(trust, "key"))
			case "compiler":
				os.WriteFile(exe, []byte("different"), 0700)
			case "env":
				t.Setenv("HOWLFRAME_ALLOW_CAPS", "changed")
			case "task":
				args[3] = "other"
			case "missing-evidence":
				os.Remove(filepath.Join(bundle, "check.json"))
			case "symlink":
				os.Remove(filepath.Join(root, "main.howl"))
				os.Symlink(filepath.Join(root, "go.sum"), filepath.Join(root, "main.howl"))
			}
			if _, s, _ := invokeSession(t, exe, args...); s == 0 {
				t.Fatal("accepted invalid state")
			}
			if kind == "key" {
				if _, e := os.Stat(filepath.Join(trust, "key")); !os.IsNotExist(e) {
					t.Fatal("verification created key")
				}
			}
		})
	}
}
func TestSessionInterruptedAndMutation(t *testing.T) {
	for _, mutate := range []bool{false, true} {
		root, bundle, trust, exe := sessionFixture(t)
		var out bytes.Buffer
		code := runSession(snapshotArgs(root, bundle, trust), exe, &out, func(string, string, string) ([]byte, error) {
			if mutate {
				os.WriteFile(filepath.Join(root, "main.howl"), []byte("edited"), 0600)
				return []byte(`{"ok":true}`), nil
			}
			return nil, os.ErrDeadlineExceeded
		})
		if code == 0 {
			t.Fatal("incomplete run passed")
		}
		if _, e := os.Stat(filepath.Join(bundle, "authentication.json")); !os.IsNotExist(e) {
			t.Fatal("published interrupted run")
		}
		if _, s, _ := invokeSession(t, exe, statusArgs(root, bundle, trust)...); s == 0 {
			t.Fatal("interrupted recovery passed")
		}
	}
}
func TestSessionArguments(t *testing.T) {
	for _, args := range [][]string{{"status"}, {"snapshot", "--root", "x"}, {"context", "--verified"}, {"status", "--verified", "--task", "a", "--task", "b"}} {
		if _, s, _ := invokeSession(t, "unused", args...); s == 0 {
			t.Fatal(args)
		}
	}
}

func TestSessionDependencyBoundary(t *testing.T) {
	for _, source := range []string{`(cli_app (include "../outside.howl"))`, `(cli_app (include"/tmp/external.howl"))`, `; include`, `(print "use")`, "(cli_app (use\u00a0\"ignored.howl\" as x))"} {
		root, bundle, trust, exe := sessionFixture(t)
		os.WriteFile(filepath.Join(root, "main.howl"), []byte(source), 0600)
		called := false
		var b bytes.Buffer
		status := runSession(snapshotArgs(root, bundle, trust), exe, &b, func(string, string, string) ([]byte, error) { called = true; return []byte(`{"ok":true}`), nil })
		if status == 0 || called {
			t.Fatal("uncovered dependency reached compiler")
		}
	}
}
func TestSessionUnsafePathsAndTrustModes(t *testing.T) {
	for _, kind := range []string{"root-link", "bundle-link", "trust-link", "public-trust", "public-key", "source-link", "nested-source-link", "inside-root"} {
		t.Run(kind, func(t *testing.T) {
			root, bundle, trust, exe := sessionFixture(t)
			args := snapshotArgs(root, bundle, trust)
			switch kind {
			case "root-link":
				p := root + "-link"
				os.Symlink(root, p)
				args[4] = p
			case "bundle-link":
				os.Symlink(root, bundle)
			case "trust-link":
				os.Symlink(root, trust)
			case "public-trust":
				os.Mkdir(trust, 0755)
			case "public-key":
				os.Mkdir(trust, 0700)
				os.WriteFile(filepath.Join(trust, "key"), make([]byte, 32), 0644)
			case "source-link":
				os.Remove(filepath.Join(root, "main.howl"))
				os.Symlink("go.sum", filepath.Join(root, "main.howl"))
			case "nested-source-link":
				os.Symlink(filepath.Dir(root), filepath.Join(root, "nested"))
				args[10] = "nested/compiler"
			case "inside-root":
				args[6] = filepath.Join(root, "bundle")
			}
			if _, s, _ := invokeSession(t, exe, args...); s == 0 {
				t.Fatal("accepted unsafe path or trust")
			}
		})
	}
}

func TestSessionIndexAndTrackedDeletion(t *testing.T) {
	root, bundle, trust, exe := sessionFixture(t)
	// A deleted tracked file is observable state, not an unreadable-file failure.
	os.Remove(filepath.Join(root, "config.json"))
	r, s, _ := invokeSession(t, exe, snapshotArgs(root, bundle, trust)...)
	if s != 0 {
		t.Fatal(r)
	}
	found := false
	for _, input := range r.Manifest.Inputs {
		if input.Path == "config.json" && input.SHA256 == "deleted" {
			found = true
		}
	}
	if !found {
		t.Fatal("deletion omitted")
	}
	// Staging the deletion changes index state even though source bytes are identical.
	if e := exec.Command("git", "-C", root, "add", "config.json").Run(); e != nil {
		t.Fatal(e)
	}
	if _, s, _ := invokeSession(t, exe, statusArgs(root, bundle, trust)...); s == 0 {
		t.Fatal("staged state ignored")
	}
}
func TestSessionAmbientUnrelatedEnvironment(t *testing.T) {
	root, bundle, trust, exe := sessionFixture(t)
	invokeSession(t, exe, snapshotArgs(root, bundle, trust)...)
	t.Setenv("PWD", "/different/recovery/directory")
	t.Setenv("SHLVL", "88")
	if r, s, _ := invokeSession(t, exe, statusArgs(root, bundle, trust)...); s != 0 {
		t.Fatal(r)
	}
}

func TestSessionHostileParentPATH(t *testing.T) {
	root, bundle, trust, exe := sessionFixture(t)
	hostile := t.TempDir()
	marker := filepath.Join(hostile, "executed")
	quotedMarker := "'" + strings.ReplaceAll(marker, "'", "'\"'\"'") + "'"
	script := []byte("#!/bin/sh\n: > " + quotedMarker + "\nexit 0\n")
	if e := os.WriteFile(filepath.Join(hostile, "git"), script, 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", hostile)
	if r, status, _ := invokeSession(t, exe, snapshotArgs(root, bundle, trust)...); status != 0 {
		t.Fatalf("trusted Git snapshot: %+v", r)
	}
	if r, status, _ := invokeSession(t, exe, statusArgs(root, bundle, trust)...); status != 0 {
		t.Fatalf("trusted Git recovery: %+v", r)
	}
	os.WriteFile(filepath.Join(root, "main.howl"), []byte("changed"), 0600)
	if _, status, _ := invokeSession(t, exe, statusArgs(root, bundle, trust)...); status == 0 {
		t.Fatal("hostile PATH hid source change")
	}
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("executed ambient Git shim")
	}
}
func TestSessionCompilerFixedEnvironment(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(dir, "environment")
	exe := filepath.Join(dir, "compiler")
	source := filepath.Join(dir, "source.howl")
	os.WriteFile(source, []byte(ExampleSource), 0600)
	quotedCapture := "'" + strings.ReplaceAll(capture, "'", "'\"'\"'") + "'"
	os.WriteFile(exe, []byte("#!/bin/sh\n/usr/bin/env > "+quotedCapture+"\nexit 1\n"), 0700)
	t.Setenv("HOWLFRAME_SESSION_SECRET", "must-not-be-inherited")
	_, _ = compileSessionCheck(exe, dir, source)
	data, e := os.ReadFile(capture)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(data, []byte("HOWLFRAME_SESSION_SECRET")) {
		t.Fatal("ambient environment reached compiler")
	}
	for _, want := range sessionEnvironment() {
		if !bytes.Contains(data, []byte(want+"\n")) {
			t.Fatalf("missing fixed environment %s: %s", want, data)
		}
	}
}
