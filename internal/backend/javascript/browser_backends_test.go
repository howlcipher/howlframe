package javascript_test

// HowlFrame's browser tests run every fake-DOM program on both backends that
// build a web_app for the browser: the JavaScript backend (app.js is the
// program) and the WebAssembly backend (app.js is the generated loader and
// app.wasm the program). Each harness runs the same JavaScript around them;
// the only difference is how a caught error is bound (the JavaScript
// backend binds an Error, so (to_string err) reads "Error: CODE: message",
// and the WebAssembly backend the VM's report), so the harnesses compare
// caught errors by code and message (howlNormJS, normalizeOutput).

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/backend/javascript"
	"github.com/howlcipher/howlframe/internal/backend/wasmgc"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

// backends are the targets every browser test runs on.
var backends = []string{"js", "wasmgc"}

// program is a web_app built for one backend.
type program struct {
	backend string
	code    string // the program (js) or its loader (wasmgc)
	wasm    []byte
}

// forBackends runs f once per backend, as subtests named for them.
func forBackends(t *testing.T, f func(t *testing.T, b string)) {
	t.Helper()
	for _, b := range backends {
		t.Run(b, func(t *testing.T) { f(t, b) })
	}
}

// build compiles a web_app for a backend.
func build(t *testing.T, backend, src string) program {
	t.Helper()
	root := parser.NewParser(lexer.NewLexer(src), "app.howl").ParseExpression()
	checker.Check(root)
	if backend == "js" {
		code, _ := javascript.GenerateJSCode(root)
		return program{backend: backend, code: code}
	}
	ast.Desugar(root)
	wasm, loader, err := wasmgc.CompileBrowser(root)
	if err != nil {
		t.Fatalf("wasmgc build: %v", err)
	}
	if tools, err := exec.LookPath("wasm-tools"); err == nil {
		path := filepath.Join(t.TempDir(), "app.wasm")
		if err := os.WriteFile(path, wasm, 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(tools, "validate", "--features", "all", path).CombinedOutput(); err != nil {
			t.Fatalf("wasm-tools rejects the module: %v\n%s", err, out)
		}
	}
	return program{backend: backend, code: loader, wasm: wasm}
}

// inContext returns a JavaScript expression that runs p in the node:vm
// context named ctx and gives its result: the context gets the page's
// standard text globals and, for the WebAssembly build, the module's bytes.
func (p program) inContext(ctx string) string {
	code, _ := json.Marshal(p.code)
	setup := ctx + ".TextEncoder = TextEncoder, " + ctx + ".TextDecoder = TextDecoder, "
	if p.wasm != nil {
		setup += ctx + `.howlFrameWasm = Buffer.from("` + base64.StdEncoding.EncodeToString(p.wasm) + `", "base64"), `
	}
	return "(" + setup + "vm.runInNewContext(" + string(code) + ", " + ctx + "))"
}

// script returns p as a script that runs in Node itself.
func (p program) script() string {
	if p.wasm == nil {
		return p.code
	}
	return `globalThis.howlFrameWasm = Buffer.from("` + base64.StdEncoding.EncodeToString(p.wasm) + `", "base64");` + "\n" + p.code
}

// howlNormJS defines howlNorm(text), which rewrites a caught error as either
// backend prints it to "CODE: message", and an engine error ("TypeError:
// ...") to "RUNTIME_ERROR: TypeError: ...".
const howlNormJS = `function howlNorm(s) {
  s = String(s);
  let m = /^Error: ([A-Z][A-Z0-9_]*: [\s\S]*)$/.exec(s);
  if (m) return m[1];
  if (s.startsWith('{"phase":"runtime"')) { try { const r = JSON.parse(s); return r.code + ": " + r.message; } catch (e) {} }
  if (/^[A-Za-z]*Error: /.test(s)) return "RUNTIME_ERROR: " + s;
  return s;
}
`

var (
	jsCaught  = regexp.MustCompile(`^Error: ([A-Z][A-Z0-9_]*: .*)$`)
	vmReport  = regexp.MustCompile(`^\{"phase":"runtime".*\}$`)
	engineErr = regexp.MustCompile(`^[A-Za-z]*Error: `)
)

// normalizeOutput applies howlNorm's rules to each line of a program's
// output.
func normalizeOutput(out string) string {
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		switch {
		case jsCaught.MatchString(line):
			lines[i] = jsCaught.FindStringSubmatch(line)[1]
		case vmReport.MatchString(line):
			var r struct{ Code, Message string }
			if json.Unmarshal([]byte(line), &r) == nil {
				lines[i] = r.Code + ": " + r.Message
			}
		case engineErr.MatchString(line):
			lines[i] = "RUNTIME_ERROR: " + line
		}
	}
	return strings.Join(lines, "\n")
}

// runNode runs a script in Node with only the given HowlFrame grants in its
// environment.
func runNode(t *testing.T, source string, envPairs ...string) (string, string, int) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node runtime not available")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "app.js")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", path)
	filtered := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "HOWLFRAME_ALLOW_CAPS=") || strings.HasPrefix(entry, "HOWLFRAME_ABI_SECRET=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	cmd.Env = append(filtered, envPairs...)
	out, err := cmd.Output()
	stderr := ""
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
			stderr = string(exitErr.Stderr)
		} else {
			t.Fatal(err)
		}
	}
	return string(out), stderr, code
}

// runWebAppOutput builds a web_app for a backend, runs it in Node and
// returns its output, caught errors normalized.
func runWebAppOutput(t *testing.T, backend, src string) string {
	t.Helper()
	stdout, stderr, exit := runNode(t, build(t, backend, src).script())
	if exit != 0 {
		t.Fatalf("node exited %d: %s %s", exit, stdout, stderr)
	}
	return normalizeOutput(strings.TrimSpace(stdout))
}
