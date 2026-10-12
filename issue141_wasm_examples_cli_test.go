package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIssue141WasmExamplesBuildOnGoJS is the regression for the #142 review:
// wasm_app bodies are statement positions, so the wasm examples must still
// build on --target=go|js and the output must compile.
func TestIssue141WasmExamplesBuildOnGoJS(t *testing.T) {
	bin := c1c2Binary(t)
	repo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range []string{"wasm_app_hello", "wasm_app_print", "wasm_math"} {
		for _, target := range []string{"go", "js"} {
			ex, target := ex, target
			t.Run(ex+"/"+target, func(t *testing.T) {
				t.Parallel()
				outDir := filepath.Join(t.TempDir(), "out")
				out, code := issue141Run(bin, "build", "--target="+target, filepath.Join(repo, "examples", ex+".howl"), "-o", outDir)
				if code != 0 || !strings.Contains(out, "Built") {
					t.Fatalf("build rejected: exit %d %s", code, out)
				}
				if target == "js" {
					if _, err := exec.LookPath("node"); err != nil {
						t.Skip("node not installed")
					}
					files, _ := filepath.Glob(filepath.Join(outDir, "*.js"))
					if len(files) == 0 {
						t.Fatal("no .js output")
					}
					for _, f := range files {
						if b, err := exec.Command("node", "--check", f).CombinedOutput(); err != nil {
							t.Fatalf("node --check %s: %s", f, b)
						}
					}
					return
				}
				mod := "module probe\n\ngo 1.21\n\nrequire github.com/howlcipher/howlframe v0.0.0\n\nreplace github.com/howlcipher/howlframe => " + repo + "\n"
				os.WriteFile(filepath.Join(outDir, "go.mod"), []byte(mod), 0o600)
				cmd := exec.Command("go", "vet", ".")
				cmd.Dir = outDir
				cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod")
				if b, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go vet: %s", b)
				}
			})
		}
	}
}
