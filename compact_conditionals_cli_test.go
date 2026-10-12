package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
)

// Compact-conditional built-ins (ver_gt, ver_ge, all, any, inline list_get and
// parse_json operands) must be pure: a program using only them needs no
// capability, and they compute the documented results.
func TestCompactConditionalBuiltinsArePureAndCorrect(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "p.howl")
	art := filepath.Join(dir, "p.hfbc")
	program := `(do
 (let (xs (list 1 2 3) kv (str_split "current=1.9.9\nproposed=1.10.0" "\n"))
  (do
   (print (all x xs (> x 0)) (all x xs (> x 1)) (all x (list) (= x 9)))
   (print (any x xs (= x 3)) (any x xs (= x 7)) (any x (list) (= x 9)))
   (print (ver_gt "1.10.0" "1.9.9") (ver_gt "1.2.3" "1.2.3") (ver_ge "1.2.3" "1.2.3") (ver_gt "0.9.9" "1.0.0") (ver_ge "2.0.0" "10.0.0") (ver_gt "1.2.10" "1.2.9"))
   (print (list_get (str_split "a.b.c" ".") 2) (ver_gt (list_get (str_split (list_get kv 1) "=") 1) (list_get (str_split (list_get kv 0) "=") 1)))
   (print (all x xs (any y (list 3 2 1) (= x y)))))))`
	if err := os.WriteFile(src, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"default", "governed"} {
		if out, err := exec.Command(bin, "check", "--profile", profile, src).CombinedOutput(); err != nil {
			t.Fatalf("check %s: %v %s", profile, err, out)
		}
	}
	if out, err := exec.Command(bin, "build", "--profile", "governed", src, "-o", art).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	out, err := exec.Command(bin, "inspect", "--caps", art).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect: %v %s", err, out)
	}
	var report bytecode.Report
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(report.Capabilities) != 0 {
		t.Fatalf("pure built-ins required capabilities %v", report.Capabilities)
	}
	run, err := exec.Command(bin, src).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v %s", err, run)
	}
	want := "true false true\ntrue false false\ntrue false true false false true\nc true\ntrue\n"
	if string(run) != want {
		t.Fatalf("got %q want %q", run, want)
	}
}

// Inline parse_json operands are accepted by the governed profile, but the
// operand is still scanned before anything is built or executed. A forbidden
// construct hidden in the operand must fail closed: diagnostic, no artifact,
// and no earlier effect in the same program runs.
func TestGovernedInlineParseJSONFailsClosed(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	side := filepath.Join(dir, "side")
	src := filepath.Join(dir, "x.howl")
	art := filepath.Join(dir, "x.hfbc")
	forms := map[string]string{
		"exec":         `(exec "touch" "` + side + `.exec")`,
		"llm_generate": `(llm_generate "x")`,
		"spawn":        `(spawn (lambda () (print "x")))`,
	}
	for name, operand := range forms {
		t.Run(name, func(t *testing.T) {
			program := `(do (write_file "` + side + `" "ran") (let (d (parse_json Any ` + operand + `)) (print d)))`
			os.WriteFile(src, []byte(program), 0600)
			for _, args := range [][]string{
				{"check", "--profile", "governed", src},
				{"build", "--profile", "governed", src, "-o", art},
			} {
				out, err := exec.Command(bin, args...).CombinedOutput()
				if err == nil || !strings.Contains(string(out), "PROFILE_FORBIDDEN_CONSTRUCT") || !strings.Contains(string(out), name) {
					t.Fatalf("%v accepted or wrong diagnostic: %v %s", args, err, out)
				}
				for _, p := range []string{art, side, side + ".exec"} {
					if _, err := os.Stat(p); !os.IsNotExist(err) {
						t.Fatalf("%s exists after rejected %v", p, args)
					}
				}
			}
		})
	}
	// The allowed shape still builds under governed.
	os.WriteFile(src, []byte(`(do (let (d (parse_json Any (read_text "in.json"))) (print d)))`), 0600)
	if out, err := exec.Command(bin, "build", "--profile", "governed", src, "-o", art).CombinedOutput(); err != nil {
		t.Fatalf("inline parse_json rejected: %v %s", err, out)
	}
}
