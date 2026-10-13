package harness

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func fakeClaude(t *testing.T, script string) ClaudeModel {
	d := t.TempDir()
	bin := filepath.Join(d, "claude")
	if e := os.WriteFile(bin, []byte("#!/bin/sh\ncat >/dev/null\n"+script), 0700); e != nil {
		t.Fatal(e)
	}
	return ClaudeModel{Work: d, Allowed: LiveAllowedModelIDs, Bin: bin}
}

// Lain blocker A: client failures and bad ids abort the run before scoring.
func TestClaudeCLIFailuresAreInfra(t *testing.T) {
	cases := map[string]string{
		"nonzero":     "echo boom >&2; exit 3\n",
		"unparseable": "echo 'not json'\n",
		"no id":       `echo '{"result":"x","usage":{},"modelUsage":{}}'` + "\n",
		"wrong id":    `echo '{"result":"x","usage":{},"modelUsage":{"other-model":{}}}'` + "\n",
	}
	for name, sc := range cases {
		_, e := fakeClaude(t, sc).Generate(context.Background(), Prompt{Text: "p"})
		var inf ErrInfra
		if !errors.As(e, &inf) {
			t.Errorf("%s: want ErrInfra, got %v", name, e)
		}
	}
	c, e := fakeClaude(t, `echo '{"result":"ok","usage":{"input_tokens":2},"modelUsage":{"claude-opus-5-5":{}}}'`+"\n").Generate(context.Background(), Prompt{Text: "p"})
	if e != nil || c.Text != "ok" {
		t.Fatalf("good response: %v %+v", e, c)
	}
}

// The runner returns ErrInfra (abort) and never runs/scores the child.
func TestRunAbortsOnClaudeFailure(t *testing.T) {
	m := fakeClaude(t, "exit 1\n")
	l := LiveRunner{Model: m, Deadline: LiveDefaultDeadline, Self: "/nonexistent"}
	out := filepath.Join(t.TempDir(), "cell")
	_, e := l.Run(context.Background(), LiveTask{Task: Task{ID: "t01"}, Set: "tuning"}, "a", "rep1", out, filepath.Join(t.TempDir(), "v.jsonl"))
	var inf ErrInfra
	if !errors.As(e, &inf) {
		t.Fatalf("want ErrInfra, got %v", e)
	}
	if _, err := os.Stat(filepath.Join(out, "live_result.json")); err == nil {
		t.Fatal("aborted cell must not be scored")
	}
}
