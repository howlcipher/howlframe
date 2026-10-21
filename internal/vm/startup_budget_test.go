package vm

import (
	"bytes"
	"testing"
)

func TestInstructionLimitStartupScoping(t *testing.T) {
	for _, tc := range []struct {
		name    string
		startup bool
		limits  VMLimits
		want    int
	}{
		{"request handler never gets the startup budget", false, VMLimits{MaxInstructions: 100, MaxStartupInstructions: 1000}, 100},
		{"server startup uses the larger budget", true, VMLimits{MaxInstructions: 100, MaxStartupInstructions: 1000}, 1000},
		{"unset startup budget changes nothing", true, VMLimits{MaxInstructions: 100}, 100},
		{"a smaller startup budget never lowers the limit", true, VMLimits{MaxInstructions: 100, MaxStartupInstructions: 10}, 100},
		{"an invalid base limit stays invalid", true, VMLimits{MaxInstructions: 0, MaxStartupInstructions: 1000}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := &BCVM{startup: tc.startup, Limits: tc.limits}
			if got := vm.instructionLimit(); got != tc.want {
				t.Fatalf("instructionLimit = %d, want %d", got, tc.want)
			}
		})
	}
}

// A program that does not define an HTTP server must not receive the larger
// startup budget, even when one is configured: the extension is for server
// startup only, not a global increase.
func TestStartupBudgetDoesNotApplyToNonServerPrograms(t *testing.T) {
	_, prog := parseAndCompile(t, `(cli_app (let (n 0) (do (while (< n 5000) (set n (+ n 1))) (print "done"))))`)
	if programServesHTTP(prog) {
		t.Fatal("a CLI program must not be classified as a server")
	}
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = 1000
	policy.Limits.MaxStartupInstructions = 10_000_000
	var out, errOut bytes.Buffer
	ev := RunBytecodeWithEvidence(prog, nil, policy, nil, nil, &out, &errOut, 0)
	if ev.RuntimeFailure == nil || ev.RuntimeFailure.Code != "LIMIT_EXCEEDED" {
		t.Fatalf("non-server program must keep the base instruction limit: %q %+v", out.String(), ev.RuntimeFailure)
	}
}

func TestServerProgramIsClassifiedAsServer(t *testing.T) {
	_, prog := parseAndCompile(t, `(http_server 8080 (route "/x" (lambda (req) (res 200 "text/plain" "ok"))))`)
	if !programServesHTTP(prog) {
		t.Fatal("http_server program not recognized")
	}
}
