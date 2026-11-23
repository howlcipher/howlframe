package vm

import (
	"encoding/json"
	"testing"
)

// The runtime error object is a consumer contract (improvements.md #112):
// every rendering carries the schema version and keeps the existing fields.
func TestRuntimeErrorCarriesSchema(t *testing.T) {
	plain := &VMError{Phase: "runtime", Code: "RUNTIME_ERROR", Function: "main", Instruction: 4, Opcode: "DIV", Message: "division by zero"}
	withExec := &VMError{Phase: "runtime", Code: "IO_ERROR", Function: "main", Message: "exec failed: exit status 3", ExecExitCode: 3, ExecOutput: []byte("boom")}
	cases := map[string]string{
		"Error":            plain.Error(),
		"CatchString":      plain.CatchString(),
		"CatchString exec": withExec.CatchString(),
	}
	for name, text := range cases {
		var doc map[string]any
		if err := json.Unmarshal([]byte(text), &doc); err != nil {
			t.Fatalf("%s: not JSON: %v: %s", name, err, text)
		}
		if doc["schema"] != RuntimeErrorSchema {
			t.Fatalf("%s: schema = %v, want %s", name, doc["schema"], RuntimeErrorSchema)
		}
		for _, field := range []string{"phase", "code", "function", "instruction", "message"} {
			if _, ok := doc[field]; !ok {
				t.Fatalf("%s: missing %q in %s", name, field, text)
			}
		}
	}
	var exec map[string]any
	_ = json.Unmarshal([]byte(withExec.CatchString()), &exec)
	if exec["exit_code"] != float64(3) || exec["output"] != "boom" {
		t.Fatalf("exec enrichment lost: %v", exec)
	}
	if RuntimeErrorSchema != "howlframe.runtime_error/v1" {
		t.Fatalf("schema constant changed: %s", RuntimeErrorSchema)
	}
}
