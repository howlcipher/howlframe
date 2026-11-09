package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// This deliberately builds the file-explicit entry point used by consumers.
func TestAgentSessionCLI(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "howlframe")
	if b, e := exec.Command("go", "build", "-o", binary, "howlframe.go").CombinedOutput(); e != nil {
		t.Fatalf("build: %v %s", e, b)
	}
	root := filepath.Join(dir, "project")
	os.Mkdir(root, 0700)
	if b, e := exec.Command("git", "init", "-q", root).CombinedOutput(); e != nil {
		t.Fatalf("git: %v %s", e, b)
	}
	source := filepath.Join(root, "main.howl")
	os.WriteFile(source, []byte(`(cli_app (print (+ 20 22)))`), 0600)
	for _, argv := range [][]string{{"-C", root, "add", "."}, {"-C", root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"}} {
		if b, e := exec.Command("git", argv...).CombinedOutput(); e != nil {
			t.Fatalf("git: %v %s", e, b)
		}
	}
	bundle := filepath.Join(dir, "bundle")
	trust := filepath.Join(dir, "trust")
	common := []string{"--task", "trial", "--root", root, "--bundle", bundle, "--state-dir", trust}
	run := func(command string, extra ...string) ([]byte, bool) {
		t.Helper()
		argv := append([]string{"agent", command}, common...)
		argv = append(argv, extra...)
		b, e := exec.Command(binary, argv...).CombinedOutput()
		var r struct {
			Schema   string `json:"schema"`
			OK       bool   `json:"ok"`
			Verified bool   `json:"verified"`
		}
		if je := json.Unmarshal(b, &r); je != nil {
			t.Fatalf("invalid JSON: %s %v", b, je)
		}
		return b, e == nil && r.Schema == "howlframe.agent.session/v1" && r.OK && r.Verified
	}
	if b, ok := run("snapshot", "--source", "main.howl", "--claim", "not verified as a task"); !ok {
		t.Fatalf("snapshot: %s", b)
	}
	first, ok := run("status", "--verified")
	if !ok {
		t.Fatalf("status: %s", first)
	}
	second, ok := run("status", "--verified")
	if !ok || !bytes.Equal(first, second) {
		t.Fatal("unstable recovery")
	}
	if b, ok := run("context"); !ok {
		t.Fatalf("context: %s", b)
	}
	os.WriteFile(source, []byte(`(cli_app (print 43))`), 0600)
	if _, ok := run("status", "--verified"); ok {
		t.Fatal("stale source verified")
	}

	// Real compiler failure must leave evidence incomplete and recovery unverified.
	os.WriteFile(source, []byte(`(cli_app (append 1))`), 0600)
	common[5] = filepath.Join(dir, "failed-check-bundle")
	b, passed := run("snapshot", "--source", "main.howl")
	if passed || !bytes.Contains(b, []byte("AGENT_SESSION_CHECK")) {
		t.Fatalf("failed compiler check accepted: %s", b)
	}
	if _, passed = run("status", "--verified"); passed {
		t.Fatal("failed compiler evidence recovered as verified")
	}
	if _, err := os.Stat(filepath.Join(common[5], "authentication.json")); !os.IsNotExist(err) {
		t.Fatal("failed check published authentication")
	}
	// Ambient variables do not enter the compiler recipe, which is authenticated.
	os.WriteFile(source, []byte(`(cli_app (print 42))`), 0600)
	common[5] = filepath.Join(dir, "fixed-environment-bundle")
	t.Setenv("HOWLFRAME_SESSION_SECRET", "must-not-be-inherited")
	b, passed = run("snapshot", "--source", "main.howl")
	if !passed {
		t.Fatalf("fixed environment snapshot: %s", b)
	}
	var report struct {
		Manifest struct {
			ChildEnvironment []string `json:"child_environment"`
		} `json:"manifest"`
	}
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatal(err)
	}
	want := []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	if len(report.Manifest.ChildEnvironment) != len(want) {
		t.Fatalf("unexpected environment: %v", report.Manifest.ChildEnvironment)
	}
	for i, v := range want {
		if report.Manifest.ChildEnvironment[i] != v {
			t.Fatalf("environment recipe: %v", report.Manifest.ChildEnvironment)
		}
	}

	// Adjacent strings are valid lexer input; dependency rejection must happen before reading it.
	os.WriteFile(source, []byte(`(cli_app (include"/nonexistent/external.howl"))`), 0600)
	common[5] = filepath.Join(dir, "dependency-bundle")
	b, ok = run("snapshot", "--source", "main.howl")
	if ok || !bytes.Contains(b, []byte("dependency coverage unsupported")) {
		t.Fatalf("external dependency not rejected: %s", b)
	}
}
