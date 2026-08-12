package todo_cli_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestTodoCLI(t *testing.T) {
	// Compile
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "todo_cli.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	runTodo := func(args ...string) (string, error) {
		cmdArgs := append([]string{"run", "../../howlframe.go", "-run-bc", "-allow-caps", "database", "todo_cli.howl.bc.bin"}, args...)
		cmd := exec.Command("go", cmdArgs...)
		cmd.Dir = "."
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 1. Lifecycle (Success path)
	out, err := runTodo("add", "Fix CI", "add", "Update docs", "list", "complete", "1", "get", "1", "delete", "2", "list")
	if err != nil {
		t.Fatalf("lifecycle failed: %v, out: %s", err, out)
	}

	expectedParts := []string{
		"ADDED 1 Fix CI",
		"ADDED 2 Update docs",
		"1 [ open ] Fix CI",
		"2 [ open ] Update docs",
		"COMPLETED 1",
		"1 [ done ] Fix CI",
		"DELETED 2",
	}

	for _, part := range expectedParts {
		if !strings.Contains(out, part) {
			t.Errorf("expected to find %q in output, got: %s", part, out)
		}
	}

	if strings.Contains(out, "2 [ open ] Update docs\n") && strings.Contains(out, "DELETED 2") {
		// Wait, after DELETED 2, the final 'list' should NOT contain "2 [open]".
		// Let's check the last part of the output specifically.
		finalListIdx := strings.LastIndex(out, "DELETED 2")
		if strings.Contains(out[finalListIdx:], "2 [") {
			t.Errorf("expected item 2 to be deleted in final list, but found it in output: %s", out)
		}
	}

	// 2. Malformed / Missing args
	out, err = runTodo("add")
	if err == nil || !strings.Contains(out, "ERROR: add requires a title") {
		t.Errorf("expected add error, got: %v, out: %s", err, out)
	}

	out, err = runTodo("get")
	if err == nil || !strings.Contains(out, "ERROR: get requires an id") {
		t.Errorf("expected get error, got: %v, out: %s", err, out)
	}

	out, err = runTodo("complete")
	if err == nil || !strings.Contains(out, "ERROR: complete requires an id") {
		t.Errorf("expected complete error, got: %v, out: %s", err, out)
	}

	out, err = runTodo("delete")
	if err == nil || !strings.Contains(out, "ERROR: delete requires an id") {
		t.Errorf("expected delete error, got: %v, out: %s", err, out)
	}

	// 3. Not found
	out, err = runTodo("get", "99")
	if err == nil || !strings.Contains(out, "ERROR: not found") {
		t.Errorf("expected not found error, got: %v, out: %s", err, out)
	}

	// 4. Capability denial
	cmdCap := exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", "todo_cli.howl.bc.bin")
	cmdCap.Dir = "."
	outCap, errCap := cmdCap.CombinedOutput()
	if errCap == nil || !strings.Contains(string(outCap), "capability denied: database") {
		t.Errorf("expected capability denied error, got: %v, out: %s", errCap, outCap)
	}

	// 5. Stress test (100 items)
	var stressArgs []string
	for i := 0; i < 100; i++ {
		stressArgs = append(stressArgs, "add", "Task")
	}
	out, err = runTodo(stressArgs...)
	if err != nil {
		t.Fatalf("stress test failed: %v, out: %s", err, out)
	}
	if !strings.Contains(out, "ADDED 100 Task") {
		t.Errorf("expected ADDED 100 Task in stress test, got: %s", out)
	}

	// Cleanup
	os.Remove("todo_cli.howl.bc.bin")
}
