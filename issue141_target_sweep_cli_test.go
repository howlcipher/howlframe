package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIssue141TargetNeverFalseBuilt builds one small program per construct
// family for --target=go and --target=js. Each build must either be rejected
// (no "Built") or produce output that compiles: go vet for Go, node --check
// for JS. A "Built" whose output does not compile is a false Built (#141).
func TestIssue141TargetNeverFalseBuilt(t *testing.T) {
	bin := c1c2Binary(t)
	repo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	_, nodeErr := exec.LookPath("node")
	probes := []struct{ name, body string }{
		{"all", `(let (xs (list 1 2)) (print (all x xs (> x 0))))`},
		{"any", `(let (xs (list 1 2)) (print (any x xs (> x 1))))`},
		{"ver_gt", `(print (ver_gt "1.2.3" "1.2.0"))`},
		{"ver_ge", `(print (ver_ge "1.2.3" "1.2.3"))`},
		{"if_value", `(let (a 1) (print (if (> a 0) "pos" "neg")))`},
		{"if_let_value", `(let (s (if true "y" "n")) (print s))`},
		{"list_get_expr", `(print (list_get (str_split "a,b" ",") 1))`},
		{"parse_json_expr", `(let (d (parse_json Any (str_trim "{}"))) (print d))`},
		{"cond", `(let (a 2) (print (cond ((= a 1) "one") ((= a 2) "two"))))`},
		{"switch", `(let (a 2) (switch a (1 (print "one")) (else (print "other"))))`},
		{"str", `(print (str "a" 1))`},
		{"str_replace", `(print (str_replace "foo" "o" "0"))`},
		{"str_contains", `(print (str_contains "foo" "o"))`},
		{"starts_with", `(print (starts_with "foo" "f"))`},
		{"ends_with", `(print (ends_with "foo" "o"))`},
		{"list_contains", `(let (xs (list "a")) (print (list_contains xs "a")))`},
		{"map_has", `(let (d (dict ("a" "b"))) (print (map_has d "a")))`},
		{"not", `(print (not true))`},
		{"blank", `(print (blank ""))`},
		{"when", `(let (a 1) (when (> a 0) (print "w")))`},
		{"unless", `(let (a 1) (unless (> a 5) (print "u")))`},
		{"inc", `(let (a 1) (do (inc a) (print a)))`},
		{"dec", `(let (a 1) (do (dec a) (print a)))`},
		{"unix_now", `(print (unix_now))`},
		{"lines", `(print (list_len (lines (read_file "x"))))`},
		{"read_text", `(print (read_text "x"))`},
		{"assert", `(assert (= 1 1))`},
		{"nz_stmt", `(let (d (dict ("a" "b"))) (print (nz (map_get d "a") "z")))`},
		{"plain", `(let (xs (list "a" "b")) (print (str_join xs ",")))`},
		{"time_now", `(print (time_now))`},
		{"tostr_time", `(print (to_string (time_now)))`},
	}
	for _, target := range []string{"go", "js"} {
		for _, p := range probes {
			target, p := target, p
			t.Run(target+"/"+p.name, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				src := filepath.Join(dir, "p.howl")
				os.WriteFile(src, []byte("(cli_app\n"+p.body+")\n"), 0o600)
				outDir := filepath.Join(dir, "out")
				out, code := issue141Run(bin, "build", "--target="+target, src, "-o", outDir)
				if issue141Panicked(out) {
					t.Fatalf("panic: %s", out)
				}
				if !strings.Contains(out, "Built") {
					if code == 0 {
						t.Fatalf("no Built but exit 0: %s", out)
					}
					return // rejected: fail closed
				}
				if target == "js" {
					if nodeErr != nil {
						t.Skip("node not installed")
					}
					files, _ := filepath.Glob(filepath.Join(outDir, "*.js"))
					if len(files) == 0 {
						t.Fatalf("Built but no .js output")
					}
					for _, f := range files {
						if b, err := exec.Command("node", "--check", f).CombinedOutput(); err != nil {
							t.Fatalf("false Built: node --check %s: %s", f, b)
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
					t.Fatalf("false Built: go vet: %s", b)
				}
			})
		}
	}
}
