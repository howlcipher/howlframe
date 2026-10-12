package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// #139: a governed build of (fetch "http://x") panicked the AST bytecode
// compiler (index out of range on the missing method child). It must fail
// closed with a diagnostic, nonzero exit, no panic and no artifact, under
// both profiles; the well-formed (fetch url method) still builds.
func TestIssue139FetchArityFailsClosed(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "f.howl")
	art := filepath.Join(dir, "f.hfbc")
	os.WriteFile(src, []byte(`(cli_app (fetch "http://x"))`), 0600)
	for _, profile := range []string{"governed", "default"} {
		out, err := exec.Command(bin, "build", "--profile", profile, src, "-o", art).CombinedOutput()
		if err == nil {
			t.Fatalf("%s: one-argument fetch built: %s", profile, out)
		}
		if strings.Contains(string(out), "panic") || strings.Contains(string(out), "goroutine") {
			t.Fatalf("%s: compiler panicked: %s", profile, out)
		}
		if !strings.Contains(string(out), "fetch expects (fetch url method [body])") {
			t.Fatalf("%s: missing diagnostic: %s", profile, out)
		}
		if _, err := os.Stat(art); err == nil {
			t.Fatalf("%s: artifact written after a rejected build", profile)
		}
	}
	good := filepath.Join(dir, "g.howl")
	os.WriteFile(good, []byte(`(cli_app (print (fetch "http://127.0.0.1:1" "GET")))`), 0600)
	if out, err := exec.Command(bin, "build", "--profile", "governed", good, "-o", art).CombinedOutput(); err != nil {
		t.Fatalf("well-formed fetch did not build: %v %s", err, out)
	}
}

// Empty min/max is a runtime error (VALUE_ERROR on stderr, exit 1), not a
// silent nil, on the interpreter, governed AST bytecode and HFIR bytecode.
func TestEmptyMinMaxIsValueError(t *testing.T) {
	bin := c1c2Binary(t)
	for _, op := range []string{"min", "max"} {
		t.Run(op, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "e.howl")
			art := filepath.Join(dir, "e.hfbc")
			hart := filepath.Join(dir, "h.hfbc")
			os.WriteFile(src, []byte(`(do (print (`+op+` (list 2 7 5))) (print (`+op+` (list))) (print "after"))`), 0600)
			if out, err := exec.Command(bin, "build", "--profile", "governed", src, "-o", art).CombinedOutput(); err != nil {
				t.Fatalf("build: %v %s", err, out)
			}
			if out, err := exec.Command(bin, "-compile-hfir-bc", src, "-o", hart).CombinedOutput(); err != nil {
				t.Fatalf("hfir: %v %s", err, out)
			}
			want := map[string]string{"min": "2\n", "max": "7\n"}[op]
			for name, args := range map[string][]string{
				"interpreter":       {"-run", src},
				"bytecode_governed": {"-run-bc", art},
				"hfir_bytecode":     {"-run-bc", hart},
			} {
				var stdout, stderr bytes.Buffer
				cmd := exec.Command(bin, args...)
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				err := cmd.Run()
				if err == nil {
					t.Fatalf("%s: empty %s succeeded: %q", name, op, stdout.String())
				}
				if stdout.String() != want {
					t.Fatalf("%s: stdout %q want %q", name, stdout.String(), want)
				}
				if !strings.Contains(stderr.String(), "VALUE_ERROR: "+op+" of empty list") {
					t.Fatalf("%s: stderr %q", name, stderr.String())
				}
			}
		})
	}
}

// Lain's guard: Go/JS builds reject the #138 core-library built-ins and nz
// (both lower to let-expressions those backends cannot emit) with a clear
// error, nonzero exit, no "Built" and no output file. A user variable that
// shares a built-in name is unaffected.
func TestGoJSTargetsRejectCoreLibraryAndNz(t *testing.T) {
	bin := c1c2Binary(t)
	programs := map[string]string{
		"sum":        `(cli_app (let (xs (list 3 1)) (print (sum xs))))`,
		"sort":       `(cli_app (let (xs (list "b" "a")) (print (str_join (sort xs) ","))))`,
		"filter":     `(cli_app (let (xs (list 1 2)) (print (list_len (filter x xs (> x 1))))))`,
		"str_slice":  `(cli_app (print (str_slice "hello" 1 3)))`,
		"count_by":   `(cli_app (let (xs (list "a")) (print (encode_json (count_by x xs x)))))`,
		"nz":         `(cli_app (let (m (dict ("a" "x"))) (print (nz (map_get m "b") "d"))))`,
		"list_slice": `(cli_app (let (xs (list 1 2 3)) (print (list_len (list_slice xs 1)))))`,
	}
	for _, target := range []string{"go", "js"} {
		for name, program := range programs {
			t.Run(target+"/"+name, func(t *testing.T) {
				dir := t.TempDir()
				src := filepath.Join(dir, "p.howl")
				outDir := filepath.Join(dir, "out")
				os.WriteFile(src, []byte(program), 0600)
				out, err := exec.Command(bin, "build", "--target="+target, src, "-o", outDir).CombinedOutput()
				if err == nil {
					t.Fatalf("build succeeded: %s", out)
				}
				if strings.Contains(string(out), "Built") || !strings.Contains(string(out), "UNSUPPORTED_ON_TARGET") {
					t.Fatalf("unexpected output: %s", out)
				}
				if entries, _ := os.ReadDir(outDir); len(entries) != 0 {
					t.Fatalf("files written after rejection: %v", entries)
				}
			})
		}
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "v.howl")
	os.WriteFile(src, []byte(`(cli_app (let (sum 2) (print sum)))`), 0600)
	if out, err := exec.Command(bin, "build", "--target=go", src, "-o", filepath.Join(dir, "out")).CombinedOutput(); err != nil {
		t.Fatalf("variable named sum rejected: %v %s", err, out)
	}
}
