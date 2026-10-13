package hfir

import (
	"flag"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/vm"
)

func adapterProgram(t *testing.T, source string) *bytecode.BCProgram {
	t.Helper()
	data, err := EncodeCandidateTransport(parseAndLower(t, source, "adapter_deadline.howl", false))
	if err != nil {
		t.Fatal(err)
	}
	c, ds := DecodeCandidate(data)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	p, ds := CompileCandidate(c)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	return p
}

// #175: the runner owns the adapter budget; it can raise it up to 30s.
func TestAdapterDeadlineRunnerOwned(t *testing.T) {
	for _, tc := range []struct {
		name              string
		adapter, deadline time.Duration
		source            string
		wantMS            int64
		wantCode, wantOut string
	}{
		{"default_is_1s", 0, 0, `(cli_app (print 1))`, 1000, "", "1\n"},
		{"raised_budget_allows_sleep", 1500 * time.Millisecond, 0, `(cli_app (sleep 1200) (print 7))`, 1500, "", "7\n"},
		{"over_cap_clamped", time.Hour, 0, `(cli_app (print 1))`, 30000, "", "1\n"},
		{"over_cap_sleep_refused", time.Hour, 0, `(cli_app (sleep 31000) (print 7))`, 30000, "HFIR_DEADLINE_EXCEEDED", ""},
		{"shorter_runner_deadline_wins", 10 * time.Second, 50 * time.Millisecond, `(cli_app (print 1))`, 50, "", "1\n"},
		{"longer_runner_deadline_capped_by_budget", 0, 5 * time.Second, `(cli_app (print 1))`, 1000, "", "1\n"},
		{"runner_deadline_over_hard_cap", time.Hour, time.Hour, `(cli_app (print 1))`, 30000, "", "1\n"},
		{"default_still_refuses_long_sleep", 0, 0, `(cli_app (sleep 1200) (print 7))`, 1000, "HFIR_DEADLINE_EXCEEDED", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := vm.DefaultExecutionPolicy()
			policy.AdapterDeadline = tc.adapter
			policy.Deadline = tc.deadline
			var out strings.Builder
			start := time.Now()
			_, receipt := vm.RunBytecodeWithReceipt(adapterProgram(t, tc.source), nil, policy, nil, strings.NewReader(""), &out, io.Discard, "test", "test")
			if receipt.Limits.DeadlineMS != tc.wantMS {
				t.Fatalf("effective deadline %dms, want %dms", receipt.Limits.DeadlineMS, tc.wantMS)
			}
			code := ""
			if receipt.Exit.Error != nil {
				code = receipt.Exit.Error.Code
			}
			if code != tc.wantCode || out.String() != tc.wantOut {
				t.Fatalf("code=%q out=%q", code, out.String())
			}
			if tc.wantCode != "" && time.Since(start) > 500*time.Millisecond {
				t.Fatal("over-budget sleep was not refused immediately")
			}
		})
	}
}

// AdapterDeadline never constrains or extends ordinary artifact runs.
func TestAdapterDeadlineIgnoredForArtifacts(t *testing.T) {
	p, ds := LowerToBytecode(parseAndLower(t, `(cli_app (print 1))`, "artifact.howl", false))
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	policy := vm.DefaultExecutionPolicy()
	policy.AdapterDeadline = 2 * time.Second
	_, receipt := vm.RunBytecodeWithReceipt(p, nil, policy, nil, strings.NewReader(""), io.Discard, io.Discard, "test", "test")
	if receipt.Limits.DeadlineMS != 0 {
		t.Fatalf("artifact run got adapter deadline %d", receipt.Limits.DeadlineMS)
	}
}

func TestAdapterDeadlineFlag(t *testing.T) {
	for raw, ok := range map[string]bool{"30s": true, "250ms": true, "30.001s": false, "31s": false, "1h": false, "0s": false, "-1s": false, "soon": false} {
		policy := vm.DefaultExecutionPolicy()
		fs := flag.NewFlagSet("runner", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		fs.Var(vm.AdapterDeadlineFlag{Policy: &policy}, "adapter-deadline", "")
		err := fs.Parse([]string{"-adapter-deadline", raw})
		if ok != (err == nil) {
			t.Errorf("%s: err=%v", raw, err)
		}
		if ok && policy.AdapterDeadline.String() != mustDuration(t, raw).String() {
			t.Errorf("%s: stored %s", raw, policy.AdapterDeadline)
		}
		if !ok && policy.AdapterDeadline != 0 {
			t.Errorf("%s: rejected value stored %s", raw, policy.AdapterDeadline)
		}
	}
}

func mustDuration(t *testing.T, s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
