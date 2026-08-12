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

func TestArtifactOversizedPayload(t *testing.T) {
	// Construct malicious headers
	cases := []struct {
		name       string
		payloadLen uint32
		expectErr  error
	}{
		{"exactly at allowed boundary", MaxArtifactPayloadSize, ErrTruncated}, // It should pass the size check, then fail reading bytes because we don't provide them
		{"one above allowed boundary", MaxArtifactPayloadSize + 1, ErrOversized},
		{"extremely large", 0xffffffff, ErrOversized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			buf.WriteString(ArtifactMagic)

			// Version
			var versionBuf [4]byte
			versionBuf[0] = 1
			buf.Write(versionBuf[:])

			// Payload length
			var lenBuf [4]byte
			lenBuf[0] = byte(tc.payloadLen)
			lenBuf[1] = byte(tc.payloadLen >> 8)
			lenBuf[2] = byte(tc.payloadLen >> 16)
			lenBuf[3] = byte(tc.payloadLen >> 24)
			buf.Write(lenBuf[:])

			_, err := ReadArtifact(&buf)
			if err != tc.expectErr {
				t.Errorf("Expected %v, got %v", tc.expectErr, err)
			}
		})
	}
}

func TestArtifactStructurallyInvalid(t *testing.T) {
	cases := []struct {
		name string
		prog *BCProgram
	}{
		{
			name: "unknown opcode",
			prog: &BCProgram{
				Version: 1,
				Main: []BCInstruction{
					{OpString: "DOES_NOT_EXIST"},
				},
			},
		},
		{
			name: "jump out of bounds positive",
			prog: &BCProgram{
				Version: 1,
				Main: []BCInstruction{
					{OpString: "JUMP", Op: OpJump, IntOperand: 2}, // len is 1, max target is 1
				},
			},
		},
		{
			name: "jump out of bounds negative",
			prog: &BCProgram{
				Version: 1,
				Main: []BCInstruction{
					{OpString: "JUMP", Op: OpJump, IntOperand: -1},
				},
			},
		},
		{
			name: "missing function reference",
			prog: &BCProgram{
				Version: 1,
				Main: []BCInstruction{
					{OpString: "CALL", Op: OpCall, StringOperand: "missing_func", IntOperand: 0},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteArtifact(&buf, tc.prog); err != nil {
				t.Fatalf("WriteArtifact failed: %v", err)
			}

			_, err := ReadArtifact(&buf)
			if err == nil {
				t.Errorf("Expected structurally invalid program to fail, but it passed")
			}
		})
	}
}
