package vm

import (
	"bytes"
	"strings"
	"testing"
)

// read_line cannot tell end of input from an empty line and strips "\r".
// read_stdin returns the remaining input byte-exact, so a program can count
// physical lines and see where input really ends (Milestone 1, ledgerlens R13).
func TestReadStdinIsByteExact(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		stdin string
		want  string
	}{
		{"blank lines and CRLF kept", `(cli_app (print (encode_json (list (read_stdin)))))`, "a\n\nb\r\nlast", `["a\n\nb\r\nlast"]` + "\n"},
		{"empty stdin is empty string", `(cli_app (print (str_len (read_stdin))))`, "", "0\n"},
		{"read_line then rest", `(cli_app (let (first (read_line)) (print first "|" (str_len (read_stdin)))))`, "x\ny\nz\n", "x | 4\n"},
		{"second call sees nothing", `(cli_app (let (a (read_stdin)) (print (str_len (read_stdin)))))`, "abc", "0\n"},
		{"non-UTF-8 and NUL bytes preserved in length", `(cli_app (print (str_len (read_stdin))))`, "a\x00b", "3\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name+"_interpret", func(t *testing.T) {
			node, _ := parseAndCompile(t, tt.code)
			var out, errOut bytes.Buffer
			if code := Interpret(node, nil, nil, strings.NewReader(tt.stdin), &out, &errOut); code != 0 {
				t.Fatalf("exit %d stderr %q", code, errOut.String())
			}
			if out.String() != tt.want {
				t.Fatalf("stdout = %q, want %q", out.String(), tt.want)
			}
		})
		t.Run(tt.name+"_bytecode", func(t *testing.T) {
			_, prog := parseAndCompile(t, tt.code)
			var out, errOut bytes.Buffer
			if code := RunBytecode(prog, nil, nil, strings.NewReader(tt.stdin), &out, &errOut); code != 0 {
				t.Fatalf("exit %d stderr %q", code, errOut.String())
			}
			if out.String() != tt.want {
				t.Fatalf("stdout = %q, want %q", out.String(), tt.want)
			}
		})
	}
}

func TestReadStdinChargesMemory(t *testing.T) {
	_, prog := parseAndCompile(t, `(cli_app (let (all (read_stdin)) (print "read")))`)
	var out, errOut bytes.Buffer
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxMemoryBytes = 1024
	ev := RunBytecodeWithEvidence(prog, nil, policy, nil, strings.NewReader(strings.Repeat("x", 100000)), &out, &errOut, 0)
	requireResourceFailure(t, ev.RuntimeFailure, "memory limit")
}

// starts_with desugars through str_split, and splitting "" yields [""]; an
// empty subject must not match a non-empty prefix (found by ledgerlens trim).
func TestStartsEndsWithEmptySubject(t *testing.T) {
	code := `(cli_app (print (starts_with "" " ") (ends_with "" " ") (starts_with "" "") (starts_with "ab" "") (starts_with "ab" "a") (starts_with "ab" "b") (starts_with "ab" "abc") (ends_with "ab" "b") (ends_with "ab" "a")))`
	want := "false false true true true false false true false\n"
	node, prog := parseAndCompile(t, code)
	var out, errOut bytes.Buffer
	if c := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut); c != 0 || out.String() != want {
		t.Fatalf("interpret: exit %d stdout %q stderr %q, want %q", c, out.String(), errOut.String(), want)
	}
	out.Reset()
	errOut.Reset()
	if c := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); c != 0 || out.String() != want {
		t.Fatalf("bytecode: exit %d stdout %q stderr %q, want %q", c, out.String(), errOut.String(), want)
	}
}
