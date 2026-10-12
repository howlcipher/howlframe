package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for the correctness bugs found by the held-out benchmark
// (docs/journals/2026-10-12_heldout_correctness_fixes.md). Each program runs
// on every applicable execution path: the AST interpreter, AST bytecode built
// under the governed profile (-run-bc), and the experimental HFIR bytecode.

type heldoutPath struct {
	name string
	run  func(t *testing.T, bin, dir, src string) (string, error)
}

func heldoutPaths() []heldoutPath {
	caps := func(dir string) string {
		return "filesystem:read=" + dir + ",filesystem:write=" + dir
	}
	runBC := func(bin, dir, art string) (string, error) {
		out, err := exec.Command(bin, "-run-bc", "-allow-caps", caps(dir), art).CombinedOutput()
		return string(out), err
	}
	return []heldoutPath{
		{"interpreter", func(t *testing.T, bin, dir, src string) (string, error) {
			out, err := exec.Command(bin, "-run", "-allow-caps", caps(dir), src).CombinedOutput()
			return string(out), err
		}},
		{"bytecode_governed", func(t *testing.T, bin, dir, src string) (string, error) {
			art := filepath.Join(dir, "g.hfbc")
			if out, err := exec.Command(bin, "build", "--profile", "governed", src, "-o", art).CombinedOutput(); err != nil {
				t.Fatalf("governed build: %v %s", err, out)
			}
			return runBC(bin, dir, art)
		}},
		{"hfir_bytecode", func(t *testing.T, bin, dir, src string) (string, error) {
			art := filepath.Join(dir, "h.hfbc")
			if out, err := exec.Command(bin, "-compile-hfir-bc", src, "-o", art).CombinedOutput(); err != nil {
				t.Fatalf("hfir compile: %v %s", err, out)
			}
			return runBC(bin, dir, art)
		}},
	}
}

func runHeldoutProgram(t *testing.T, bin, program, want string, after func(t *testing.T, dir string)) {
	t.Helper()
	for _, p := range heldoutPaths() {
		t.Run(p.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "p.howl")
			body := strings.ReplaceAll(program, "$DIR", dir)
			if err := os.WriteFile(src, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := p.run(t, bin, dir, src)
			if err != nil {
				t.Fatalf("run: %v %s", err, out)
			}
			if out != want {
				t.Fatalf("got %q want %q", out, want)
			}
			if after != nil {
				after(t, dir)
			}
		})
	}
}

// Bug 1: shorthand nested inside value-taking built-in arguments expands under
// the governed profile.
func TestHeldoutNestedShorthandExpands(t *testing.T) {
	bin := c1c2Binary(t)
	program := `(do
 (let (t "a foo b")
  (do
   (write_file "$DIR/out.txt" (str_replace t "foo" "bar"))
   (print (to_lower (str "AB" "C")) (read_text "$DIR/out.txt")))))`
	runHeldoutProgram(t, bin, program, "abc a bar b\n", nil)
}

// Bug 1, fail-closed half: a forbidden construct hidden in a nested shorthand
// argument is rejected by the governed build; no artifact, no effect.
func TestHeldoutNestedShorthandGovernedFailsClosed(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	side := filepath.Join(dir, "side")
	src := filepath.Join(dir, "x.howl")
	art := filepath.Join(dir, "x.hfbc")
	program := `(do (write_file "` + side + `" "ran") (write_file "` + side + `2" (str_replace (exec "touch" "` + side + `.exec") "a" "b")))`
	if err := os.WriteFile(src, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(bin, "build", "--profile", "governed", src, "-o", art).CombinedOutput(); err == nil {
		t.Fatalf("governed build accepted nested exec: %s", out)
	}
	for _, p := range []string{art, side, side + "2", side + ".exec"} {
		if _, err := os.Stat(p); err == nil {
			t.Fatalf("%s exists after a rejected governed build", p)
		}
	}
}

// Bug 2: and/or short-circuit. The right side is not evaluated when the left
// decides, so its effects (write_file) and its errors (to_int "qq") never run.
func TestHeldoutShortCircuitSkipsRightSide(t *testing.T) {
	bin := c1c2Binary(t)
	program := `(do
 (print (and (= 1 2) (do (write_file "$DIR/and.txt" "ran") (= 1 1)))
        (or (= 1 1) (do (write_file "$DIR/or.txt" "ran") (= 1 1)))
        (and (= 1 2) (= (to_int "qq") 1))
        (or (= 1 1) (= (to_int "zz") 1)))
 (print (and (= 1 1) (do (write_file "$DIR/and_rhs.txt" "ran") (= 2 2)))
        (or (= 1 2) (= 2 3))))`
	runHeldoutProgram(t, bin, program, "false true false true\ntrue false\n", func(t *testing.T, dir string) {
		for _, name := range []string{"and.txt", "or.txt"} {
			if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
				t.Fatalf("short-circuited right side ran: %s exists", name)
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "and_rhs.txt")); err != nil {
			t.Fatalf("undecided right side did not run: %v", err)
		}
	})
}

// Bug 3: map_get of a missing key is nil, so is_nil and nz defaults work. nil
// renders as "" in text.
func TestHeldoutMapGetMissIsNil(t *testing.T) {
	bin := c1c2Binary(t)
	program := `(do
 (let (m (dict ("a" "x") ("e" "")))
  (do
   (print (is_nil (map_get m "b")) (is_nil (map_get m "e")) (= (map_get m "b") ""))
   (print (nz (map_get m "b") "dflt") (nz (map_get m "a") "dflt"))
   (print (str "[" (map_get m "b") "]") (map_get m "b")))))`
	runHeldoutProgram(t, bin, program, "true false false\ndflt x\n[] \n", nil)
}

// Bug 4: lists are values. append rebinds only the named variable; a list held
// in a map (or another variable) is unchanged until it is map_set back.
func TestHeldoutListsAreValues(t *testing.T) {
	bin := c1c2Binary(t)
	program := `(cli_app
  (let (m (dict ("xs" (list "a"))))
    (let (xs (map_get m "xs"))
      (let (ys xs)
        (do
          (append xs "b")
          (print "map" (list_len (map_get m "xs")))
          (print "alias" (list_len ys))
          (print "var" (list_len xs))
          (map_set m "xs" xs)
          (print "after" (list_len (map_get m "xs")))
          (print (str_join (map_get m "xs") ","))
        )))))`
	runHeldoutProgram(t, bin, program, "map 1\nalias 1\nvar 2\nafter 2\na,b\n", nil)
}
