package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// HF-05: a value-less form in a value position used to pass check and build,
// perform its effect at run time, and then fail with STACK_UNDERFLOW. Every
// compile path must now refuse it before an artifact exists, so the effect
// can never happen; the same effect in statement position still runs.
func TestValuePositionVoidRejectedBeforeEffects(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	invalid := map[string]string{
		"try_let binding":  `(cli_app (try_let (r (write_file "fx.txt" "x")) (catch e (print "caught")) (print "bound")))`,
		"dict value":       `(cli_app (print (encode_json (dict ("k" (write_file "fx.txt" "x"))))))`,
		"return value":     `(cli_app (do (defun f () (return (write_file "fx.txt" "x"))) (print (call f))))`,
		"builtin argument": `(cli_app (print (read_file (write_file "fx.txt" "x"))))`,
		// The effect is an earlier statement: rejection must still come first.
		"if without else": `(cli_app (do (write_file "fx.txt" "x") (let (r (if false 1)) (print r))))`,
		"try_let body":    `(cli_app (let (x (try_let (r "v") (catch e "err") (write_file "fx.txt" "x"))) (print x)))`,
		"nested try_let":  `(cli_app (let (x (if true (try_let (r "v") (catch e "err") (write_file "fx.txt" "x")) "o")) (print x)))`,
	}
	for name, program := range invalid {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "p.howl")
			if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{
				{"check", source},
				{"build", source, "-o", filepath.Join(dir, "build.hfbc")},
				{"-compile-bc", source, "-o", filepath.Join(dir, "bc.hfbc")},
				{"-compile-hfir-bc", source, "-o", filepath.Join(dir, "hfir.hfbc")},
			} {
				cmd := exec.Command(binary, args...)
				cmd.Dir = dir
				out, err := cmd.CombinedOutput()
				if err == nil || !strings.Contains(string(out), "produces no value") {
					t.Errorf("%v accepted or wrong diagnostic: %v %s", args[0], err, out)
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Fatalf("rejected program left files behind: %v", entries)
			}
		})
	}

	dir := t.TempDir()
	source, artifact := filepath.Join(dir, "ok.howl"), filepath.Join(dir, "ok.hfbc")
	valid := `(cli_app (do (write_file "fx.txt" "x") (try_let (r (do (write_file "fx.txt" "y") 1)) (catch e (print "caught")) (print r))))`
	if err := os.WriteFile(source, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(binary, "-compile-bc", source, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("valid program rejected: %v %s", err, out)
	}
	cmd := exec.Command(binary, "-run-bc", "-allow-caps", "filesystem:write="+dir, artifact)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil || string(out) != "1\n" {
		t.Fatalf("valid program: %v %q", err, out)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "fx.txt")); err != nil || string(data) != "y" {
		t.Fatalf("statement-position writes: %q %v", data, err)
	}
}
