package harness

import (
	"os"
	"os/exec"
	"path/filepath"
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

// Lain C: ~/.claude/.credentials.json must be unreadable inside the jail.
func TestJailMasksClaudeCredentials(t *testing.T) {
	bw := bwrapOK()
	if bw == "" {
		t.Skip("bwrap unavailable")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	cred := filepath.Join(home, ".claude", ".credentials.json")
	if e := os.MkdirAll(filepath.Dir(cred), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(cred, []byte("SECRET"), 0600); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(cred); e != nil || string(b) != "SECRET" {
		t.Fatal("setup")
	}
	dir := t.TempDir()
	out, _ := exec.Command(JailArgv(bw, dir, []string{"/bin/cat", cred})[0], JailArgv(bw, dir, []string{"/bin/cat", cred})[1:]...).CombinedOutput()
	if strings.Contains(string(out), "SECRET") {
		t.Fatalf("credentials readable in jail: %s", out)
	}
	if fi, e := os.Stat("/home"); e == nil && fi.IsDir() {
		if !strings.Contains(strings.Join(JailArgv(bw, dir, nil), " "), "--tmpfs /home") {
			t.Fatal("/home not masked")
		}
	}
}
