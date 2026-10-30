package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
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

// Core-library built-ins are pure sugar: a program using only them needs no
// capability under the governed profile, and computes the documented results
// on the interpreter, governed AST bytecode, and HFIR bytecode.
func TestCoreLibraryBuiltinsArePureOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "p.howl")
	art := filepath.Join(dir, "p.hfbc")
	hart := filepath.Join(dir, "h.hfbc")
	if err := os.WriteFile(src, []byte(corelibProgram), 0600); err != nil {
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
		t.Fatalf("pure core-library built-ins required capabilities %v", report.Capabilities)
	}
	if out, err := exec.Command(bin, "-compile-hfir-bc", src, "-o", hart).CombinedOutput(); err != nil {
		t.Fatalf("hfir compile: %v %s", err, out)
	}
	runs := map[string][]string{
		"interpreter":       {"-run", src},
		"bytecode_governed": {"-run-bc", art},
		"hfir_bytecode":     {"-run-bc", hart},
	}
	for name, args := range runs {
		t.Run(name, func(t *testing.T) {
			got, err := exec.Command(bin, args...).CombinedOutput()
			if err != nil {
				t.Fatalf("run: %v %s", err, got)
			}
			if string(got) != corelibWant {
				t.Fatalf("got %q want %q", got, corelibWant)
			}
		})
	}
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
