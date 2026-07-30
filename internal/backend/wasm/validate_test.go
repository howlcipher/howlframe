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
