//go:build linux

package vm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/capability"
)

// Regression for audit finding HF-02 (issue #181): when an exec call ends, every
// process it started must be gone, including background children and children
// that moved to a new session. Each test uses unique sleep durations as markers,
// finds survivors by scanning /proc, and kills any survivor in a cleanup so a
// failing run leaves nothing behind. Nothing touches paths outside t.TempDir().

func processesWithMarker(marker string) []int {
	var pids []int
	entries, _ := os.ReadDir("/proc")
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		if strings.Contains(string(raw), marker) && !strings.Contains(string(raw), "exec_containment") {
			pids = append(pids, pid)
		}
	}
	return pids
}

func killMarked(marker string) {
	for _, pid := range processesWithMarker(marker) {
		syscall.Kill(pid, syscall.SIGKILL)
	}
}

// waitGone polls briefly: a killed process can take a moment to leave /proc.
func waitGone(marker string, within time.Duration) []int {
	deadline := time.Now().Add(within)
	for {
		left := processesWithMarker(marker)
		if len(left) == 0 || time.Now().After(deadline) {
			return left
		}
		time.Sleep(25 * time.Millisecond)
	}
}

type containmentTools struct{ sh, sleep, setsid string }

func lookContainmentTools(t *testing.T) containmentTools {
	t.Helper()
	var tools containmentTools
	for name, dst := range map[string]*string{"sh": &tools.sh, "sleep": &tools.sleep, "setsid": &tools.setsid} {
		p, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("%s not available", name)
		}
		*dst = p
	}
	sh, err := filepath.EvalSymlinks(tools.sh)
	if err != nil {
		t.Skip("cannot resolve sh")
	}
	tools.sh = sh
	return tools
}

// runExecScript runs script through the governed exec and returns after the
// program ends. grants allow only sh -c <one script argument>.
func runExecScript(t *testing.T, tools containmentTools, script string, policy ExecutionPolicy) (stdout string, wall time.Duration) {
	t.Helper()
	caps := []capability.Capability{capability.Capability("process:exec=" + tools.sh + " -c *")}
	src := fmt.Sprintf(`(cli_app (try_let (r (exec %q "-c" %q)) (catch e (print "caught")) (print "done")))`, tools.sh, script)
	_, prog := parseAndCompile(t, src)
	var out, errOut bytes.Buffer
	start := time.Now()
	RunBytecodeWithEvidence(prog, nil, policy, caps, nil, &out, &errOut, 0)
	return out.String(), time.Since(start)
}

func uniqueMarker(t *testing.T, salt int) string {
	m := fmt.Sprintf("%d.%d", 7000+os.Getpid()%900, time.Now().UnixNano()%1_000_000+int64(salt))
	t.Cleanup(func() { killMarked(m) })
	return m
}

func TestExecKillsBackgroundAndSessionEscapedDescendantsOnNormalExit(t *testing.T) {
	tools := lookContainmentTools(t)
	bg, escaped := uniqueMarker(t, 1), uniqueMarker(t, 2)
	script := fmt.Sprintf("%s %s >/dev/null 2>&1 & %s %s %s >/dev/null 2>&1 & exit 0", tools.sleep, bg, tools.setsid, tools.sleep, escaped)
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = 10_000_000
	out, _ := runExecScript(t, tools, script, policy)
	if !strings.Contains(out, "done") {
		t.Fatalf("exec should complete normally, got %q", out)
	}
	for name, marker := range map[string]string{"background": bg, "session-escaped": escaped} {
		if left := waitGone(marker, 3*time.Second); len(left) != 0 {
			t.Errorf("%s descendant survived the end of exec: pids %v", name, left)
		}
	}
}

func TestExecDeadlineKillsEveryDescendant(t *testing.T) {
	tools := lookContainmentTools(t)
	fg, bg, escaped := uniqueMarker(t, 3), uniqueMarker(t, 4), uniqueMarker(t, 5)
	script := fmt.Sprintf("%s %s >/dev/null 2>&1 & %s %s %s >/dev/null 2>&1 & %s %s", tools.sleep, bg, tools.setsid, tools.sleep, escaped, tools.sleep, fg)
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = 10_000_000
	policy.Deadline = 1500 * time.Millisecond
	_, wall := runExecScript(t, tools, script, policy)
	if wall > 10*time.Second {
		t.Fatalf("deadline not enforced promptly: %v", wall)
	}
	for name, marker := range map[string]string{"foreground": fg, "background": bg, "session-escaped": escaped} {
		if left := waitGone(marker, 3*time.Second); len(left) != 0 {
			t.Errorf("%s descendant survived the deadline: pids %v", name, left)
		}
	}
}

func TestExecOutputLimitKillsEveryDescendant(t *testing.T) {
	tools := lookContainmentTools(t)
	bg, escaped := uniqueMarker(t, 6), uniqueMarker(t, 7)
	// The sleepers keep the output pipe open (no redirect), then the script floods it.
	script := fmt.Sprintf("%s %s & %s %s %s & yes | head -c 200000", tools.sleep, bg, tools.setsid, tools.sleep, escaped)
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = 10_000_000
	policy.Limits.MaxExecOutputBytes = 1000
	policy.Deadline = 20 * time.Second
	_, wall := runExecScript(t, tools, script, policy)
	if wall > 15*time.Second {
		t.Fatalf("output limit did not stop the exec promptly: %v", wall)
	}
	for name, marker := range map[string]string{"background": bg, "session-escaped": escaped} {
		if left := waitGone(marker, 3*time.Second); len(left) != 0 {
			t.Errorf("%s descendant survived the output-limit stop: pids %v", name, left)
		}
	}
}
