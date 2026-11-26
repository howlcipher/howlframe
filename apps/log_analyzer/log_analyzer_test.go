package log_analyzer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogAnalyzer(t *testing.T) {
	tmp := t.TempDir()
	artifact := filepath.Join(tmp, "log_analyzer.hfbc")
	// Compile
	cmd := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", "log_analyzer.howl", "-o", artifact)
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to compile: %v", err)
	}

	// Clean log
	cleanLog := filepath.Join(tmp, "clean.log")
	if err := os.WriteFile(cleanLog, []byte("INFO: Start\nINFO: End\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", artifact, cleanLog)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("clean log failed: %v, out: %s", err, out)
	}
	if !strings.Contains(string(out), "status=CLEAN") {
		t.Errorf("expected CLEAN, got: %s", out)
	}

	// Err log
	errLog := filepath.Join(tmp, "err.log")
	if err := os.WriteFile(errLog, []byte("ERROR: Failed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", artifact, errLog)
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected err log to fail with exit 1")
	}
	if !strings.Contains(string(out), "status=ATTENTION") {
		t.Errorf("expected ATTENTION, got: %s", out)
	}

	// Missing file
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", artifact, filepath.Join(tmp, "missing.log"))
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected missing to fail with exit 2")
	}
	if !strings.Contains(string(out), "error=") {
		t.Errorf("expected error=, got: %s", out)
	}

	// Capability denied
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", artifact, cleanLog)
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
	staticSrc := filepath.Join(tmp, "static.howl")
	staticOut := filepath.Join(tmp, "static.hfbc")
	if err := os.WriteFile(staticSrc, []byte(staticScript), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("go", "run", "../../howlframe.go", "-compile-bc", staticSrc, "-o", staticOut)
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "str_split argument 1 must be string") {
		t.Errorf("expected static str_split rejection, got err=%v out=%s", err, out)
	}
	if _, statErr := os.Stat(staticOut); statErr == nil {
		t.Errorf("a rejected program must not emit bytecode")
	}

	// The runtime guard still fails closed when the type is hidden from the
	// checker behind an untyped (any) parameter.
	badScript := `(cli_app
		(defun split_lines ((value any)) list (return (str_split value "\n")))
		(let (content (read_file ` + `"` + cleanLog + `"` + `))
			(print (call split_lines content))
		)
	)`
	badSrc := filepath.Join(tmp, "bad.howl")
	badOut := filepath.Join(tmp, "bad.hfbc")
	if err := os.WriteFile(badSrc, []byte(badScript), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "run", "../../howlframe.go", "-compile-bc", badSrc, "-o", badOut).CombinedOutput(); err != nil {
		t.Fatalf("compile bad.howl: %v\n%s", err, out)
	}
	cmd = exec.Command("go", "run", "../../howlframe.go", "-run-bc", "-allow-caps", "filesystem", badOut)
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Errorf("expected type error to fail")
	}
	if !strings.Contains(string(out), "TYPE_ERROR") || strings.Contains(string(out), "interface {} is []interface {}, not string") {
		t.Errorf("expected structured TYPE_ERROR without Go panic text, got: %s", out)
	}
}
