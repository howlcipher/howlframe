package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const corelibProgram = `(do
 (let (xs (list 5 3 9 1 3) ws (list "pear" "Apple" "fig" "pear"))
  (do
   (print (str_join (list_slice ws 1 3) ",") (str_join (list_slice ws (- 0 2) (list_len ws)) ",") (str_slice "hello" 1 4) (str_slice "hello" (- 0 3) 5) (str_len "hello"))
   (print (index_of ws "fig") (index_of ws "kiwi") (sum xs) (min xs) (max xs) (min ws) (sum (list)))
   (print (sum (list 1.5 2.25)) (sum (filter f (list 0.5) (> f 9.0))))
   (print (encode_json (map_list x xs (* x 2))) (encode_json (filter x xs (> x 3))) (reduce a 1 x xs (* a x)))
   (print (encode_json (reverse ws)) (encode_json (sort xs)) (encode_json (sort ws)) (encode_json (sort (list))))
   (print (encode_json (count_by w ws w)) (< "Apple" "apple") (>= "b" "a"))
   (for w (sort ws) (print w)))))`

const corelibWant = "Apple,fig fig,pear ell llo 5\n" +
	"2 -1 21 1 9 Apple 0\n" +
	"3.75 0\n" +
	"[10,6,18,2,6] [5,9] 405\n" +
	"[\"pear\",\"fig\",\"Apple\",\"pear\"] [1,3,3,5,9] [\"Apple\",\"fig\",\"pear\",\"pear\"] []\n" +
	"{\"Apple\":1,\"fig\":1,\"pear\":2} true true\n" +
	"Apple\nfig\npear\npear\n"

func assertPureProgramAllPaths(t *testing.T, program, want string, includeInterpreter, includeHFIR bool) {
	t.Helper()
	bin := c1c2Binary(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "p.howl")
	if err := os.WriteFile(src, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	paths := map[string][]string{
		"bytecode_governed": {"-run-bc", filepath.Join(dir, "p.hfbc")},
	}
	if includeInterpreter {
		paths["interpreter"] = []string{"-run", src}
	} else {
		paths["run"] = []string{src}
	}
	if includeHFIR {
		paths["hfir_bytecode"] = []string{"-run-bc", filepath.Join(dir, "h.hfbc")}
	}
	assertPureProgramBuiltinsAndOutputs(t, bin, src, paths, want)
}

// Core-library built-ins are pure sugar: a program using only them needs no
// capability under the governed profile, and computes the documented results
// on the interpreter, governed AST bytecode, and HFIR bytecode.
func TestCoreLibraryBuiltinsArePureOnEveryPath(t *testing.T) {
	assertPureProgramAllPaths(t, corelibProgram, corelibWant, true, true)
}

// Mixed string/number ordering stays a TYPE_ERROR; only string pairs gained
// byte-order comparison.
func TestStringOrderingRejectsMixedOperands(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "m.howl")
	os.WriteFile(src, []byte(`(do (let (xs (list "a" 1)) (print (< (list_get xs 0) 1))))`), 0600)
	if out, err := exec.Command(bin, "-run", src).CombinedOutput(); err == nil {
		t.Fatalf("mixed < succeeded: %s", out)
	}
}
