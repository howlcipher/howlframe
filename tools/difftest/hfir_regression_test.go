package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// Focused regressions for #155 and #159: each fixture isolates one HFIR
// lowering root cause and must agree on HFIR bytecode, AST bytecode and the
// interpreter. Go and JavaScript are excluded on purpose: they reject
// parse_json-based programs as UNSUPPORTED_ON_TARGET, which is not what these
// fixtures pin.
func TestHFIRRegressionParity(t *testing.T) {
	cases := map[string]string{
		// parse_json evaluated its symbol body onto the stack in addition to
		// PARSE_JSON reading it by name; the stray raw string was printed
		// instead of the re-encoded JSON.
		"155_parse_json_operand_stack.howl": `{"a":[2,{"b":null}],"z":1} 1
a!,z!
2 {"a":[2,{"b":null}],"z":1}`,
		// map_get whose dict is itself an expression failed to lower.
		"159_map_get_expression_target.howl": "map[name:kim]\nkim\n2\n7",
	}
	targets := []Target{TargetHFIRBytecode, TargetBytecode, TargetInterpreter}
	for name, want := range cases {
		name, want := name, want
		t.Run(name, func(t *testing.T) {
			report, err := VerifyParity(filepath.Join("..", "..", "tests", "hfir_parity", name), nil, "", targets)
			if err != nil {
				t.Fatal(err)
			}
			if report.OverallStatus != StatusPass {
				t.Fatalf("parity mismatch:\n%s", strings.Join(report.Discrepancies, "\n"))
			}
			for target, result := range report.TargetResults {
				if NormalizeOutput(result.Stdout) != want {
					t.Errorf("%s stdout %q, want %q", target, result.Stdout, want)
				}
			}
		})
	}
}
