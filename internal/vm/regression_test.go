package vm

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/capability"
)

func TestSpawnInheritsCapabilitiesAndResumesExecution(t *testing.T) {
	// This tests the bug fix where `OpSpawn` failed to pass AllowedCaps to child VM
	// and skipped the instruction after the spawn block due to an errant `continue`.
	src := `(cli_app
    (spawn (fn () (print "Spawned")))
    (print "Resumed")
)`
	_, prog := parseAndCompile(t, src)

	var out, errOut bytes.Buffer
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
	src := `(http_server 8090
    (route "/" (lambda (req) (print "Routed")))
)`
	_, prog := parseAndCompile(t, src)

	var out, errOut bytes.Buffer
	caps := []capability.Capability{capability.Network}

	go func() {
		RunBytecode(prog, nil, caps, strings.NewReader(""), &out, &errOut)
	}()
    time.Sleep(100 * time.Millisecond)

	outStr := out.String()
	if !strings.Contains(outStr, "Listening on 8090") {
		t.Errorf("Parent VM did not execute http_server correctly, output: %s", outStr)
	}
}
