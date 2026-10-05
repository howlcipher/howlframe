package receipt_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSwarmDogfoodHFBC(t *testing.T) {
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
	source := filepath.Join(repoRoot, "examples", "swarm_dogfood", "swarm_dogfood.howl")
	want := "swarm start\n"
	for _, pair := range [][2]string{{"Researcher", "collect sources"}, {"Writer", "draft summary"}, {"Reviewer", "check summary"}} {
		want += fmt.Sprintf("[Swarm VM] Spawning agent %q for task: %q\n[Swarm VM] Agent %q completed task: %q\n", pair[0], pair[1], pair[0], pair[1])
	}
	want += "swarm done\n"
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
			if code, out, stderr := run("-run-bc", "-allow-caps", "process", artifact); code != 0 || out != want || stderr != "" {
				t.Fatalf("run: exit %d stdout %q stderr %q; want %q", code, out, stderr, want)
			}
			code, out, stderr := run("-run-bc", artifact)
			if code != 1 || out != "swarm start\n" {
				t.Fatalf("denied run: exit %d stdout %q stderr %q", code, out, stderr)
			}
			var failure struct {
				Phase   string
				Code    string
				Opcode  string
				Message string
			}
			if err := json.Unmarshal([]byte(stderr), &failure); err != nil {
				t.Fatalf("invalid JSON: %v: %q", err, stderr)
			}
			if failure.Phase != "runtime" || failure.Code != "CAPABILITY_DENIED" || failure.Opcode != "SPAWN_AGENT" || failure.Message != "capability denied: process" {
				t.Fatalf("unexpected failure: %#v", failure)
			}
		})
	}
}
