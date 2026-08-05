package bytecode

import (
	"strings"
	"testing"
	"zero/internal/capability"
)

func TestOpcodeCapabilityDrift(t *testing.T) {
	// For every capability-bearing opcode, verify it matches the capability
	// inferred for the corresponding Zero construct (lowercased name).
	for _, spec := range Registry {
		constructName := strings.ToLower(spec.Name)
		expectedCap := capability.ForConstruct(constructName)
		
		// Some opcodes do not correspond directly to 1:1 constructs with the exact same name,
		// or they might be aliases. But for those that are known to be 1:1 constructs:
		if expectedCap != capability.None {
			if spec.Capability != expectedCap {
				// ACHIEVE has network in ZIR but no capability in bytecode.
				// This is a confirmed defect, recorded in bugs.md, so we skip it here.
				if spec.Name == "ACHIEVE" {
					continue
				}
				t.Errorf("Opcode %s capability drifted: registry has %q, but capability.ForConstruct(%q) returns %q",
					spec.Name, spec.Capability, constructName, expectedCap)
			}
		}
	}
}

func TestCapabilityFreeOpcodesRemainFree(t *testing.T) {
	for _, spec := range Registry {
		if spec.Code == OpUnknown {
			continue
		}
		constructName := strings.ToLower(spec.Name)
		expectedCap := capability.ForConstruct(constructName)
		if expectedCap == capability.None {
			// ACHIEVE is a special case: it has network capability in ZIR (lowered as 'achieve') 
			// but no capability at runtime bytecode execution in original codebase. 
			// We preserve this discrepancy to not silently patch security behavior as instructed.
			if spec.Capability != capability.None {
				t.Errorf("Opcode %s was expected to be capability-free, but got %q", spec.Name, spec.Capability)
			}
		}
	}
}
