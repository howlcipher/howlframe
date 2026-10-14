package bytecode

import (
	"bytes"
	"encoding/gob"
	"errors"
	"testing"
)

func readNamedOpcodeFixture(t *testing.T, version uint32, prog *BCProgram) (*BCProgram, error) {
	t.Helper()
	if version == artifactVersionMapPayload {
		var payload bytes.Buffer
		if err := gob.NewEncoder(&payload).Encode(prog); err != nil {
			t.Fatal(err)
		}
		return ReadArtifact(envelope(version, payload.Bytes()))
	}
	var artifact bytes.Buffer
	if err := WriteArtifact(&artifact, prog); err != nil {
		t.Fatal(err)
	}
	return ReadArtifact(&artifact)
}

func TestArtifactOpcodeNamesSurviveNumericEnumChanges(t *testing.T) {
	for _, version := range []uint32{artifactVersionMapPayload, ArtifactVersion} {
		t.Run(string(rune('0'+version)), func(t *testing.T) {
			prog := &BCProgram{Version: 1, Functions: map[string]*BCFunction{
				"convert": {Name: "convert", Instructions: []BCInstruction{
					// Release v0.1.0's CONVERT number became CALL after enum insertions.
					{Op: OpCall, OpString: "CONVERT", StringOperand: "bytes_to_string"},
				}},
			}, Main: []BCInstruction{
				{Op: OpForNext, OpString: "PRINT", IntOperand: 1},
			}}
			decoded, err := readNamedOpcodeFixture(t, version, prog)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Main[0].Op != OpPrint || decoded.Functions["convert"].Instructions[0].Op != OpConvert {
				t.Fatal("artifact opcode names did not retain their semantics")
			}
		})
	}
}

func TestArtifactNamedOpcodesStillRequireStructuralValidation(t *testing.T) {
	for _, instruction := range []BCInstruction{
		{Op: OpLoadConst, OpString: "UNKNOWN_WIRE_OPCODE"},
		{Op: OpLoadConst, OpString: "JUMP", IntOperand: 2},
		{Op: OpLoadConst, OpString: "CALL", StringOperand: "missing"},
		{Op: Opcode(65535)},
	} {
		for _, version := range []uint32{artifactVersionMapPayload, ArtifactVersion} {
			_, err := readNamedOpcodeFixture(t, version, &BCProgram{Main: []BCInstruction{instruction}})
			if !errors.Is(err, ErrInvalidProgram) {
				t.Fatalf("version%d accepted invalid named instruction %+v: %v", version, instruction, err)
			}
		}
	}
}
