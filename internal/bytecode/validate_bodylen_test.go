package bytecode

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestValidateBodyLengths(t *testing.T) {
	type bodyCase struct {
		name      string
		op        Opcode
		lengths   [3]int64
		remaining int
		segment   string
		valid     bool
	}
	cases := []bodyCase{}
	for _, op := range []Opcode{OpSpawnAgent, OpSpawn, OpHttpRoute} {
		for _, length := range []int64{-1, 3, math.MaxInt64} {
			cases = append(cases, bodyCase{fmt.Sprintf("%s/%d", Registry[op].Name, length), op, [3]int64{length}, 2, "body", false})
		}
		for _, length := range []int64{0, 2} {
			cases = append(cases, bodyCase{fmt.Sprintf("%s/valid/%d", Registry[op].Name, length), op, [3]int64{length}, int(length), "", true})
		}
	}
	cases = append(cases,
		bodyCase{"try/value-negative", OpTryLet, [3]int64{-1, 0, 0}, 3, "value", false},
		bodyCase{"try/catch-negative", OpTryLet, [3]int64{1, -1, 0}, 3, "catch", false},
		bodyCase{"try/success-negative", OpTryLet, [3]int64{1, 1, -1}, 3, "success", false},
		bodyCase{"try/sum-past-end", OpTryLet, [3]int64{1, 1, 2}, 3, "success", false},
		bodyCase{"try/catch-past-end", OpTryLet, [3]int64{2, 2, 0}, 3, "catch", false},
		bodyCase{"try/overflow", OpTryLet, [3]int64{math.MaxInt64, math.MaxInt64, math.MaxInt64}, 3, "value", false},
		bodyCase{"try/catch-overflow", OpTryLet, [3]int64{1, math.MaxInt64, 1}, 3, "catch", false},
		bodyCase{"try/success-overflow", OpTryLet, [3]int64{1, 1, math.MaxInt64}, 3, "success", false},
		bodyCase{"try/exact", OpTryLet, [3]int64{1, 1, 1}, 3, "", true},
		bodyCase{"try/zero", OpTryLet, [3]int64{}, 0, "", true},
	)
	for _, tc := range cases {
		for _, function := range []bool{false, true} {
			for _, stringOnly := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/function=%t/stringOnly=%t", tc.name, function, stringOnly), func(t *testing.T) {
					insts := []BCInstruction{{Op: OpLoadConst, OpString: "LOAD_CONST"}, {Op: tc.op, OpString: Registry[tc.op].Name, IntOperand: tc.lengths[0], IntOperand2: tc.lengths[1], IntOperand3: tc.lengths[2]}}
					if stringOnly {
						insts[1].Op = 0
					}
					for i := 0; i < tc.remaining; i++ {
						insts = append(insts, BCInstruction{Op: OpLoadConst, OpString: "LOAD_CONST"})
					}
					prog := &BCProgram{Version: 1, Main: insts}
					if function {
						prog.Main = nil
						prog.Functions = map[string]*BCFunction{"test": {Instructions: insts}}
					}
					err := ValidateProgram(prog)
					if tc.valid {
						if err != nil {
							t.Fatalf("valid program rejected: %v", err)
						}
						return
					}
					if err == nil {
						t.Fatal("corrupt program accepted")
					}
					if !strings.Contains(err.Error(), tc.segment) {
						t.Fatalf("error does not identify %s segment: %v", tc.segment, err)
					}
				})
			}
		}
	}
}

func TestValidateJumpOffsets(t *testing.T) {
	for _, op := range []Opcode{OpJump, OpJumpIfFalse, OpForNext} {
		for _, offset := range []int64{math.MaxInt64, math.MinInt64, -2, 2, -1, 0, 1} {
			t.Run(fmt.Sprintf("%s/%d", Registry[op].Name, offset), func(t *testing.T) {
				prog := &BCProgram{Main: []BCInstruction{{Op: OpLoadConst, OpString: "LOAD_CONST"}, {Op: op, OpString: Registry[op].Name, IntOperand: offset}}}
				err := ValidateProgram(prog)
				valid := offset >= -1 && offset <= 1
				if (err == nil) != valid {
					t.Fatalf("offset %d: valid=%t, error=%v", offset, valid, err)
				}
			})
		}
	}
}

func TestArtifactRejectsCorruptSpawnAgentBody(t *testing.T) {
	prog := &BCProgram{Version: 1, Main: []BCInstruction{{Op: OpSpawnAgent, OpString: "SPAWN_AGENT", IntOperand: math.MaxInt64}}}
	var buf bytes.Buffer
	if err := WriteArtifact(&buf, prog); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadArtifact(&buf)
	if !errors.Is(err, ErrInvalidProgram) {
		t.Fatalf("expected ErrInvalidProgram, got %v", err)
	}
	if loaded != nil {
		t.Fatal("corrupt program returned by loader")
	}
}
