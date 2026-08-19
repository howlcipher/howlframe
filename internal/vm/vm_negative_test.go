package vm

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

func TestVMNegativeStackUnderflowHandling(t *testing.T) {
	// Program with OpBinop but empty stack - must panic recoverably or error cleanly
	prog := &bytecode.BCProgram{
		Main: []bytecode.BCInstruction{
			{Op: bytecode.OpBinop, StringOperand: "+"},
		},
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected VM to panic/recover on stack underflow, but it completed silently")
		}
	}()

	env := NewBcEnv(nil)
	vm := &BCVM{
		prog:        prog,
		env:         env,
		insts:       prog.Main,
		stores:      newBCStoreRegistry(),
		Limits:      DefaultLimits,
		AllowedCaps: []capability.Capability{capability.Network},
	}
	vm.run(vm.insts, env)
}

func TestVMNegativeUndefinedVariable(t *testing.T) {
	prog := &bytecode.BCProgram{
		Main: []bytecode.BCInstruction{
			{Op: bytecode.OpLoadVar, StringOperand: "non_existent_variable_xyz"},
		},
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("Expected VM to panic/recover on undefined variable, but it completed silently")
		}
	}()

	env := NewBcEnv(nil)
	vm := &BCVM{
		prog:        prog,
		env:         env,
		insts:       prog.Main,
		stores:      newBCStoreRegistry(),
		Limits:      DefaultLimits,
		AllowedCaps: []capability.Capability{capability.Network},
	}
	vm.run(vm.insts, env)
}
