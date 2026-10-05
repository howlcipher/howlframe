package receipt_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSwarmNestedDogfoodHFBC(t *testing.T) {
	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "howlframe")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, out)
	}
	toolDir := t.TempDir()
	invoked := filepath.Join(toolDir, "invoked")
	for _, name := range []string{"go", "node", "wat2wasm"} {
		body := "#!/bin/sh\necho \"$0\" >> \"" + invoked + "\"\nexit 99\n"
		if err := os.WriteFile(filepath.Join(toolDir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	workDir := t.TempDir()
	env := toolchainFreeEnv(t, toolDir)
	run := func(args ...string) (int, string, string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir, cmd.Env = workDir, env
		var out, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &stderr
		err := cmd.Run()
		if ctx.Err() != nil {
			t.Fatalf("%v timed out: %s %s", args, &out, &stderr)
		}
		code := 0
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		if _, err := os.Stat(invoked); !os.IsNotExist(err) {
			data, _ := os.ReadFile(invoked)
			t.Fatalf("toolchain invoked: %s (stat: %v)", data, err)
		}
		return code, out.String(), stderr.String()
	}
	source := filepath.Join(repoRoot, "examples", "swarm_nested_dogfood", "swarm_nested_dogfood.howl")
	want := "swarm start\n"
	for _, line := range []string{
		"[Swarm VM] Spawning agent \"Outer\" for task: \"outer work\"",
		"outer before",
		"[Swarm VM] Spawning agent \"Inner\" for task: \"inner work\"",
		"inner body",
		"[Swarm VM] Agent \"Inner\" completed task: \"inner work\"",
		"outer after",
		"[Swarm VM] Agent \"Outer\" completed task: \"outer work\"",
		"[Swarm VM] Spawning agent \"Denied\" for task: \"read environment\"",
		"swarm done",
	} {
		want += line + "\n"
	}
	wantErr := "[Swarm VM] Agent \"Denied\" failed task: \"read environment\": CAPABILITY_DENIED: capability denied: environment\n"
	for _, mode := range []string{"compile-bc", "build"} {
		t.Run(mode, func(t *testing.T) {
			artifact := filepath.Join(workDir, mode+".hfbc")
			args := []string{"-compile-bc", source, "-o", artifact}
			if mode == "build" {
				args = []string{"build", source, "-o", artifact}
			}
			if code, out, stderr := run(args...); code != 0 || stderr != "" {
				t.Fatalf("compile: exit %d stdout %q stderr %q", code, out, stderr)
			}
			data, err := os.ReadFile(artifact)
			if err != nil || len(data) == 0 {
				t.Fatalf("nonempty artifact: %v", err)
			}
			if code, out, stderr := run("-run-bc", "-allow-caps", "process", artifact); code != 0 || out != want || stderr != wantErr {
				t.Fatalf("run: exit %d stdout %q stderr %q; want %q", code, out, stderr, want)
			}
			code, out, stderr := run("-run-bc", artifact)
			if code != 1 || out != "swarm start\n" {
				t.Fatalf("denied run: exit %d stdout %q stderr %q", code, out, stderr)
			}
			var failure struct {
				Instruction int
				Phase       string
				Code        string
				Opcode      string
				Message     string
			}
			if err := json.Unmarshal([]byte(stderr), &failure); err != nil {
				t.Fatalf("invalid JSON: %v: %q", err, stderr)
			}
			if failure.Instruction != 3 || failure.Phase != "runtime" || failure.Code != "CAPABILITY_DENIED" || failure.Opcode != "SPAWN_AGENT" || failure.Message != "capability denied: process" {
				t.Fatalf("unexpected failure: %#v", failure)
			}
		})
	}
}
