package log_analyzer_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLogAnalyzer(t *testing.T) {
	// Compile
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "log_analyzer.howl")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	// Clean log
	os.WriteFile("clean.log", []byte("INFO: Start\nINFO: End\n"), 0644)
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", "log_analyzer.howl.bc.bin", "clean.log")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("clean log failed: %v, out: %s", err, out)
	}
	if !strings.Contains(string(out), "status=CLEAN") {
		t.Errorf("expected CLEAN, got: %s", out)
	}

	// Err log
	os.WriteFile("err.log", []byte("ERROR: Failed\n"), 0644)
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", "log_analyzer.howl.bc.bin", "err.log")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected err log to fail with exit 1")
	}
	if !strings.Contains(string(out), "status=ATTENTION") {
		t.Errorf("expected ATTENTION, got: %s", out)
	}

	// Missing file
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", "log_analyzer.howl.bc.bin", "missing.log")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected missing to fail with exit 2")
	}
	if !strings.Contains(string(out), "error=") {
		t.Errorf("expected error=, got: %s", out)
	}

	// Capability denied
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "log_analyzer.howl.bc.bin", "clean.log")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if !strings.Contains(string(out), "capability denied: filesystem") {
		t.Errorf("expected filesystem capability denied, got: %s", out)
	}

	// A provably wrong builtin argument (bytes from read_file into str_split)
	// is rejected by the checker before any bytecode exists (HFREC-010).
	staticScript := `(cli_app
		(let (content (read_file "clean.log"))
			(let (lines (str_split content "\n"))
				(print "should fail")
			)
		)
	)`
	os.WriteFile("static.howl", []byte(staticScript), 0644)
	cmd = exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "static.howl")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "str_split argument 1 must be string") {
		t.Errorf("expected static str_split rejection, got err=%v out=%s", err, out)
	}
	if _, statErr := os.Stat("static.howl.bc.bin"); statErr == nil {
		t.Errorf("a rejected program must not emit bytecode")
	}
	os.Remove("static.howl")

	// The runtime guard still fails closed when the type is hidden from the
	// checker behind an untyped (any) parameter.
	badScript := `(cli_app
		(defun split_lines ((value any)) list (return (str_split value "\n")))
		(let (content (read_file "clean.log"))
			(print (call split_lines content))
		)
	)`
	os.WriteFile("bad.howl", []byte(badScript), 0644)
	exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "bad.howl").Run()
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", "bad.howl.bc.bin")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected type error to fail")
	}
	if !strings.Contains(string(out), "TYPE_ERROR") || strings.Contains(string(out), "interface {} is []interface {}, not string") {
		t.Errorf("expected structured TYPE_ERROR without Go panic text, got: %s", out)
	}

	// Cleanup
	os.Remove("clean.log")
	os.Remove("err.log")
	os.Remove("bad.howl")
	os.Remove("bad.howl.bc.bin")
	os.Remove("err.log")
}
