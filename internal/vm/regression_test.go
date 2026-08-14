package vm

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/capability"
)

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestSpawnInheritsCapabilitiesAndResumesExecution(t *testing.T) {
	// This tests the bug fix where `OpSpawn` failed to pass AllowedCaps to child VM
	// and skipped the instruction after the spawn block due to an errant `continue`.
	src := `(cli_app
    (spawn (fn () (print "Spawned")))
    (print "Resumed")
)`
	_, prog := parseAndCompile(t, src)

	var out, errOut safeBuffer
	caps := []capability.Capability{capability.Process}

	go func() {
		RunBytecode(prog, nil, caps, strings.NewReader(""), &out, &errOut)
	}()
	time.Sleep(50 * time.Millisecond) // wait for spawn

	outStr := out.String()
	if !strings.Contains(outStr, "Spawned") {
		t.Errorf("Child VM did not execute correctly or lacked capabilities, output: %s", outStr)
	}
	if !strings.Contains(outStr, "Resumed") {
		t.Errorf("Parent VM did not resume correctly (ip misalignment), output: %s", outStr)
	}
}

func TestHttpRouteInheritsCapabilitiesAndResumesExecution(t *testing.T) {
	// This tests the bug fix where `OpHttpRoute` failed to pass AllowedCaps to child VM
	// and skipped the instruction after the route block due to an errant `continue`.
	src := `(http_server 0
    (route "/" (lambda (req) (print "Routed")))
)`
	_, prog := parseAndCompile(t, src)

	var out, errOut safeBuffer
	caps := []capability.Capability{capability.Network}
	env := NewBcEnv(nil)
	machine := &BCVM{prog: prog, env: env, insts: prog.Main, stores: newBCStoreRegistry(), Limits: DefaultLimits, AllowedCaps: caps, Out: &out, ErrOut: &errOut}
	route := prog.Main[1]
	machine.run(prog.Main[:2+int(route.IntOperand)], env)
	mux := env.vars["__http_mux"].(*http.ServeMux)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest("GET", "/", nil))

	outStr := out.String()
	if !strings.Contains(outStr, "Routed") {
		t.Errorf("route handler did not execute, output: %s", outStr)
	}
}
