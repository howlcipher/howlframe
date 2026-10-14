package vm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
)

func decodeShiftedOpcodeProgram(t *testing.T, instructions []bytecode.BCInstruction) *bytecode.BCProgram {
	t.Helper()
	var artifact bytes.Buffer
	if err := bytecode.WriteArtifact(&artifact, &bytecode.BCProgram{Main: instructions}); err != nil {
		t.Fatal(err)
	}
	prog, err := bytecode.ReadArtifact(&artifact)
	if err != nil {
		t.Fatal(err)
	}
	return prog
}

func TestArtifactNamedConversionExecutesAfterEnumChanges(t *testing.T) {
	prog := decodeShiftedOpcodeProgram(t, []bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(65)},
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: float64(66)},
		{Op: bytecode.OpMakeList, OpString: "MAKE_LIST", IntOperand: 2},
		{Op: bytecode.OpCall, OpString: "CONVERT", StringOperand: "bytes_to_string"},
		{Op: bytecode.OpForNext, OpString: "PRINT", IntOperand: 1},
	})
	var out, errOut bytes.Buffer
	code := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut)
	if code != 0 || out.String() != "AB\n" || errOut.Len() != 0 {
		t.Fatalf("exit%d stdout%q stderr%q", code, out.String(), errOut.String())
	}
}

func TestArtifactNamedFilesystemOpcodeStillRequiresCapability(t *testing.T) {
	prog := decodeShiftedOpcodeProgram(t, []bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "not-accessible-without-grant"},
		{Op: bytecode.OpLoadConst, OpString: "READ_FILE"},
	})
	var out, errOut bytes.Buffer
	evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errOut, 0)
	if evidence.RuntimeFailure == nil || evidence.RuntimeFailure.Code != "CAPABILITY_DENIED" {
		t.Fatalf("named filesystem instruction bypassed authority: %+v", evidence)
	}
}
