//go:build !unix

package vm

import (
	"context"
	"io"
	"os/exec"
	"time"
)

// Platforms without process groups or subreapers (for example Windows) kill
// only the direct child when an exec call is cancelled. Descendants are NOT
// contained there; this is a documented limit
// (docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md).
func runContained(ctx context.Context, program string, args []string, env []string, out io.Writer) error {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = out, out
	cmd.WaitDelay = 100 * time.Millisecond
	return cmd.Run()
}
