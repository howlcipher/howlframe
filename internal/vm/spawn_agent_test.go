package vm

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

func spawnTestBody(name string, body ...bytecode.BCInstruction) []bytecode.BCInstruction {
	return append([]bytecode.BCInstruction{{Op: bytecode.OpTask, StringOperand: "work"}, {Op: bytecode.OpSpawnAgent, StringOperand: name, IntOperand: int64(len(body))}}, body...)
}
func spawnTestPrint(s string) []bytecode.BCInstruction {
	return []bytecode.BCInstruction{{Op: bytecode.OpLoadConst, ValueOperand: s}, {Op: bytecode.OpPrint, IntOperand: 1}}
}
func spawnTestStart(name string) string {
	return fmt.Sprintf("[Swarm VM] Spawning agent %q for task: %q\n", name, "work")
}
func spawnTestDone(name string) string {
	return fmt.Sprintf("[Swarm VM] Agent %q completed task: %q\n", name, "work")
}

func TestVMSpawnAgentNesting(t *testing.T) {
	failure := []bytecode.BCInstruction{{Op: bytecode.OpLoadConst, ValueOperand: "HOME"}, {Op: bytecode.OpEnv}}
	three := spawnTestBody("Outer", spawnTestBody("Middle", spawnTestBody("Inner", spawnTestPrint("leaf")...)...)...)
	nestedFailure := spawnTestBody("Outer", append(spawnTestBody("Inner", failure...), spawnTestPrint("outer after")...)...)
	writes := []bytecode.BCInstruction{{Op: bytecode.OpLoadConst, ValueOperand: "child"}, {Op: bytecode.OpSetVar, StringOperand: "x"}, {Op: bytecode.OpLoadConst, ValueOperand: "leftover"}}
	vars := []bytecode.BCInstruction{{Op: bytecode.OpLoadConst, ValueOperand: "parent"}, {Op: bytecode.OpStoreVar, StringOperand: "x"}, {Op: bytecode.OpLoadConst, ValueOperand: "sentinel"}}
	vars = append(vars, spawnTestBody("Writer", writes...)...)
	vars = append(vars, bytecode.BCInstruction{Op: bytecode.OpPrint, IntOperand: 1}, bytecode.BCInstruction{Op: bytecode.OpLoadVar, StringOperand: "x"}, bytecode.BCInstruction{Op: bytecode.OpPrint, IntOperand: 1})
	maps := []bytecode.BCInstruction{{Op: bytecode.OpLoadConst, ValueOperand: map[string]any{"key": "parent"}}, {Op: bytecode.OpStoreVar, StringOperand: "m"}}
	maps = append(maps, spawnTestBody("Writer",
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, ValueOperand: "key"},
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, ValueOperand: "child"},
		bytecode.BCInstruction{Op: bytecode.OpMapSet, StringOperand: "m"},
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, ValueOperand: "HOME"},
		bytecode.BCInstruction{Op: bytecode.OpEnv},
	)...)
	maps = append(maps, bytecode.BCInstruction{Op: bytecode.OpLoadConst, ValueOperand: "key"}, bytecode.BCInstruction{Op: bytecode.OpMapGet, StringOperand: "m"}, bytecode.BCInstruction{Op: bytecode.OpPrint, IntOperand: 1})

	for _, tc := range []struct {
		name           string
		insts          []bytecode.BCInstruction
		out, err, code string
		budget, depth  int
	}{
		{"three levels", three, spawnTestStart("Outer") + spawnTestStart("Middle") + spawnTestStart("Inner") + "leaf\n" + spawnTestDone("Inner") + spawnTestDone("Middle") + spawnTestDone("Outer"), "", "", 0, 0},
		{"nested failure", nestedFailure, spawnTestStart("Outer") + spawnTestStart("Inner") + "outer after\n" + spawnTestDone("Outer"), "[Swarm VM] Agent \"Inner\" failed task: \"work\": CAPABILITY_DENIED: capability denied: environment\n", "", 0, 0},
		{"failed child then sibling", append(append(spawnTestBody("Bad", failure...), spawnTestBody("Good", spawnTestPrint("sibling")...)...), spawnTestPrint("parent after")...), spawnTestStart("Bad") + spawnTestStart("Good") + "sibling\n" + spawnTestDone("Good") + "parent after\n", "[Swarm VM] Agent \"Bad\" failed task: \"work\": CAPABILITY_DENIED: capability denied: environment\n", "", 0, 0},
		{"type error isolated", append(spawnTestBody("Bad", bytecode.BCInstruction{Op: bytecode.OpLoadConst, ValueOperand: int64(1)}, bytecode.BCInstruction{Op: bytecode.OpEnv}), spawnTestPrint("after")...), spawnTestStart("Bad") + "after\n", "[Swarm VM] Agent \"Bad\" failed task: \"work\": TYPE_ERROR: env expected string name, got int64\n", "", 0, 0},
		{"map writes before failure stay isolated", maps, spawnTestStart("Writer") + "parent\n", "[Swarm VM] Agent \"Writer\" failed task: \"work\": CAPABILITY_DENIED: capability denied: environment\n", "", 0, 0},
		{"bindings and stack", vars, spawnTestStart("Writer") + spawnTestDone("Writer") + "sentinel\nparent\n", "", "", 0, 0},
		{"exact shared budget", three, spawnTestStart("Outer") + spawnTestStart("Middle") + spawnTestStart("Inner") + "leaf\n" + spawnTestDone("Inner") + spawnTestDone("Middle") + spawnTestDone("Outer"), "", "", 8, 0},
		{"failed work counts", append(spawnTestBody("Bad", failure...), spawnTestPrint("after")...), spawnTestStart("Bad"), "[Swarm VM] Agent \"Bad\" failed task: \"work\": CAPABILITY_DENIED: capability denied: environment\n", "LIMIT_EXCEEDED", 4, 0},
		{"shared budget", three, spawnTestStart("Outer") + spawnTestStart("Middle") + spawnTestStart("Inner"), "", "LIMIT_EXCEEDED", 6, 0},
		{"depth guard", three, spawnTestStart("Outer") + spawnTestStart("Middle"), "", "LIMIT_EXCEEDED", 0, 2},
		{"return escapes", spawnTestBody("Return", bytecode.BCInstruction{Op: bytecode.OpLoadConst, ValueOperand: int64(7)}, bytecode.BCInstruction{Op: bytecode.OpReturn}), spawnTestStart("Return"), "", "", 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := DefaultExecutionPolicy()
			if tc.budget > 0 {
				policy.Limits.MaxInstructions = tc.budget
			}
			if tc.depth > 0 {
				policy.Limits.MaxCallDepth = tc.depth
			}
			var out, err bytes.Buffer
			caps := []capability.Capability{capability.Process}
			if tc.name == "type error isolated" {
				caps = append(caps, capability.Environment)
			}
			ev := RunBytecodeWithEvidence(&bytecode.BCProgram{Main: tc.insts}, nil, policy, caps, nil, &out, &err, 0)
			code := ""
			if ev.RuntimeFailure != nil {
				code = ev.RuntimeFailure.Code
			}
			if code != tc.code || out.String() != tc.out || err.String() != tc.err {
				t.Fatalf("failure %#v stdout %q stderr %q; want code %q stdout %q stderr %q", ev.RuntimeFailure, out.String(), err.String(), tc.code, tc.out, tc.err)
			}
			if tc.name == "return escapes" && ev.ExitCode != 7 {
				t.Fatalf("exit %d, want 7", ev.ExitCode)
			}
		})
	}
}
