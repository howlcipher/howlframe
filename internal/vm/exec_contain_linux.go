//go:build linux

package vm

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Process containment for governed exec (audit finding HF-02, issue #181).
//
// Killing only the direct child leaves its descendants running, and a
// descendant that calls setsid also escapes a process-group kill. Linux gives
// an unprivileged way to keep exact lineage: a "subreaper". Each exec call runs
// the real program under a tiny supervisor, a copy of this binary started with
// supervisorFlag. The supervisor marks itself a child subreaper, so every
// process orphaned anywhere beneath it is re-parented to the supervisor instead
// of to init. When the program exits, or the runner closes the control pipe
// (deadline, output limit, or the runner dying), the supervisor kills every
// process that is still its descendant, repeating until none remain.
//
// Guarantee: when an exec call returns, no process it started is still running,
// including background children and children that moved to a new session.
// Not covered: a process that cannot be signalled by the runner's user.

const supervisorFlag = "--howlframe-exec-supervisor-v1"

// prSetChildSubreaper is PR_SET_CHILD_SUBREAPER from <linux/prctl.h>.
const prSetChildSubreaper = 36

// supervisorStartFailure is the exit code the supervisor uses when it could not
// start the program; the reason is written to the status pipe.
const supervisorStartFailure = 126

func init() {
	if len(os.Args) >= 4 && os.Args[1] == supervisorFlag && os.Args[2] == "--" {
		os.Exit(runSupervisor(os.Args[3], os.Args[4:]))
	}
}

// runContained runs program with args, capturing stdout and stderr into out.
// It returns what (*exec.Cmd).Run would: nil, an *exec.ExitError for a non-zero
// or signalled exit, or another error when the program could not be started.
//
// Ordinary behavior is unchanged: the call returns once the program has exited
// and every process still holding its output has closed it. Only then, or at
// once on cancellation or output-limit failure, is everything left in the tree
// killed. The end of the output is never taken for the end of the program: a
// program may close its own stdout and keep running, so the supervisor reports
// the program's exit explicitly over the status pipe.
func runContained(ctx context.Context, program string, args []string, env []string, out io.Writer) error {
	self, err := os.Executable()
	if err != nil {
		return errors.New("exec containment unavailable: cannot locate the runner executable")
	}
	ctrlR, ctrlW, err := os.Pipe()
	if err != nil {
		return err
	}
	statusR, statusW, err := os.Pipe()
	if err != nil {
		ctrlR.Close()
		ctrlW.Close()
		return err
	}
	defer statusR.Close()
	outR, outW, err := os.Pipe()
	if err != nil {
		ctrlR.Close()
		ctrlW.Close()
		statusW.Close()
		return err
	}
	defer outR.Close()

	argv := append([]string{supervisorFlag, "--", program}, args...)
	cmd := exec.Command(self, argv...)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = outW, outW
	cmd.ExtraFiles = []*os.File{ctrlR, statusW}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		ctrlR.Close()
		ctrlW.Close()
		statusW.Close()
		outW.Close()
		return err
	}
	ctrlR.Close()
	statusW.Close()
	outW.Close()

	copyDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(out, outR)
		copyDone <- err
	}()
	// The first line on the status pipe says how the program ended; end-of-file
	// without one means the supervisor is gone.
	statusLine := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(statusR).ReadString('\n')
		statusLine <- strings.TrimSpace(line)
	}()
	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	// Finish normally only when the program has exited AND its output has ended;
	// anything else (cancellation, failure to store output) stops it at once.
	var report string
	copyEnded, exited, stopNow := false, false, false
	for !(copyEnded && exited) && !stopNow {
		select {
		case err := <-copyDone:
			copyEnded = true
			copyDone = nil
			stopNow = err != nil // the output limit refused more data
		case report = <-statusLine:
			exited = true
			statusLine = nil
		case <-ctx.Done():
			stopNow = true
		}
	}
	ctrlW.Close() // asks the supervisor to sweep the tree
	if stopNow {
		outR.Close() // release any writer blocked on a full pipe
	}

	var runErr error
	select {
	case runErr = <-waitDone:
	case <-time.After(5 * time.Second):
		// The supervisor did not finish its sweep in time: remove its group.
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		runErr = <-waitDone
	}
	// Safety net for a supervisor that was itself killed.
	syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	outR.Close()
	if copyDone != nil {
		<-copyDone
	}

	if reason, ok := strings.CutPrefix(report, startFailurePrefix); ok {
		return errors.New(reason)
	}
	return runErr
}

