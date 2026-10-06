package vm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type capsTransport func(*http.Request) (*http.Response, error)

func (f capsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestRequiredCapabilitiesExecution(t *testing.T) {
	previous := http.DefaultClient
	requests := 0
	http.DefaultClient = &http.Client{Transport: capsTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = previous })
	path := filepath.Join(t.TempDir(), "store.json")
	source := fmt.Sprintf(`(cli_app
 (store_open db "file://%s")
 (store_put db "x" (dict ("value" 1)))
 (print (store_get db "x"))
 (print (store_keys db))
 (store_delete db "x")
 (print (env "PATH"))
 (spawn_agent "agent" (task "task" (print (env "PATH"))))
 (try_let (x (fetch "http://fixture.invalid" "GET")) (catch err (print err)) (print x)))`, path)
	prog := bytecode.CompileToBytecode(parser.NewParser(lexer.NewLexer(source), "synthetic.howl").ParseExpression())
	report := bytecode.RequiredCapabilities(prog)
	var grant []capability.Capability
	for _, c := range report.Capabilities {
		grant = append(grant, capability.Capability(c))
	}
	run := func(caps []capability.Capability) bytecode.ExecutionEvidence {
		var out, errout bytes.Buffer
		return RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), caps, strings.NewReader(""), &out, &errout, 0)
	}
	if e := run(grant); e.RuntimeFailure != nil {
		t.Fatal(e.RuntimeFailure)
	}
	if requests != 1 {
		t.Fatalf("fetch did not execute: %d", requests)
	}
	for _, removed := range grant {
		t.Run(string(removed), func(t *testing.T) {
			var reduced []capability.Capability
			for _, c := range grant {
				if c != removed {
					reduced = append(reduced, c)
				}
			}
			// try_let/agent isolate failures, so also inspect their captured diagnostics.
			var out, errout bytes.Buffer
			e := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), reduced, strings.NewReader(""), &out, &errout, 0)
			denied := e.RuntimeFailure != nil && e.RuntimeFailure.Code == "CAPABILITY_DENIED"
			if !denied && !strings.Contains(out.String()+errout.String(), "CAPABILITY_DENIED") {
				t.Fatalf("removing %s did not deny: %+v %s %s", removed, e.RuntimeFailure, &out, &errout)
			}
		})
	}
}

func TestRequiredCapabilitiesLazyGeneratedEffects(t *testing.T) {
	previous := http.DefaultClient
	path := filepath.Join(t.TempDir(), "synthesized.txt")
	code := fmt.Sprintf(`(do (write_file %q "generated") (return 1))`, path)
	response, _ := json.Marshal(map[string]string{"response": code})
	http.DefaultClient = &http.Client{Transport: capsTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(response)), Header: make(http.Header)}, nil
	})}
	t.Cleanup(func() { http.DefaultClient = previous })
	source := `(cli_app (lazy_synthesize f () "test") (print (call f)))`
	compile := func() *bytecode.BCProgram {
		return bytecode.CompileToBytecode(parser.NewParser(lexer.NewLexer(source), "lazy.howl").ParseExpression())
	}
	prog := compile()
	var grant []capability.Capability
	for _, c := range bytecode.RequiredCapabilities(prog).Capabilities {
		grant = append(grant, capability.Capability(c))
	}
	var out, errout bytes.Buffer
	e := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), grant, strings.NewReader(""), &out, &errout, 0)
	if e.RuntimeFailure != nil {
		t.Fatal(e.RuntimeFailure)
	}
	if content, err := os.ReadFile(path); err != nil || string(content) != "generated" {
		t.Fatalf("synthesized body did not execute: %v %s", err, content)
	}
	var reduced []capability.Capability
	for _, c := range grant {
		if c != capability.Filesystem {
			reduced = append(reduced, c)
		}
	}
	e = RunBytecodeWithEvidence(compile(), nil, DefaultExecutionPolicy(), reduced, strings.NewReader(""), &out, &errout, 0)
	if e.RuntimeFailure == nil || e.RuntimeFailure.Code != "CAPABILITY_DENIED" {
		t.Fatalf("generated filesystem effect was not gated: %+v", e.RuntimeFailure)
	}
}
