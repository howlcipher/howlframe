package harness

import (
	"strings"
	"testing"
	"time"
)

// Lain B: a setsid grandchild holding stderr must not block the deadline.
func TestDeadlineFiresWithSetsidGrandchild(t *testing.T) {
	jails := []string{""}
	if bw := bwrapOK(); bw != "" {
		jails = append(jails, bw)
	}
	for _, jail := range jails {
		dir := t.TempDir()
		l := LiveRunner{Deadline: time.Second, Jail: jail, ChildArgv: func(_, _, _, _ string) []string {
			return []string{"/bin/sh", "-c", "setsid sleep 30 & sleep 30"}
		}}
		start := time.Now()
		_, killed, _, _, e := l.child("a", LiveTask{Task: Task{ID: "t01"}}, []byte("x"), dir)
		if e != nil || !killed {
			t.Fatalf("jail=%q: killed=%v err=%v", jail, killed, e)
		}
		if el := time.Since(start); el > time.Second+liveWaitDelay+2*time.Second {
			t.Fatalf("jail=%q: deadline took %s", jail, el)
		}
	}
}

func TestJailArgvHasPidNamespace(t *testing.T) {
	a := strings.Join(JailArgv("bwrap", "/w", []string{"x"}), " ")
	for _, f := range []string{"--unshare-pid", "--die-with-parent", "--unshare-net"} {
		if !strings.Contains(a, f) {
			t.Errorf("missing %s", f)
		}
	}
}

