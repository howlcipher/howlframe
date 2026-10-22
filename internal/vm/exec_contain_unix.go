//go:build unix && !linux

package vm

import (
	"context"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// On Unix systems without a subreaper facility the exec child runs in its own
// process group and the whole group is killed when the call ends, so ordinary
// background children are contained. A descendant that starts a new session
// (setsid) leaves the group and is NOT contained on these platforms; this is a
// documented limit (docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md).
func runContained(ctx context.Context, program string, args []string, env []string, out io.Writer) error {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = out, out
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return nil
	}
	cmd.WaitDelay = 100 * time.Millisecond
	err := cmd.Run()
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return err
}
