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

// Records and JSON sugar (heldout-v3 records slice): rec, get, put, pluck,
// select, group_by, merge are pure; parse_json also runs under -run.
const recordsProgram = `(do
 (let (r (rec id 1 name "ann" "tags" (list "a" "b") addr (rec city "oslo")))
  (do
   (print (encode_json r))
   (print (get r "name") (get r "addr" "city") (get r "tags" 1) (is_nil (get r "nope")))
   (put r "addr" "zip" "0150")
   (put r "meta" "seen" "count" 3)
   (print (encode_json r))
   (print (encode_json (merge r (rec id 2 extra true) (rec name "bo"))) (get r "id"))))
 (let (rs (list (rec n "a" d "x" v 1) (rec n "b" d "y" v 2) (rec n "c" d "x" v 3)))
  (do
   (print (encode_json (pluck rs "n")))
   (print (encode_json (select rs n v)))
   (print (encode_json (group_by r rs (get r "d"))))
   (print (encode_json (group_by r rs (> (get r "v") 1))))))
 (let (s (str "{" "\"z\":1,\"a\":{\"y\":[1,2.5,null]}}"))
  (do (print (encode_json (parse_json Any s))) (print (get (parse_json Any s) "a" "y" 1)))))`

const recordsWant = `{"addr":{"city":"oslo"},"id":1,"name":"ann","tags":["a","b"]}
ann oslo b true
{"addr":{"city":"oslo","zip":"0150"},"id":1,"meta":{"seen":{"count":3}},"name":"ann","tags":["a","b"]}
{"addr":{"city":"oslo","zip":"0150"},"extra":true,"id":2,"meta":{"seen":{"count":3}},"name":"bo","tags":["a","b"]} 1
["a","b","c"]
[{"n":"a","v":1},{"n":"b","v":2},{"n":"c","v":3}]
{"x":[{"d":"x","n":"a","v":1},{"d":"x","n":"c","v":3}],"y":[{"d":"y","n":"b","v":2}]}
{"false":[{"d":"x","n":"a","v":1}],"true":[{"d":"y","n":"b","v":2},{"d":"x","n":"c","v":3}]}
{"a":{"y":[1,2.5,null]},"z":1}
2.5
`

func recordsPaths(t *testing.T, bin, src, dir string) map[string][]string {
	t.Helper()
	art := filepath.Join(dir, "p.hfbc")
	hart := filepath.Join(dir, "h.hfbc")
	if out, err := exec.Command(bin, "build", "--profile", "governed", src, "-o", art).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	if out, err := exec.Command(bin, "-compile-hfir-bc", src, "-o", hart).CombinedOutput(); err != nil {
		t.Fatalf("hfir compile: %v %s", err, out)
	}
	return map[string][]string{
		"interpreter":       {"-run", src},
		"bytecode_governed": {"-run-bc", art},
		"hfir_bytecode":     {"-run-bc", hart},
	}
}

func TestRecordsSugarIsPureOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	src := writeTemp(t, dir, "p.howl", recordsProgram)
	for _, profile := range []string{"default", "governed"} {
		if out, err := exec.Command(bin, "check", "--profile", profile, src).CombinedOutput(); err != nil {
			t.Fatalf("check %s: %v %s", profile, err, out)
		}
	}
	runs := recordsPaths(t, bin, src, dir)
	out, err := exec.Command(bin, "inspect", "--caps", runs["bytecode_governed"][1]).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect: %v %s", err, out)
	}
	var report bytecode.Report
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(report.Capabilities) != 0 {
		t.Fatalf("pure records sugar required capabilities %v", report.Capabilities)
	}
	for name, args := range runs {
		t.Run(name, func(t *testing.T) {
			got, err := exec.Command(bin, args...).CombinedOutput()
			if err != nil {
				t.Fatalf("run: %v %s", err, got)
			}
			if string(got) != recordsWant {
				t.Fatalf("got %q want %q", got, recordsWant)
			}
		})
	}
}

