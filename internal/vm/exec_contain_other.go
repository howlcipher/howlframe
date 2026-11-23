//go:build !unix

package vm

import (
	"context"
	"io"
	"os"
	"os/exec"
	"time"
)

// Platforms without process groups or subreapers (for example Windows) kill
// only the direct child when an exec call is cancelled. Descendants are NOT
// contained there; this is a documented limit
// (docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md).
func runContained(ctx context.Context, program string, args []string, env []string, stdout, stderr io.Writer, signals <-chan os.Signal) error {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = stdout, stderr
	cmd.WaitDelay = 100 * time.Millisecond
	// Without process groups a forwarded signal cannot reach descendants;
	// any signal ends the direct child.
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-signals:
			cmd.Process.Kill()
		case <-done:
		}
	}()
	err := cmd.Wait()
	close(done)
	return err
}
