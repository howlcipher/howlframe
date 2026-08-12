package bytecode

import (
	"bytes"
	"testing"
)

func TestArtifactRoundTrip(t *testing.T) {
	prog := &BCProgram{
		Version: 1,
		Main: []BCInstruction{
			{OpString: "LOAD_CONST", ValueOperand: int64(42)},
			{OpString: "EXIT"},
		},
	}
	
	var buf bytes.Buffer
	if err := WriteArtifact(&buf, prog); err != nil {
		t.Fatalf("WriteArtifact failed: %v", err)
	}
	
	decoded, err := ReadArtifact(&buf)
	if err != nil {
		t.Fatalf("ReadArtifact failed: %v", err)
	}
	
	if len(decoded.Main) != 2 {
		t.Errorf("Expected 2 instructions, got %d", len(decoded.Main))
	}
}

func TestArtifactCorruptMagic(t *testing.T) {
	prog := &BCProgram{}
	var buf bytes.Buffer
	WriteArtifact(&buf, prog)
	
	b := buf.Bytes()
	b[0] = 'X' // corrupt magic
	
	_, err := ReadArtifact(bytes.NewReader(b))
	if err != ErrInvalidMagic {
		t.Errorf("Expected ErrInvalidMagic, got %v", err)
	}
}

func TestArtifactCorruptVersion(t *testing.T) {
	prog := &BCProgram{}
	var buf bytes.Buffer
	WriteArtifact(&buf, prog)
	
	b := buf.Bytes()
	b[4] = 99 // corrupt version
	
	_, err := ReadArtifact(bytes.NewReader(b))
	if err == nil {
		t.Errorf("Expected error for unsupported version")
	}
}

func TestArtifactCorruptPayload(t *testing.T) {
	prog := &BCProgram{}
	var buf bytes.Buffer
	WriteArtifact(&buf, prog)
	
	b := buf.Bytes()
	b[len(b)-1] ^= 0xff // flip bit in payload
	
	_, err := ReadArtifact(bytes.NewReader(b))
	if err != ErrCorrupt {
		t.Errorf("Expected ErrCorrupt, got %v", err)
	}
}

func TestArtifactTrailingGarbage(t *testing.T) {
	prog := &BCProgram{}
	var buf bytes.Buffer
	WriteArtifact(&buf, prog)
	
	buf.Write([]byte("garbage"))
	
	_, err := ReadArtifact(&buf)
	if err != ErrCorrupt {
		t.Errorf("Expected ErrCorrupt, got %v", err)
	}
}

func TestArtifactTruncated(t *testing.T) {
	prog := &BCProgram{}
	var buf bytes.Buffer
	WriteArtifact(&buf, prog)
	
	b := buf.Bytes()
	_, err := ReadArtifact(bytes.NewReader(b[:len(b)-5]))
	if err != ErrTruncated {
		t.Errorf("Expected ErrTruncated, got %v", err)
	}
}
