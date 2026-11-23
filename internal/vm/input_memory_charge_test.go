package vm

import (
	"bytes"
	"strings"
	"testing"
)

// HF-D3: read_line charged nothing, so a line far larger than the memory
// budget was buffered whole and accepted.
func TestReadLineChargesMemory(t *testing.T) {
	_, prog := parseAndCompile(t, `(cli_app (let (line (read_line)) (print "read")))`)
	var out, errOut bytes.Buffer
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxMemoryBytes = 1024
	ev := RunBytecodeWithEvidence(prog, nil, policy, nil, strings.NewReader(strings.Repeat("x", 100000)+"\n"), &out, &errOut, 0)
	requireResourceFailure(t, ev.RuntimeFailure, "memory limit")
	if ev.RuntimeFailure.Opcode != "READ_LINE" {
		t.Fatalf("opcode = %q, want READ_LINE", ev.RuntimeFailure.Opcode)
	}
	if out.Len() != 0 {
		t.Fatalf("program continued after the limit: %q", out.String())
	}
}

// Lines longer than the bufio buffer must still arrive intact within budget.
func TestReadLineLongLineWithinBudget(t *testing.T) {
	_, prog := parseAndCompile(t, `(cli_app (let (a (read_line)) (let (b (read_line)) (print (str_len a) b))))`)
	var out, errOut bytes.Buffer
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxMemoryBytes = 1 << 20
	stdin := strings.Repeat("y", 10000) + "\r\nnext"
	if code := RunBytecodeWithPolicy(prog, nil, policy, nil, strings.NewReader(stdin), &out, &errOut); code != 0 {
		t.Fatalf("exit %d stderr %q", code, errOut.String())
	}
	if got := out.String(); got != "10000 next\n" {
		t.Fatalf("stdout = %q", got)
	}
}

// HF-D4: parse_json charged nothing for the decoded tree, so many small values
// decoded from input well inside the budget could use many times that memory.
func TestParseJSONChargesDecodedTree(t *testing.T) {
	_, prog := parseAndCompile(t, `(cli_app (let (raw (read_stdin)) (let (doc (parse_json Doc raw)) (print (list_len doc)))))`)
	payload := "[" + strings.Repeat(`{"a":[]},`, 9999) + `{"a":[]}]`
	var out, errOut bytes.Buffer
	policy := DefaultExecutionPolicy()
	// Room for the 90 KB input text, not for 10000 dicts and lists as well.
	policy.Limits.MaxMemoryBytes = len(payload) + 4096
	ev := RunBytecodeWithEvidence(prog, nil, policy, nil, strings.NewReader(payload), &out, &errOut, 0)
	requireResourceFailure(t, ev.RuntimeFailure, "memory limit")
	if ev.RuntimeFailure.Opcode != "PARSE_JSON" {
		t.Fatalf("opcode = %q, want PARSE_JSON", ev.RuntimeFailure.Opcode)
	}
}

func TestParseJSONWithinBudgetStillDecodes(t *testing.T) {
	_, prog := parseAndCompile(t, `(cli_app (let (raw (read_stdin)) (let (doc (parse_json Doc raw)) (print (map_get (map_get doc "k") "n")))))`)
	var out, errOut bytes.Buffer
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxMemoryBytes = 4096
	if code := RunBytecodeWithPolicy(prog, nil, policy, nil, strings.NewReader(`{"k":{"n":[1,"two",null]}}`), &out, &errOut); code != 0 {
		t.Fatalf("exit %d stderr %q", code, errOut.String())
	}
	if got := out.String(); got != "[1 two <nil>]\n" {
		t.Fatalf("stdout = %q", got)
	}
}