// startFailurePrefix begins the status line of a program that could not be started.
const startFailurePrefix = "start-failed "

// runSupervisor is the body of the supervisor process.
func runSupervisor(program string, args []string) int {
	syscall.RawSyscall(syscall.SYS_PRCTL, prSetChildSubreaper, 1, 0)
	// Keep the control and status descriptors away from the program itself.
	syscall.CloseOnExec(3)
	syscall.CloseOnExec(4)
	ctrl := os.NewFile(3, "control")
	status := os.NewFile(4, "status")

	child := exec.Command(program, args...)
	child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
	child.Env = os.Environ()
	if err := child.Start(); err != nil {
		status.WriteString(startFailurePrefix + strings.ReplaceAll(err.Error(), "\n", " ") + "\n")
		status.Close()
		return supervisorStartFailure
	}
	// Drop the supervisor's own copies of the output pipe so that its end-of-file
	// is decided only by the program and its descendants.
	os.Stdout.Close()
	os.Stderr.Close()

	ctrlClosed := make(chan struct{})
	go func() {
		io.Copy(io.Discard, ctrl)
		close(ctrlClosed)
	}()
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()

	var waitErr error
	select {
	case waitErr = <-waited:
	case <-ctrlClosed:
		// The runner stopped us before the program finished (deadline, output
		// limit, or the runner dying). Unless it has just exited, stop everything.
		select {
		case waitErr = <-waited:
		default:
			killDescendants()
			return 137
		}
	}
	// Tell the runner the program has exited, then wait for its go-ahead (it also
	// needs the program's output to have ended) before removing what is left.
	status.WriteString("exited\n")
	status.Close()
	<-ctrlClosed
	killDescendants()

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			// Report death by the same signal, as an unsupervised child would.
			signal.Reset(ws.Signal())
			syscall.Kill(os.Getpid(), ws.Signal())
			time.Sleep(100 * time.Millisecond)
			return 128 + int(ws.Signal())
		}
		return exitErr.ExitCode()
	}
	if waitErr != nil {
		return 1
	}
	return 0
}

// killDescendants kills every process still descended from this supervisor and
// reaps them, repeating because killing a parent re-parents its children to the
// supervisor (it is a subreaper), where the next pass finds them.
func killDescendants() {
	self := os.Getpid()
	for pass := 0; pass < 200; pass++ {
		kids := childrenOf(self)
		if len(kids) == 0 {
			reapAll()
			return
		}
		for _, pid := range kids {
			syscall.Kill(pid, syscall.SIGKILL)
		}
		reapAll()
		time.Sleep(2 * time.Millisecond)
	}
}

func reapAll() {
	for {
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
		if pid <= 0 || err != nil {
			return
		}
	}
}

// childrenOf lists processes whose parent is ppid, from /proc.
func childrenOf(ppid int) []int {
	var kids []int
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		s := string(raw)
		// "pid (comm) state ppid ...": comm may contain spaces and parentheses.
		i := strings.LastIndexByte(s, ')')
		if i < 0 {
			continue
		}
		fields := strings.Fields(s[i+1:])
		if len(fields) < 2 {
			continue
		}
		if parent, err := strconv.Atoi(fields[1]); err == nil && parent == ppid {
			kids = append(kids, pid)
		}
	}
	return kids
}
