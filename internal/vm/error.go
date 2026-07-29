package vm

import (
	"encoding/json"
	"fmt"
	"zero/internal/bytecode"
)

type VMError struct {
	Phase       string `json:"phase"`
	Code        string `json:"code"`
	Function    string `json:"function"`
	Instruction int    `json:"instruction"`
	Opcode      string `json:"opcode,omitempty"`
	Message     string `json:"message"`
}

func (e *VMError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func NewRuntimeError(code string, fn string, ip int, op bytecode.Opcode, msg string, args ...any) *VMError {
	opName := ""
	if spec, ok := bytecode.Registry[op]; ok {
		opName = spec.Name
	}
	return &VMError{
		Phase:       "runtime",
		Code:        code,
		Function:    fn,
		Instruction: ip,
		Opcode:      opName,
		Message:     fmt.Sprintf(msg, args...),
	}
}

type VMLimits struct {
	MaxInstructions int
	MaxMemoryBytes  int
	MaxCallDepth    int
}

var DefaultLimits = VMLimits{
	MaxInstructions: 100000,
	MaxMemoryBytes:  67108864,
	MaxCallDepth:    128,
}