// read_json/write_json need exactly filesystem (inferred from the
// read_file/write_file they expand to), run the same on every path with the
// grant, and fail closed without it or with only the read root.
func TestReadWriteJSONUnderFilesystemCapability(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	in, outDir := filepath.Join(dir, "in"), filepath.Join(dir, "out")
	os.MkdirAll(in, 0o755)
	os.MkdirAll(outDir, 0o755)
	os.WriteFile(filepath.Join(in, "c.json"), []byte(`{"z":1,"a":{"y":[1,2]},"m":"q"}`), 0o644)
	src := writeTemp(t, dir, "j.howl", `(do (let (c (read_json "in/c.json")) (do (put c "a" "k" 5) (write_json "out/o.json" c) (print (get c "m")))))`)
	runs := recordsPaths(t, bin, src, dir)
	out, err := exec.Command(bin, "inspect", "--caps", runs["bytecode_governed"][1]).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect: %v %s", err, out)
	}
	var report bytecode.Report
	json.Unmarshal(out, &report)
	if strings.Join(report.Capabilities, ",") != "filesystem" {
		t.Fatalf("capabilities %v, want [filesystem]", report.Capabilities)
	}
	grant := "filesystem:read=" + in + ",filesystem:write=" + outDir
	for name, args := range runs {
		t.Run(name, func(t *testing.T) {
			os.Remove(filepath.Join(outDir, "o.json"))
			full := append([]string{args[0], "-allow-caps", grant}, args[1:]...)
			c := exec.Command(bin, full...)
			c.Dir = dir
			got, err := c.CombinedOutput()
			if err != nil || string(got) != "q\n" {
				t.Fatalf("run: %v %q", err, got)
			}
			data, _ := os.ReadFile(filepath.Join(outDir, "o.json"))
			if string(data) != `{"a":{"k":5,"y":[1,2]},"m":"q","z":1}` {
				t.Fatalf("wrote %q", data)
			}
			for _, deny := range [][]string{{args[0]}, {args[0], "-allow-caps", "filesystem:read=" + in}} {
				os.Remove(filepath.Join(outDir, "o.json"))
				c := exec.Command(bin, append(deny, args[1:]...)...)
				c.Dir = dir
				got, err := c.CombinedOutput()
				if err == nil || !strings.Contains(string(got), "denied") {
					t.Errorf("%v: err=%v out=%q", deny, err, got)
				}
				if _, statErr := os.Stat(filepath.Join(outDir, "o.json")); statErr == nil {
					t.Errorf("%v wrote output without the write grant", deny)
				}
			}
		})
	}
}

func TestRecordsErrorsOnEveryPath(t *testing.T) {
	bin := c1c2Binary(t)
	for src, want := range map[string]string{
		`(do (let (j (parse_json Any (str "5"))) (print (get j "a"))))`: "map_get",
		`(do (let (s "{bad") (print (parse_json Any s))))`:              "parse_json failed",
		`(do (let (m (rec a 1)) (do (put m "a" "b" 2) (print m))))`:     "map_set",
	} {
		dir := t.TempDir()
		p := writeTemp(t, dir, "e.howl", src)
		for name, args := range recordsPaths(t, bin, p, dir) {
			out, err := exec.Command(bin, args...).CombinedOutput()
			if err == nil || !strings.Contains(string(out), want) {
				t.Errorf("%s %s: err=%v out=%q want %q", src, name, err, out, want)
			}
		}
	}
}

func TestRecordsCompileErrors(t *testing.T) {
	bin := c1c2Binary(t)
	for src, want := range map[string]string{
		`(do (print (rec a)))`:                    "rec expects key/value pairs",
		`(do (print (rec (+ 1 2) 3)))`:            "rec keys must be names or string literals",
		`(do (print (select (list) (+ 1 2))))`:    "select keys must be names or string literals",
		`(do (let (m (list 1)) (put m 0 2)))`:     "put: the last key must be a map key",
		`(do (print (group_by (list) (list) 1)))`: "group_by expects a variable name",
		`(do (print (get (rec a 1))))`:            "ARITY_ERROR: get expects at least 2 args, got 1",
		`(do (print (read_json)))`:                "ARITY_ERROR: read_json expects 1 arg",
		`(do (print (pluck (list) "a" "b")))`:     "ARITY_ERROR: pluck expects 2 args, got 3",
		`(do (print (merge)))`:                    "ARITY_ERROR: merge expects at least 1 arg",
	} {
		dir := t.TempDir()
		p := writeTemp(t, dir, "c.howl", src)
		for _, args := range [][]string{{"-run", p}, {"build", "--profile", "governed", p, "-o", filepath.Join(dir, "c.hfbc")}, {"-compile-hfir-bc", p, "-o", filepath.Join(dir, "h.hfbc")}} {
			out, err := exec.Command(bin, args...).CombinedOutput()
			if err == nil || !strings.Contains(string(out), want) {
				t.Errorf("%s %v: err=%v out=%q want %q", src, args[0], err, out, want)
			}
		}
	}
}

