package bytecode

import (
	"github.com/howlcipher/howlframe/internal/capability"
	"strings"
	"testing"
)

func TestOpcodeCapabilityDrift(t *testing.T) {
	// For every capability-bearing opcode, verify it matches the capability
	// inferred for the corresponding HowlFrame construct (lowercased name).
	for _, spec := range Registry {
		constructName := strings.ToLower(spec.Name)
		expectedCap := capability.ForConstruct(constructName)

		// Some opcodes do not correspond directly to 1:1 constructs with the exact same name,
		// or they might be aliases. But for those that are known to be 1:1 constructs:
		if expectedCap != capability.None {
			if spec.Capability != expectedCap {
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
			if spec.Capability != capability.None {
				t.Errorf("Opcode %s was expected to be capability-free, but got %q", spec.Name, spec.Capability)
			}
		}
	}
}

func TestOpcodeAchieveCapability(t *testing.T) {
	if Registry[OpAchieve].Capability != capability.Network {
		t.Errorf("OpAchieve capability = %v, want %v", Registry[OpAchieve].Capability, capability.Network)
	}
}
