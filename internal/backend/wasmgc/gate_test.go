package wasmgc

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestHowlSceneGate is the M1 exit gate (docs/design/wasm_gc_backend.md):
// HowlScene HF's schema and renderer, run on its fixture projects, give
// byte-identical output on the VM and as WebAssembly. It needs a checkout
// of HowlScene HF, named by HOWLSCENE_HF, and is skipped without one.
func TestHowlSceneGate(t *testing.T) {
	hs := os.Getenv("HOWLSCENE_HF")
	if hs == "" {
		t.Skip("set HOWLSCENE_HF to a HowlScene HF checkout to run the M1 exit gate")
	}
	fixtures, err := filepath.Glob(filepath.Join(hs, "tests", "fixtures", "*", "*.json"))
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no fixture projects under %s: %v", hs, err)
	}
	sort.Strings(fixtures)
	dir := t.TempDir()
	core, err := filepath.Rel(dir, filepath.Join(hs, "src", "core"))
	if err != nil {
		t.Fatal(err)
	}
	// The driver embeds each fixture as text: a cli_app on WebAssembly has
	// no file system.
	var body strings.Builder
	for _, path := range fixtures {
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
		body.WriteString(`
  (let (text ` + howlString(string(text)) + `)
    (let (r (call schema/parse_project text reg))
      (do
        (print "=== ` + name + `" (map_get r "ok") (str_join (map_get r "errors") "; "))
        (print (call render/render_document (map_get r "config") reg (dict ("css" "") ("script" true) ("csp" true))))
        (print (call render/render_document (map_get r "config") reg (dict ("css" "body{margin:0}") ("script" false) ("csp" false) ("motion_off" true))))
        (print (encode_json (map_get r "config"))))))`)
	}
	driver := filepath.Join(dir, "gate.howl")
	src := `(cli_app
  (use "` + core + `/registry.howl" as registry)
  (use "` + core + `/schema.howl" as schema)
  (use "` + core + `/render.howl" as render)
  (let (reg (call registry/registry))
    (do` + body.String() + `)))
`
	if err := os.WriteFile(driver, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// The renderer runs far past the VM's default instruction limit.
	want := run(t, compilerBin, "run", "--max-instructions", "500000000", driver)
	if want.failed {
		t.Fatalf("VM: %s", want.errMsg)
	}
	got := onWasmAt(t, driver)
	if got.failed {
		t.Fatalf("WebAssembly: %s", got.errMsg)
	}
	if !bytes.Equal([]byte(got.stdout), []byte(want.stdout)) {
		t.Fatalf("output differs (vm %d bytes, wasm %d bytes); first difference at byte %d",
			len(want.stdout), len(got.stdout), firstDifference(want.stdout, got.stdout))
	}
	t.Logf("%d fixture projects, %d bytes of identical output", len(fixtures), len(want.stdout))
}

// howlString is s as a HowlFrame string literal.
func howlString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\t", `\t`, "\r", `\r`)
	return `"` + r.Replace(s) + `"`
}

func firstDifference(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}
