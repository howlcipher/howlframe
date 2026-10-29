package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// An inner binding shadows an outer variable of the same name only inside
// its own scope. The direct interpreter and the JavaScript backend always
// scoped this way; the bytecode VM stored every binding in one environment
// per function, so an inner let, for or try_let overwrote the outer value,
// both from the production compiler and from the experimental HFIR lowering.
func TestBindingsShadowOnlyInsideTheirScope(t *testing.T) {
	bin := buildHowlFrameBinaryForTest(t)
	// vmOnly: the direct interpreter does not support try_let.
	cases := []struct {
		name, body, want string
		vmOnly           bool
	}{
		{"let in let", `(let (x 1) (do (let (x 2) (print x)) (print x)))`, "2\n1", false},
		{"let in if", `(let (x "outer") (do (if true (let (x "inner") (print x))) (print x)))`, "inner\nouter", false},
		{"for variable", `(let (x 10) (do (for x (list 1 2) (print x)) (print x)))`, "1\n2\n10", false},
		{"set inside reaches inner", `(let (x 1) (do (let (x 2) (do (set x 3) (print x))) (print x)))`, "3\n1", false},
		{"set outer stays outer", `(let (x 1) (do (let (y 2) (set x 5)) (print x)))`, "5", false},
		{"try_let names", `(let (r "outer" err "e") (do (try_let (r (to_int "7")) (catch err (print err)) (print r)) (print r err)))`, "7\nouter e", true},
		{"function parameter", `(defun f (x) (type_hint x "int") (type_hint return "int") (let (x (+ x 1)) (return x))) (print (call f 1))`, "2", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "p.howl")
			os.WriteFile(src, []byte("(cli_app "+tc.body+")\n"), 0o600)
			hfbc := filepath.Join(dir, "p.hfbc")
			if out, err := exec.Command(bin, "-compile-hfir-bc", src, "-o", hfbc).CombinedOutput(); err != nil {
				t.Fatalf("HFIR bytecode: %v\n%s", err, out)
			}
			runs := [][]string{{src}, {"-run-bc", hfbc}, {"run", "--target", "interpreter", src}}
			if tc.vmOnly {
				runs = runs[:2]
			}
			for _, args := range runs {
				out, err := exec.Command(bin, args...).CombinedOutput()
				if err != nil {
					t.Fatalf("%v: %v\n%s", args, err, out)
				}
				if got := strings.TrimSpace(string(out)); got != tc.want {
					t.Fatalf("%v: got %q, want %q", args[:len(args)-1], got, tc.want)
				}
			}
		})
	}
}
