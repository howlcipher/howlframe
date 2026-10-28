package bytecode

import (
	"bytes"
	"errors"
	"math"
	"testing"
)

// NUMERIC_CONTRACT section 2.3: no artifact loads a NaN or infinite constant.
// The compiler used to write +Inf for an out-of-range float literal, and a
// hand-made artifact could carry NaN.
func TestArtifactRefusesNonFiniteConstants(t *testing.T) {
	constant := func(value any) *BCProgram {
		return &BCProgram{Version: 1, Main: []BCInstruction{{OpString: "LOAD_CONST", Op: OpLoadConst, ValueOperand: value}}}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		var buf bytes.Buffer
		if err := WriteArtifact(&buf, constant(value)); err != nil {
			t.Fatalf("WriteArtifact(%v): %v", value, err)
		}
		if _, err := ReadArtifact(&buf); !errors.Is(err, ErrInvalidProgram) {
			t.Errorf("ReadArtifact(LOAD_CONST %v) error = %v, want ErrInvalidProgram", value, err)
		}
		buf.Reset()
		if err := WriteValidatedArtifact(&buf, constant(value)); !errors.Is(err, ErrInvalidProgram) || buf.Len() != 0 {
			t.Errorf("WriteValidatedArtifact(LOAD_CONST %v) error = %v and %d bytes, want ErrInvalidProgram and none", value, err, buf.Len())
		}
	}
	for _, value := range []any{math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64, math.Copysign(0, -1), int64(math.MaxInt64), "NaN", "Inf"} {
		var buf bytes.Buffer
		if err := WriteValidatedArtifact(&buf, constant(value)); err != nil {
			t.Fatalf("WriteValidatedArtifact(LOAD_CONST %v): %v", value, err)
		}
		if _, err := ReadArtifact(&buf); err != nil {
			t.Errorf("ReadArtifact(LOAD_CONST %v): %v", value, err)
		}
	}
}
