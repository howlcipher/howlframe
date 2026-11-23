//go:build unix

package vm

import (
	"context"
	"testing"
)

// Separate stdout and stderr captures share one byte limit (improvements.md
// #113); the combined form keeps one buffer.
func TestRunContainedSplitsStreams(t *testing.T) {
	sh := "/bin/sh"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout, stderr := newCapture(1024, cancel, true)
	if err := runContained(ctx, sh, []string{"-c", "echo out; echo err >&2; echo out2"}, []string{}, stdout, stderr, nil); err != nil {
		t.Fatal(err)
	}
	if got := string(stdout.buffer.Bytes()); got != "out\nout2\n" {
		t.Fatalf("stdout = %q", got)
	}
	if got := string(stderr.buffer.Bytes()); got != "err\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestSplitStreamsShareTheOutputLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout, stderr := newCapture(8, cancel, true)
	_ = runContained(ctx, "/bin/sh", []string{"-c", "printf 12345; printf 67890 >&2; sleep 1"}, []string{}, stdout, stderr, nil)
	if !stdout.limit.exceeded {
		t.Fatalf("10 bytes across both streams passed an 8 byte limit: %q %q", stdout.buffer.Bytes(), stderr.buffer.Bytes())
	}
	if total := stdout.buffer.Len() + stderr.buffer.Len(); total > 8 {
		t.Fatalf("captured %d bytes over an 8 byte limit", total)
	}
}
