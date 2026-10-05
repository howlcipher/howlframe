package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBarePathUnnamedRootsFailClosed proves leftover/unnamed roots on the
// bare/default path never fall through to gogen/server.go. They exit nonzero
// with a clear diagnostic and leave no Go artifacts.
func TestBarePathUnnamedRootsFailClosed(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", binaryPath, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "defun_root",
			source: `(defun main () (print "hi"))`,
			want:   "Expected http_server, cli_app, web_app, or wasm_app as root symbol, got defun",
		},
		{
			name:   "module_root",
			source: `(module (export (defun x () (return 1))))`,
			want:   "Expected http_server, cli_app, web_app, or wasm_app as root symbol, got module",
		},
		{
			name:   "unknown_root",
			source: `(totally_unknown (print "x"))`,
			want:   "Expected http_server, cli_app, web_app, or wasm_app as root symbol, got totally_unknown",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			input := filepath.Join(workDir, tc.name+".howl")
			if err := os.WriteFile(input, []byte(tc.source), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binaryPath, input)
			cmd.Dir = workDir
			output, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected fail-closed rejection, got success:\n%s", output)
			}
			if !strings.Contains(string(output), tc.want) {
				t.Fatalf("diagnostic = %q, want substring %q", output, tc.want)
			}
			for _, name := range []string{"server.go", "server_test.go", tc.name + ".hfbc"} {
				if _, statErr := os.Stat(filepath.Join(workDir, name)); !os.IsNotExist(statErr) {
					t.Fatalf("unexpected artifact %s after fail-closed rejection: %v", name, statErr)
				}
			}
		})
	}
}

// TestBarePathNamedRootsStillWriteHFBC proves the four named roots keep writing
// .hfbc on the bare/default path and still do not emit server.go.
func TestBarePathNamedRootsStillWriteHFBC(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", binaryPath, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	cases := []struct {
		name   string
		source string
		stem   string
		runVM  bool
	}{
		{
			name:   "cli_app",
			source: `(cli_app (print "ok"))`,
			stem:   "cli",
			runVM:  true,
		},
		{
			name:   "http_server",
			source: "(http_server 8080\n  (route \"/\" (lambda (req)\n    (res 200 \"text/plain\" \"hi\")\n  ))\n)",
			stem:   "http",
		},
		{
			name:   "web_app",
			source: "(web_app\n  (spawn_agent \"JS_Researcher\" (task \"find sources\"))\n  (spawn_agent \"JS_Writer\" (task \"summarize\"))\n)",
			stem:   "web",
		},
		{
			name:   "wasm_app",
			source: `(wasm_app (+ 1 2))`,
			stem:   "wasm",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			input := filepath.Join(workDir, tc.stem+".howl")
			if err := os.WriteFile(input, []byte(tc.source), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(binaryPath, "-o", workDir, input)
			cmd.Dir = workDir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("named root bare path failed: %v\n%s", err, output)
			}
			artifact := filepath.Join(workDir, tc.stem+".hfbc")
			info, statErr := os.Stat(artifact)
			if statErr != nil {
				t.Fatalf("%s.hfbc missing: %v\n%s", tc.stem, statErr, output)
			}
			if info.Size() == 0 {
				t.Fatalf("%s.hfbc is empty", tc.stem)
			}
			for _, name := range []string{"server.go", "server_test.go", "app.js", "app.wat"} {
				if _, goErr := os.Stat(filepath.Join(workDir, name)); !os.IsNotExist(goErr) {
					t.Fatalf("unexpected %s for %s: %v", name, tc.name, goErr)
				}
			}
			if tc.runVM {
				// Flagged -o writes and stops for cli_app; no-flag also runs.
				runDir := t.TempDir()
				runInput := filepath.Join(runDir, tc.stem+".howl")
				if err := os.WriteFile(runInput, []byte(tc.source), 0o644); err != nil {
					t.Fatal(err)
				}
				run := exec.Command(binaryPath, runInput)
				run.Dir = runDir
				runOut, runErr := run.CombinedOutput()
				if runErr != nil {
					t.Fatalf("cli_app no-flag run failed: %v\n%s", runErr, runOut)
				}
				if string(runOut) != "ok\n" {
					t.Fatalf("cli_app stdout = %q, want ok newline", runOut)
				}
				if _, err := os.Stat(filepath.Join(runDir, tc.stem+".hfbc")); err != nil {
					t.Fatalf("cli_app no-flag missing .hfbc: %v", err)
				}
				if _, err := os.Stat(filepath.Join(runDir, "server.go")); !os.IsNotExist(err) {
					t.Fatalf("cli_app no-flag wrote server.go: %v", err)
				}
			}
		})
	}
}

// TestBarePathExplicitGoTargetStillEmitsServerGo proves build --target=go is
// unchanged: the bare-path fail-closed does not remove explicit Go emission.
func TestBarePathExplicitGoTargetStillEmitsServerGo(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", binaryPath, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	workDir := t.TempDir()
	input := filepath.Join(workDir, "cli.howl")
	if err := os.WriteFile(input, []byte(`(cli_app (print "ok"))`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binaryPath, "build", "--target=go", "-o", workDir, input)
	cmd.Dir = workDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build --target=go failed: %v\n%s", err, output)
	}
	generated, err := os.ReadFile(filepath.Join(workDir, "server.go"))
	if err != nil {
		t.Fatalf("server.go missing after explicit Go target: %v", err)
	}
	if !strings.Contains(string(generated), `fmt.Println("ok")`) {
		t.Fatalf("explicit Go target omitted print: %s", generated)
	}
}
