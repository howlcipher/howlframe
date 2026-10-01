package bytecode

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"sort"
)

const (
	ArtifactMagic = "HFBC"
	// ArtifactVersion is the version WriteArtifact emits. Version 2 stores
	// functions as a name-sorted list, so identical programs serialize to
	// identical bytes. Version 1 gob-encoded the Functions map directly, and
	// gob writes maps in Go's randomized iteration order.
	ArtifactVersion = uint32(2)
	// artifactVersionMapPayload is still accepted by ReadArtifact.
	artifactVersionMapPayload = uint32(1)
)

// artifactPayload is the version 2 wire form of a BCProgram.
type artifactPayload struct {
	Version   int
	Functions []*BCFunction
	Names     []string
	Main      []BCInstruction
}

func newArtifactPayload(prog *BCProgram) artifactPayload {
	names := make([]string, 0, len(prog.Functions))
	for name := range prog.Functions {
		names = append(names, name)
	}
	sort.Strings(names)
	fns := make([]*BCFunction, len(names))
	for i, name := range names {
		fns[i] = prog.Functions[name]
	}
	return artifactPayload{Version: prog.Version, Functions: fns, Names: names, Main: prog.Main}
}

func (p artifactPayload) program() (*BCProgram, error) {
	if len(p.Names) != len(p.Functions) {
		return nil, fmt.Errorf("function table has %d names for %d functions", len(p.Names), len(p.Functions))
	}
	prog := &BCProgram{Version: p.Version, Main: p.Main, Functions: make(map[string]*BCFunction, len(p.Names))}
	for i, name := range p.Names {
		if _, dup := prog.Functions[name]; dup {
			return nil, fmt.Errorf("duplicate function %q", name)
		}
		prog.Functions[name] = p.Functions[i]
	}
	return prog, nil
}

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
	if err := enc.Encode(newArtifactPayload(prog)); err != nil {
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
	if version != ArtifactVersion && version != artifactVersionMapPayload {
		return nil, fmt.Errorf("%w: expected %d or %d, got %d", ErrUnsupportedVer, artifactVersionMapPayload, ArtifactVersion, version)
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

	dec := gob.NewDecoder(bytes.NewReader(payloadBytes))
	var prog *BCProgram
	if version == artifactVersionMapPayload {
		prog = &BCProgram{}
		if err := dec.Decode(prog); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
		}
	} else {
		var wire artifactPayload
		if err := dec.Decode(&wire); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
		}
		decoded, err := wire.program()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
		}
		prog = decoded
	}

	if err := ValidateProgram(prog); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidProgram, err)
	}

	return prog, nil
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
