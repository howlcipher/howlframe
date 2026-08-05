package vm

import (
	"testing"
	"zero/internal/bytecode"
	"zero/internal/capability"
)

// capabilityDenialCode runs a single instruction under the given allow-list
// and reports the VMError code it panicked with, or "" if it didn't panic
// with a VMError at all. The capability gate runs before an opcode's own
// logic, so a bare instruction with no operands is enough to prove whether
// the gate let it through: a denied instruction always panics with
// CAPABILITY_DENIED before touching the stack, while an allowed instruction
// either succeeds or fails for an unrelated (non-capability) reason.
func capabilityDenialCode(inst bytecode.BCInstruction, allowedCaps []capability.Capability) (code string) {
	vm := &BCVM{
		env:         NewBcEnv(nil),
		Limits:      DefaultLimits,
		stores:      newBCStoreRegistry(),
		AllowedCaps: allowedCaps,
	}
	defer func() {
		if r := recover(); r != nil {
			if vmerr, ok := r.(*VMError); ok {
				code = vmerr.Code
			}
		}
	}()
	vm.run([]bytecode.BCInstruction{inst}, vm.env)
	return ""
}

func TestCapabilityGatePerKind(t *testing.T) {
	cases := []struct {
		name string
		cap  capability.Capability
		inst bytecode.BCInstruction
	}{
		{"network", capability.Network, bytecode.BCInstruction{Op: bytecode.OpFetch, OpString: "FETCH"}},
		{"network_achieve", capability.Network, bytecode.BCInstruction{Op: bytecode.OpAchieve, OpString: "ACHIEVE"}},
		{"filesystem", capability.Filesystem, bytecode.BCInstruction{Op: bytecode.OpReadFile, OpString: "READ_FILE"}},
		{"process", capability.Process, bytecode.BCInstruction{Op: bytecode.OpExec, OpString: "EXEC"}},
		{"environment", capability.Environment, bytecode.BCInstruction{Op: bytecode.OpEnv, OpString: "ENV"}},
		{"database", capability.Database, bytecode.BCInstruction{Op: bytecode.OpDbConnect, OpString: "DB_CONNECT"}},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/denied", func(t *testing.T) {
			if code := capabilityDenialCode(tc.inst, nil); code != "CAPABILITY_DENIED" {
				t.Fatalf("code = %q, want CAPABILITY_DENIED", code)
			}
		})

		t.Run(tc.name+"/deniedByOtherCaps", func(t *testing.T) {
			var otherCaps []capability.Capability
			for _, other := range cases {
				if other.cap != tc.cap {
					otherCaps = append(otherCaps, other.cap)
				}
			}
			if code := capabilityDenialCode(tc.inst, otherCaps); code != "CAPABILITY_DENIED" {
				t.Fatalf("code = %q, want CAPABILITY_DENIED", code)
			}
		})

		t.Run(tc.name+"/allowed", func(t *testing.T) {
			if code := capabilityDenialCode(tc.inst, []capability.Capability{tc.cap}); code == "CAPABILITY_DENIED" {
				t.Fatalf("instruction denied even though %s was allowed", tc.cap)
			}
		})
	}
}

func TestCapabilityGateIgnoresCapNoneInstructions(t *testing.T) {
	inst := bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "hi"}
	if code := capabilityDenialCode(inst, nil); code == "CAPABILITY_DENIED" {
		t.Fatalf("CapNone instruction was denied with an empty allow-list")
	}
}
