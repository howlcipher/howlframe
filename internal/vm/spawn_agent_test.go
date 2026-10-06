package vm

import (
	"bytes"
	"fmt"
	"strings"
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
	returnBody := []bytecode.BCInstruction{{Op: bytecode.OpLoadConst, ValueOperand: int64(7)}, {Op: bytecode.OpReturn}}
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
		{"body return isolated", append(spawnTestBody("Return", returnBody...), spawnTestPrint("after")...), spawnTestStart("Return") + "after\n", "[Swarm VM] Agent \"Return\" failed task: \"work\": RETURN: return from spawn agent body\n", "", 0, 0},
		{"nested return isolated", spawnTestBody("Outer", append(spawnTestBody("Inner", returnBody...), spawnTestPrint("outer after")...)...), spawnTestStart("Outer") + spawnTestStart("Inner") + "outer after\n" + spawnTestDone("Outer"), "[Swarm VM] Agent \"Inner\" failed task: \"work\": RETURN: return from spawn agent body\n", "", 0, 0},
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
			if tc.code == "" && ev.ExitCode != 0 {
				t.Fatalf("exit %d, want 0", ev.ExitCode)
			}
		})
	}
}

func TestVMSpawnAgentInheritsArgvAndStdin(t *testing.T) {
	printValue := func(op bytecode.Opcode) []bytecode.BCInstruction {
		return []bytecode.BCInstruction{{Op: op}, {Op: bytecode.OpPrint, IntOperand: 1}}
	}
	argv := printValue(bytecode.OpCliArgs)
	argv = append(argv, bytecode.BCInstruction{Op: bytecode.OpLoadConst, ValueOperand: int64(0)}, bytecode.BCInstruction{Op: bytecode.OpCliArgsGet}, bytecode.BCInstruction{Op: bytecode.OpPrint, IntOperand: 1})
	read := printValue(bytecode.OpReadLine)
	interleaved := append([]bytecode.BCInstruction{}, read...)
	child := append([]bytecode.BCInstruction{}, read...)
	child = append(child, spawnTestBody("Inner", read...)...)
	interleaved = append(interleaved, spawnTestBody("Outer", child...)...)
	interleaved = append(interleaved, read...)
	for _, tc := range []struct {
		name        string
		insts       []bytecode.BCInstruction
		stdin, want string
	}{
		{"child argv", spawnTestBody("Outer", argv...), "", spawnTestStart("Outer") + "[alpha --beta]\nalpha\n" + spawnTestDone("Outer")},
		{"nested argv", spawnTestBody("Outer", spawnTestBody("Inner", argv...)...), "", spawnTestStart("Outer") + spawnTestStart("Inner") + "[alpha --beta]\nalpha\n" + spawnTestDone("Inner") + spawnTestDone("Outer")},
		{"child reads next line", append(append([]bytecode.BCInstruction{}, read...), spawnTestBody("Outer", read...)...), "line1\nline2\n", "line1\n" + spawnTestStart("Outer") + "line2\n" + spawnTestDone("Outer")},
		{"shared stream interleaving", interleaved, "line1\nline2\nline3\nline4\n", "line1\n" + spawnTestStart("Outer") + "line2\n" + spawnTestStart("Inner") + "line3\n" + spawnTestDone("Inner") + spawnTestDone("Outer") + "line4\n"},
		{"child EOF", spawnTestBody("Outer", read...), "", spawnTestStart("Outer") + "\n" + spawnTestDone("Outer")},
		{"parent reads after child", append(spawnTestBody("Outer", read...), read...), "line1\nline2\n", spawnTestStart("Outer") + "line1\n" + spawnTestDone("Outer") + "line2\n"},
		{"parent EOF after child drains", append(spawnTestBody("Outer", read...), read...), "only\n", spawnTestStart("Outer") + "only\n" + spawnTestDone("Outer") + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			ev := RunBytecodeWithEvidence(&bytecode.BCProgram{Main: tc.insts}, []string{"alpha", "--beta"}, DefaultExecutionPolicy(), []capability.Capability{capability.Process}, strings.NewReader(tc.stdin), &out, &stderr, 0)
			if ev.RuntimeFailure != nil || ev.ExitCode != 0 || out.String() != tc.want || stderr.String() != "" {
				t.Fatalf("failure %#v exit %d stdout %q stderr %q; want exit 0 stdout %q stderr empty", ev.RuntimeFailure, ev.ExitCode, out.String(), stderr.String(), tc.want)
			}
		})
	}
}
