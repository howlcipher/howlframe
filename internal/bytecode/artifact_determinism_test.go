package bytecode

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"testing"
)

func multiFunctionProgram() *BCProgram {
	prog := &BCProgram{Version: 1, Functions: map[string]*BCFunction{}}
	for i := 0; i < 8; i++ {
		name := fmt.Sprintf("f%d", i)
		prog.Functions[name] = &BCFunction{
			Name:   name,
			Params: []string{"x"},
			Instructions: []BCInstruction{
				{OpString: "LOAD_VAR", Op: OpLoadVar, StringOperand: "x"},
				{OpString: "RETURN", Op: OpReturn},
			},
		}
	}
	prog.Main = []BCInstruction{
		{OpString: "LOAD_CONST", Op: OpLoadConst, ValueOperand: 1.0},
		{OpString: "CALL", Op: OpCall, StringOperand: "f3", IntOperand: 1},
		{OpString: "PRINT", Op: OpPrint, IntOperand: 1},
	}
	return prog
}

// Identical programs must serialize to identical bytes so an artifact hash
// can be reproduced from source and used as a provenance identity.
func TestWriteArtifactIsDeterministic(t *testing.T) {
	var first []byte
	for i := 0; i < 50; i++ {
		var buf bytes.Buffer
		if err := WriteArtifact(&buf, multiFunctionProgram()); err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = buf.Bytes()
			continue
		}
		if !bytes.Equal(first, buf.Bytes()) {
			t.Fatalf("serialization %d differs from the first", i)
		}
	}
}

func TestWriteArtifactRoundTripsFunctions(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteArtifact(&buf, multiFunctionProgram()); err != nil {
		t.Fatal(err)
	}
	prog, err := ReadArtifact(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Functions) != 8 || prog.Functions["f5"].Name != "f5" {
		t.Fatalf("functions not preserved: %d", len(prog.Functions))
	}
}

func envelope(version uint32, payload []byte) *bytes.Buffer {
	var buf bytes.Buffer
	buf.WriteString(ArtifactMagic)
	binary.Write(&buf, binary.LittleEndian, version)
	binary.Write(&buf, binary.LittleEndian, uint32(len(payload)))
	sum := sha256.Sum256(payload)
	buf.Write(sum[:])
	buf.Write(payload)
	return &buf
}

// Artifacts written before version 2 keep loading.
func TestReadArtifactAcceptsVersion1MapPayload(t *testing.T) {
	var payload bytes.Buffer
	if err := gob.NewEncoder(&payload).Encode(multiFunctionProgram()); err != nil {
		t.Fatal(err)
	}
	prog, err := ReadArtifact(envelope(1, payload.Bytes()))
	if err != nil {
		t.Fatalf("version 1 artifact rejected: %v", err)
	}
	if len(prog.Functions) != 8 {
		t.Fatalf("functions = %d", len(prog.Functions))
	}
}

func TestReadArtifactRejectsDuplicateFunctionNames(t *testing.T) {
	fn := &BCFunction{Name: "f", Instructions: []BCInstruction{{OpString: "RETURN", Op: OpReturn}}}
	var payload bytes.Buffer
	wire := artifactPayload{Version: 1, Functions: []*BCFunction{fn, fn}, Names: []string{"f", "f"}}
	if err := gob.NewEncoder(&payload).Encode(wire); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadArtifact(envelope(ArtifactVersion, payload.Bytes())); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("err = %v, want ErrCorrupt", err)
	}
}
