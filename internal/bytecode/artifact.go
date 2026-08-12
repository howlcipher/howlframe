package bytecode

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
)

const (
	ArtifactMagic   = "HFBC"
	ArtifactVersion = uint32(1)
)

var (
	ErrInvalidMagic   = errors.New("invalid artifact magic identifier")
	ErrUnsupportedVer = errors.New("unsupported artifact version")
	ErrCorrupt        = errors.New("artifact integrity checksum mismatch or corrupted payload")
	ErrTruncated      = errors.New("truncated artifact")
	ErrOversized      = errors.New("artifact payload exceeds trusted maximum size")
)

const (
	// MaxArtifactPayloadSize is the maximum allowed payload size for an HFBC artifact.
	// Current observed max applications (repo analyst, action executor) are ~10-15KB.
	// A 10MB limit provides substantial headroom while preventing OOM attacks from malformed lengths.
	MaxArtifactPayloadSize = 10 * 1024 * 1024
)

// WriteArtifact serializes the BCProgram with an explicit versioned envelope
// containing a magic identifier, version, checksum, and gob-encoded payload.
func WriteArtifact(w io.Writer, prog *BCProgram) error {
	var payload bytes.Buffer
	enc := gob.NewEncoder(&payload)
	if err := enc.Encode(prog); err != nil {
		return err
	}

	payloadBytes := payload.Bytes()
	hash := sha256.Sum256(payloadBytes)

	// Magic: 4 bytes
	if _, err := w.Write([]byte(ArtifactMagic)); err != nil {
		return err
	}

	// Version: 4 bytes (uint32)
	if err := binary.Write(w, binary.LittleEndian, ArtifactVersion); err != nil {
		return err
	}

	// Payload length: 4 bytes (uint32)
	if err := binary.Write(w, binary.LittleEndian, uint32(len(payloadBytes))); err != nil {
		return err
	}

	// Checksum: 32 bytes (SHA-256)
	if _, err := w.Write(hash[:]); err != nil {
		return err
	}

	// Payload
	if _, err := w.Write(payloadBytes); err != nil {
		return err
	}

	return nil
}

// ReadArtifact reads and verifies a HowlFrame bytecode artifact.
func ReadArtifact(r io.Reader) (*BCProgram, error) {
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, ErrTruncated
		}
		return nil, err
	}
	if string(magic) != ArtifactMagic {
		return nil, ErrInvalidMagic
	}

	var version uint32
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, ErrTruncated
		}
		return nil, err
	}
	if version != ArtifactVersion {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrUnsupportedVer, ArtifactVersion, version)
	}

	var payloadLen uint32
	if err := binary.Read(r, binary.LittleEndian, &payloadLen); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, ErrTruncated
		}
		return nil, err
	}
	if payloadLen > MaxArtifactPayloadSize {
		return nil, ErrOversized
	}

	var expectedHash [32]byte
	if _, err := io.ReadFull(r, expectedHash[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, ErrTruncated
		}
		return nil, err
	}

	payloadBytes := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payloadBytes); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil, ErrTruncated
		}
		return nil, err
	}

	actualHash := sha256.Sum256(payloadBytes)
	if actualHash != expectedHash {
		return nil, ErrCorrupt
	}

	// Check for trailing garbage
	var dummy [1]byte
	if n, _ := r.Read(dummy[:]); n > 0 {
		return nil, ErrCorrupt // Trailing garbage is treated as corruption
	}

	var prog BCProgram
	dec := gob.NewDecoder(bytes.NewReader(payloadBytes))
	if err := dec.Decode(&prog); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}

	if err := ValidateProgram(&prog); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidProgram, err)
	}

	return &prog, nil
}

var ErrInvalidProgram = errors.New("structurally invalid bytecode program")

// ValidateProgram performs structural verification on a decoded BCProgram
// to ensure it cannot violate VM invariants (e.g., out-of-bounds jumps,
// unknown opcodes, or missing functions) leading to Go panics.
func ValidateProgram(prog *BCProgram) error {
	if prog == nil {
		return errors.New("nil program")
	}

	if err := validateInstructions(prog.Main, prog); err != nil {
		return fmt.Errorf("main: %w", err)
	}

	for name, fn := range prog.Functions {
		if fn == nil {
			return fmt.Errorf("function %q is nil", name)
		}
		if err := validateInstructions(fn.Instructions, prog); err != nil {
			return fmt.Errorf("function %q: %w", name, err)
		}
	}

	return nil
}

func validateInstructions(insts []BCInstruction, prog *BCProgram) error {
	for i, inst := range insts {
		op := inst.Op
		if op == 0 {
			// fallback if gob deserialized 0 but string was set
			o, ok := NameToOpcode(inst.OpString)
			if !ok {
				return fmt.Errorf("instruction %d: unknown opcode %q", i, inst.OpString)
			}
			op = o
		} else if _, ok := Registry[op]; !ok {
			return fmt.Errorf("instruction %d: unknown opcode %d (%q)", i, op, inst.OpString)
		}

		// Jump targets
		if op == OpJump || op == OpJumpIfFalse {
			target := i + int(inst.IntOperand)
			if target < 0 || target > len(insts) {
				return fmt.Errorf("instruction %d: jump target %d out of bounds [0, %d]", i, target, len(insts))
			}
		}

		// Function calls
		if op == OpCall {
			if _, exists := prog.Functions[inst.StringOperand]; !exists {
				return fmt.Errorf("instruction %d: references missing function %q", i, inst.StringOperand)
			}
		}
	}
	return nil
}
