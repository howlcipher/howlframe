package vm

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// processBudget counts the exec children alive in one run (improvements.md #115).
type processBudget struct {
	mu   sync.Mutex
	live int
}

// processSlots returns the run's shared counter, creating it for a VM built
// without one (library callers and tests); such a VM bounds only itself.
func (l *VMLimits) processSlots() *processBudget {
	if l.processes == nil {
		l.processes = &processBudget{}
	}
	return l.processes
}

func (l *VMLimits) maxProcesses() int {
	if l.MaxProcesses <= 0 {
		return DefaultMaxProcesses
	}
	return l.MaxProcesses
}

// acquire reserves a child slot, reporting false when the ceiling is reached.
func (b *processBudget) acquire(max int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.live >= max {
		return false
	}
	b.live++
	return true
}

func (b *processBudget) release() {
	b.mu.Lock()
	b.live--
	b.mu.Unlock()
}

// captureLimit is the byte ceiling shared by a child's output streams. Going
// over it cancels the child.
type captureLimit struct {
	mu       sync.Mutex
	used     int
	max      int
	exceeded bool
	cancel   context.CancelFunc
}

// limitedOutput captures one stream. Returning an error stops copying;
// cancellation also stops a producing process.
type limitedOutput struct {
	limit  *captureLimit
	buffer bytes.Buffer
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	b.limit.mu.Lock()
	defer b.limit.mu.Unlock()
	if len(p) > b.limit.max-b.limit.used {
		b.limit.exceeded = true
		b.limit.cancel()
		return 0, io.ErrShortWrite
	}
	b.limit.used += len(p)
	return b.buffer.Write(p)
}

// newCapture returns stdout and stderr captures sharing one limit. With split
// false both are the same writer, so the child's streams stay interleaved in
// one buffer exactly as written (the long-standing combined form).
func newCapture(max int, cancel context.CancelFunc, split bool) (*limitedOutput, *limitedOutput) {
	limit := &captureLimit{max: max, cancel: cancel}
	stdout := &limitedOutput{limit: limit}
	if !split {
		return stdout, stdout
	}
	return stdout, &limitedOutput{limit: limit}
}

// execResult is one finished exec child.
type execResult struct {
	stdout, stderr []byte
	exitCode       int // >0 for a non-zero exit; 0 for success, a signal or a start failure
	err            error
}

// runExec runs an authorized child under the run's process ceiling, per-call
// time bound, deadline and output limit. Limit and deadline failures panic
// with typed errors; a child that ran and failed is returned in err.
func (vm *BCVM) runExec(program string, args []string, maxOutput int, split bool, ip int, op bytecode.Opcode) execResult {
	slots := vm.Limits.processSlots()
	if !slots.acquire(vm.Limits.maxProcesses()) {
		panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, op, "exec would exceed --max-processes of %d live children", vm.Limits.maxProcesses()))
	}
	defer slots.release()
	base, stopTimer := vm.ctx, context.CancelFunc(func() {})
	if vm.Limits.MaxExecMillis > 0 {
		base, stopTimer = context.WithTimeout(vm.ctx, time.Duration(vm.Limits.MaxExecMillis)*time.Millisecond)
	}
	defer stopTimer()
	cmdCtx, cancel := context.WithCancel(base)
	stdout, stderr := newCapture(maxOutput, cancel, split)
	// runContained also ends every descendant when the call ends (HF-02).
	err := runContained(cmdCtx, program, args, execChildEnv(capability.Grants(vm.AllowedCaps), program), stdout, stderr)
	timedOut := base.Err() == context.DeadlineExceeded && vm.ctx.Err() == nil
	cancel()
	vm.checkDeadline(ip, op)
	if timedOut {
		panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, op, "exec exceeded --max-exec-millis of %d ms", vm.Limits.MaxExecMillis))
	}
	if stdout.limit.exceeded {
		panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, op, "exec output exceeds maximum limit of %d bytes", maxOutput))
	}
	res := execResult{stdout: stdout.buffer.Bytes(), err: err}
	if split {
		res.stderr = stderr.buffer.Bytes()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
		res.exitCode = exitErr.ExitCode()
	}
	return res
}

// sameWriter reports whether two writers are the same value. Writers of a
// type that cannot be compared are treated as distinct.
func sameWriter(a, b io.Writer) (same bool) {
	defer func() {
		if recover() != nil {
			same = false
		}
	}()
	return a == b
}

// execChildEnv is an authorized child's environment. By default only names
// granted by process:env= pass. A process:exec_inherit_env= grant for this
// exact program, which only the runner can hold, passes the runner's own
// environment unchanged instead.
func execChildEnv(grants capability.Grants, program string) []string {
	if grants.InheritsEnv(program) {
		return append([]string{}, os.Environ()...)
	}
	return grants.ExecEnv(os.LookupEnv)
}
