package wasm

import "testing"

func TestValidateWATAcceptsGeneratedShape(t *testing.T) {
	source := `(module
  (func (export "main") (result i64)
    (i64.const 42)
  )
)`
	if err := ValidateWAT(source); err != nil {
		t.Fatalf("expected valid WAT shape: %v", err)
	}
}

func TestValidateWATRejectsMalformedShape(t *testing.T) {
	for _, source := range []string{
		"(func (result i64) (i64.const 1))",
		"(module (func (result i64) (i64.const 1))",
		`(module (func (export "main") (result i64) (i64.const "unterminated)))`,
	} {
		if err := ValidateWAT(source); err == nil {
			t.Errorf("expected malformed WAT to fail: %q", source)
		}
	}
}

func TestValidateWATRejectsInstructionTypeErrors(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "function result",
			source: `(module (func (export "main") (result i64) (i32.const 1)))`,
		},
		{
			name:   "binary operand",
			source: `(module (func (export "main") (result i64) (i64.add (i32.const 1) (i64.const 2))))`,
		},
		{
			name:   "if branch",
			source: `(module (func (export "main") (result i64) (if (result i64) (i32.const 1) (then (i64.const 1)) (else (f64.const 2)))))`,
		},
		{
			name:   "load without memory",
			source: `(module (func (export "main") (result i64) (i64.load (i32.const 0))))`,
		},
		{
			name:   "unknown instruction",
			source: `(module (func (export "main") (result i64) (i64.magic (i64.const 1))))`,
		},
		{
			name:   "invalid constant",
			source: `(module (func (export "main") (result i64) (i64.const nope)))`,
		},
		{
			name:   "data exceeds memory",
			source: `(module (memory 1) (data (i32.const 65535) "\00\00") (func (result i32) (i32.const 0)))`,
		},
		{
			name:   "i32 store value type",
			source: `(module (memory 1) (func (result i32) (block (result i32) (i32.store (i32.const 0) (i64.const 1)) (i32.const 0))))`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateWAT(test.source); err == nil {
				t.Fatalf("expected invalid WAT to fail: %s", test.source)
			}
		})
	}
}

func TestValidateWATAcceptsTypedMemoryAndControlFlow(t *testing.T) {
	source := `(module
		(memory (export "memory") 1)
		(data (i32.const 0) "\01\00")
		(func (export "main") (result i64)
			(if (result i64)
				(i64.lt_u (i64.const 0) (i64.load (i32.const 0)))
				(then (i64.const 1))
				(else (i64.const 0)))))`
	if err := ValidateWAT(source); err != nil {
		t.Fatalf("expected typed WAT to pass: %v", err)
	}
}

func TestValidateWATAcceptsI32ComparisonsEmittedForBooleans(t *testing.T) {
	source := `(module
		(func (export "main") (result i32)
			(i32.eq (i32.const 1) (i32.const 0))))`
	if err := ValidateWAT(source); err != nil {
		t.Fatalf("expected i32 comparison to pass: %v", err)
	}
}

func TestValidateWATAcceptsI32StoresForPointerTables(t *testing.T) {
	source := `(module
		(memory (export "memory") 1)
		(func (export "main") (result i32)
			(block (result i32)
				(i32.store (i32.const 8) (i32.const 16))
				(i32.load (i32.const 8)))))`
	if err := ValidateWAT(source); err != nil {
		t.Fatalf("expected i32 store to pass: %v", err)
	}
}