func TestRecordsRejectedOnGoJS(t *testing.T) {
	bin := c1c2Binary(t)
	forms := []string{
		`(print (rec a 1))`, `(let (m (dict)) (print (get m "a")))`, `(let (m (dict)) (put m "a" 1))`,
		`(print (read_json "x.json"))`, `(write_json "x.json" (list))`, `(print (pluck (list) "a"))`,
		`(print (select (list) a))`, `(print (group_by x (list) x))`, `(print (merge (dict)))`,
		`(let (rec (rec a 1)) (print rec))`,
		`(let (x 1 rec (rec a 1)) (print rec))`,
		`(try_let (rec (rec a 1)) (catch err (print err)) (print rec))`,
		`(defun f (rec) (return (get rec "a")))`,
		`(let (f (lambda (rec) (get rec "a"))) (print "ok"))`,
		`(print (dict ("a" (rec b 1))))`,
	}
	for _, f := range forms {
		for _, target := range []string{"go", "js"} {
			dir := t.TempDir()
			p := writeTemp(t, dir, "g.howl", "(do "+f+")")
			o := filepath.Join(dir, "out")
			out, err := exec.Command(bin, "build", "--target", target, p, "-o", o).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "UNSUPPORTED_ON_TARGET") || strings.Contains(string(out), "Built") {
				t.Errorf("%s on %s: err=%v out=%s", f, target, err, out)
			}
			if _, statErr := os.Stat(o); statErr == nil {
				t.Errorf("%s on %s wrote output", f, target)
			}
		}
	}
}

func TestRecordsGovernedFailsClosed(t *testing.T) {
	bin := c1c2Binary(t)
	for _, src := range []string{
		`(do (print (rec a (exec "ls"))))`,
		`(do (print (get (rec a 1) (exec "ls"))))`,
		`(do (let (m (dict)) (put m "a" (exec "ls"))))`,
		`(do (print (read_json (exec "ls"))))`,
		`(do (write_json "o.json" (exec "ls")))`,
		`(do (print (pluck (exec "ls") "a")))`,
		`(do (print (select (exec "ls") a)))`,
		`(do (print (group_by x (list 1) (exec "ls"))))`,
		`(do (print (merge (dict) (exec "ls"))))`,
	} {
		dir := t.TempDir()
		p := writeTemp(t, dir, "g.howl", src)
		out, err := exec.Command(bin, "build", "--profile", "governed", p, "-o", filepath.Join(dir, "g.hfbc")).CombinedOutput()
		if err == nil || !strings.Contains(string(out), "PROFILE_FORBIDDEN_CONSTRUCT") {
			t.Errorf("%s: err=%v out=%s", src, err, out)
		}
	}
}

func TestTargetSugarNamesInBindings(t *testing.T) {
	bin := c1c2Binary(t)
	for _, target := range []string{"go", "js"} {
		for _, body := range []string{
			`(let (rec (dict ("delta" 1))) (print (map_get rec "delta")))`,
			`(let (range 1 rec 2) (print range rec))`,
			`(defun echo (rec) (return rec)) (print (call echo "ok"))`,
			`(let (f (lambda (rec) (print rec))) (print "ok"))`,
		} {
			t.Run(target+body, func(t *testing.T) {
				dir := t.TempDir()
				p := writeTemp(t, dir, "binding.howl", "(do "+body+")")
				out, err := exec.Command(bin, "build", "--target", target, p, "-o", filepath.Join(dir, "out")).CombinedOutput()
				if err != nil {
					t.Fatalf("ordinary binding rejected: %v %s", err, out)
				}
			})
		}
	}
}
