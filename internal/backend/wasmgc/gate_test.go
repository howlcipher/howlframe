package wasmgc

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
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
	engines := map[string]func(*testing.T, string) outcome{"node": onWasmAt}
	if _, err := exec.LookPath("wasmtime"); err == nil {
		engines["wasmtime"] = onWasmtimeAt
	}
	for engine, runOn := range engines {
		got := runOn(t, driver)
		if got.failed {
			t.Fatalf("WebAssembly (%s): %s", engine, got.errMsg)
		}
		if !bytes.Equal([]byte(got.stdout), []byte(want.stdout)) {
			t.Fatalf("output differs (vm %d bytes, %s %d bytes); first difference at byte %d",
				len(want.stdout), engine, len(got.stdout), firstDifference(want.stdout, got.stdout))
		}
		t.Logf("%s: %d fixture projects, %d bytes of identical output", engine, len(fixtures), len(want.stdout))
	}
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

// TestHowlSceneBackupGate runs HowlScene HF's backup module (which reads
// base64 with base64_text and sniff_media_type, browser host operations
// with no VM form) as WebAssembly and on the JavaScript backend, its
// reference: backup files for the portfolio fixture, then reading a good
// backup and broken ones back, byte for byte. Like TestHowlSceneGate it
// needs HOWLSCENE_HF.
func TestHowlSceneBackupGate(t *testing.T) {
	hs := os.Getenv("HOWLSCENE_HF")
	if hs == "" {
		t.Skip("set HOWLSCENE_HF to a HowlScene HF checkout to run the backup gate")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	project, err := os.ReadFile(filepath.Join(hs, "tests", "fixtures", "neutral", "portfolio.json"))
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile(filepath.Join(hs, "tests", "fixtures", "neutral", "img-sample01.png"))
	if err != nil {
		t.Fatal(err)
	}
	b64 := func(b []byte) string { return howlString(base64.StdEncoding.EncodeToString(b)) }
	dir := t.TempDir()
	core, err := filepath.Rel(dir, filepath.Join(hs, "src", "core"))
	if err != nil {
		t.Fatal(err)
	}
	entry := func(path, data string) string { return `(dict ("path" "` + path + `") ("base64" ` + data + `))` }
	good := `(list ` + entry("howlscene-project.json", b64(project)) + ` ` + entry("assets/img-sample01.png", b64(image)) + `)`
	src := `(cli_app
  (use "` + core + `/registry.howl" as registry)
  (use "` + core + `/schema.howl" as schema)
  (use "` + core + `/backup.howl" as backup)
  (let (reg (call registry/registry) paths (dict ("img-sample01" "assets/img-sample01.png")))
    (let (r (call schema/parse_project ` + howlString(string(project)) + ` reg))
      (let (config (map_get r "config"))
        (do
          (print (encode_json (call backup/backup_files config (dict ("img-sample01" ` + b64(image) + `)) paths)))
          (print (encode_json (call backup/backup_project_text ` + good + `)))
          (print (encode_json (call backup/backup_asset_data config ` + good + ` paths)))
          (print (encode_json (call backup/backup_project_text (list ` + entry("howlscene-project.json", `"/w=="`) + `))))
          (print (encode_json (call backup/backup_project_text (list ` + entry("other.json", `"e30="`) + `))))
          (print (encode_json (call backup/backup_asset_data config (list ` + entry("assets/img-sample01.png", `"R0lGODlhAQABAAAAACw="`) + `) paths)))
          (print (encode_json (call backup/backup_asset_data config (list) paths))))))))
`
	driver := filepath.Join(dir, "backup.howl")
	if err := os.WriteFile(driver, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	want := onJSAt(t, driver)
	if want.failed {
		t.Fatalf("JavaScript: %s", want.errMsg)
	}
	for i, line := range strings.Split(strings.TrimSpace(want.stdout), "\n") {
		t.Logf("line %d: %.110s", i+1, line)
	}
	engines := map[string]func(*testing.T, string) outcome{"node": onWasmAt}
	if _, err := exec.LookPath("wasmtime"); err == nil {
		engines["wasmtime"] = onWasmtimeAt
	}
	for engine, runOn := range engines {
		got := runOn(t, driver)
		if got.failed || got.stdout != want.stdout {
			t.Fatalf("%s differs from the JavaScript backend (failed=%v %s); first difference at byte %d\njs   %.300q\nwasm %.300q",
				engine, got.failed, got.errMsg, firstDifference(want.stdout, got.stdout), want.stdout, got.stdout)
		}
		t.Logf("%s: %d bytes identical to the JavaScript backend's", engine, len(want.stdout))
	}
}
