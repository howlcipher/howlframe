package vm

import (
	"reflect"
	"testing"
	"zero/internal/bytecode"
)

func TestBytecodeStoreLifecycleAndNamedAttachment(t *testing.T) {
	vm := newStoreTestVM()
	vm.run([]bytecode.BCInstruction{
		storeInstruction(bytecode.OpStoreOpen, "STORE_OPEN", "primary", "memory://session"),
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "title"},
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "Add login"},
		{Op: bytecode.OpMakeDict, OpString: "MAKE_DICT", IntOperand: 1},
		storeInstruction(bytecode.OpStorePut, "STORE_PUT", "primary", ""),
		storeInstruction(bytecode.OpStoreOpen, "STORE_OPEN", "attached", "memory://session"),
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "attached", ""),
	}, vm.env)

	got := vm.pop(bytecode.OpStoreGet)
	want := map[string]any{"title": "Add login"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("STORE_GET result = %#v, want %#v", got, want)
	}

	vm.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreDelete, "STORE_DELETE", "primary", ""),
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreDelete, "STORE_DELETE", "primary", ""),
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "primary", ""),
	}, vm.env)

	// Missing keys deterministically push nil.
	if got := vm.pop(bytecode.OpStoreGet); got != nil {
		t.Fatalf("missing STORE_GET result = %#v, want nil", got)
	}
}

func TestBytecodeStoresAreIsolatedPerVM(t *testing.T) {
	first := newStoreTestVM()
	second := newStoreTestVM()

	putStoreRecord(first, "first", "memory://session", "task:1", map[string]any{"status": "open"})
	second.run([]bytecode.BCInstruction{
		storeInstruction(bytecode.OpStoreOpen, "STORE_OPEN", "second", "memory://session"),
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "second", ""),
	}, second.env)

	if got := second.pop(bytecode.OpStoreGet); got != nil {
		t.Fatalf("second VM observed first VM record: %#v", got)
	}
}

func TestBytecodeStoreCopiesRecords(t *testing.T) {
	vm := newStoreTestVM()
	record := map[string]any{
		"tags": []any{"compiler"},
		"meta": map[string]any{"status": "open"},
	}
	putStoreRecord(vm, "store", "memory://session", "task:1", record)

	record["tags"].([]any)[0] = "mutated"
	record["meta"].(map[string]any)["status"] = "closed"

	got := getStoreRecord(vm, "store", "task:1")
	got["tags"].([]any)[0] = "changed after get"
	got["meta"].(map[string]any)["status"] = "changed after get"

	again := getStoreRecord(vm, "store", "task:1")
	want := map[string]any{
		"tags": []any{"compiler"},
		"meta": map[string]any{"status": "open"},
	}
	if !reflect.DeepEqual(again, want) {
		t.Fatalf("stored record changed through aliasing: got %#v, want %#v", again, want)
	}
}

func newStoreTestVM() *BCVM {
	return &BCVM{
		env:         NewBcEnv(nil),
		Limits:      DefaultLimits,
		stores:      newBCStoreRegistry(),
		AllowedCaps: []bytecode.Capability{bytecode.CapDatabase},
	}
}

func storeInstruction(op bytecode.Opcode, name string, handle string, uri string) bytecode.BCInstruction {
	return bytecode.BCInstruction{
		Op:             op,
		OpString:       name,
		StringOperand:  handle,
		StringOperand2: uri,
	}
}

func putStoreRecord(vm *BCVM, handle string, uri string, key string, record map[string]any) {
	vm.run([]bytecode.BCInstruction{
		storeInstruction(bytecode.OpStoreOpen, "STORE_OPEN", handle, uri),
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: record},
		storeInstruction(bytecode.OpStorePut, "STORE_PUT", handle, ""),
	}, vm.env)
}

func getStoreRecord(vm *BCVM, handle string, key string) map[string]any {
	vm.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", handle, ""),
	}, vm.env)
	return vm.pop(bytecode.OpStoreGet).(map[string]any)
}
