//go:build linux

package vm

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// Review finding: a program that closes its own stdout and stderr and keeps
// running is not "finished". Output reaching end-of-file must never be taken
// for cancellation; the exec call has to wait for the program and report its
// real exit status, as it did before containment.
func TestExecProgramThatClosesItsOutputEarlyKeepsRunning(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	var out bytes.Buffer
	start := time.Now()
	runErr := runContained(context.Background(), sh, []string{"-c", "exec >/dev/null 2>&1; sleep 0.5; exit 7"}, []string{}, &out)
	took := time.Since(start)
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("want exit status 7, got %v after %v", runErr, took)
	}
	if took < 400*time.Millisecond {
		t.Fatalf("the program was stopped before it finished (%v)", took)
	}
}
