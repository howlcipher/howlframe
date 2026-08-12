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
	
	return &prog, nil
}
