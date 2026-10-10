package vm

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

func TestBytecodeStoreLifecycleAndNamedAttachment(t *testing.T) {
	vm := newStoreTestVM()
	insts := openStoreInsts("primary", "memory://session")
	insts = append(insts,
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "title"},
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "Add login"},
		bytecode.BCInstruction{Op: bytecode.OpMakeDict, OpString: "MAKE_DICT", IntOperand: 1},
		storeInstruction(bytecode.OpStorePut, "STORE_PUT", "primary", ""),
	)
	insts = append(insts, openStoreInsts("attached", "memory://session")...)
	insts = append(insts,
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "attached", ""),
	)
	vm.run(insts, vm.env)

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
	insts := openStoreInsts("second", "memory://session")
	insts = append(insts,
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "second", ""),
	)
	second.run(insts, second.env)

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
		AllowedCaps: []capability.Capability{capability.Database},
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

func openStoreInsts(handle, uri string) []bytecode.BCInstruction {
	return []bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: uri},
		storeInstruction(bytecode.OpStoreOpen, "STORE_OPEN", handle, uri),
	}
}

func putStoreRecord(vm *BCVM, handle string, uri string, key string, record map[string]any) {
	insts := openStoreInsts(handle, uri)
	insts = append(insts,
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		bytecode.BCInstruction{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: record},
		storeInstruction(bytecode.OpStorePut, "STORE_PUT", handle, ""),
	)
	vm.run(insts, vm.env)
}

func getStoreRecord(vm *BCVM, handle string, key string) map[string]any {
	vm.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", handle, ""),
	}, vm.env)
	return vm.pop(bytecode.OpStoreGet).(map[string]any)
}

// store_keys is the enumeration primitive every prior HowlFrame application had
// to fake with a hand-maintained index record. Order must be sorted: Go
// randomizes map iteration, and callers list records for display.
func TestBytecodeStoreKeysSortedAndDeterministic(t *testing.T) {
	vm := newStoreTestVM()
	for _, key := range []string{"mission:c", "_seq", "mission:a", "mission:b"} {
		putStoreRecord(vm, "kv", "memory://session", key, map[string]any{"n": key})
	}

	want := []any{"_seq", "mission:a", "mission:b", "mission:c"}
	for attempt := 0; attempt < 8; attempt++ {
		vm.run([]bytecode.BCInstruction{
			storeInstruction(bytecode.OpStoreKeys, "STORE_KEYS", "kv", ""),
		}, vm.env)
		got := vm.pop(bytecode.OpStoreKeys)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("attempt %d: STORE_KEYS = %#v, want %#v", attempt, got, want)
		}
	}
}

func TestBytecodeStoreKeysReflectsDeletesAndEmptyStore(t *testing.T) {
	vm := newStoreTestVM()
	insts := openStoreInsts("kv", "memory://session")
	insts = append(insts, storeInstruction(bytecode.OpStoreKeys, "STORE_KEYS", "kv", ""))
	vm.run(insts, vm.env)
	if got := vm.pop(bytecode.OpStoreKeys); !reflect.DeepEqual(got, []any{}) {
		t.Fatalf("empty store STORE_KEYS = %#v, want empty list", got)
	}

	putStoreRecord(vm, "kv", "memory://session", "task:1", map[string]any{"status": "open"})
	putStoreRecord(vm, "kv", "memory://session", "task:2", map[string]any{"status": "open"})
	vm.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "task:1"},
		storeInstruction(bytecode.OpStoreDelete, "STORE_DELETE", "kv", ""),
		storeInstruction(bytecode.OpStoreKeys, "STORE_KEYS", "kv", ""),
	}, vm.env)
	if got := vm.pop(bytecode.OpStoreKeys); !reflect.DeepEqual(got, []any{"task:2"}) {
		t.Fatalf("post-delete STORE_KEYS = %#v, want [task:2]", got)
	}
}

func TestBytecodeStoreKeysRequiresDatabaseCapability(t *testing.T) {
	if got := bytecode.Registry[bytecode.OpStoreKeys].Capability; got != capability.Database {
		t.Fatalf("OpStoreKeys capability = %q, want database", got)
	}
	if got := capability.ForConstruct("store_keys"); got != capability.Database {
		t.Fatalf("ForConstruct(store_keys) = %q, want database", got)
	}

	vm := newStoreTestVM()
	putStoreRecord(vm, "kv", "memory://session", "task:1", map[string]any{"status": "open"})
	vm.AllowedCaps = []capability.Capability{capability.Filesystem}

	failure := captureVMError(func() {
		vm.run([]bytecode.BCInstruction{
			storeInstruction(bytecode.OpStoreKeys, "STORE_KEYS", "kv", ""),
		}, vm.env)
	})

	if failure == nil || failure.Code != "CAPABILITY_DENIED" || !strings.Contains(failure.Message, "database") {
		t.Fatalf("STORE_KEYS without database capability = %#v, want CAPABILITY_DENIED database", failure)
	}
}

// TestFileStoreKeysDeniedWithDatabaseAlone fails if a file:// enumeration
// succeeds without filesystem. The store is already open, so the denial is
// the STORE_KEYS filesystem check and not only STORE_OPEN.
func TestFileStoreKeysDeniedWithDatabaseAlone(t *testing.T) {
	uri := fileStoreURI(t, "keys.json")
	vm := newStoreTestVM()
	vm.AllowedCaps = []capability.Capability{capability.Database, capability.Filesystem}
	putStoreRecord(vm, "kv", uri, "task:1", map[string]any{"status": "open"})

	path := strings.TrimPrefix(uri, "file://")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	vm.AllowedCaps = []capability.Capability{capability.Database}
	failure := captureVMError(func() {
		vm.run([]bytecode.BCInstruction{
			storeInstruction(bytecode.OpStoreKeys, "STORE_KEYS", "kv", ""),
		}, vm.env)
	})
	if failure == nil || failure.Code != "CAPABILITY_DENIED" || !strings.Contains(failure.Message, "filesystem") {
		t.Fatalf("file:// STORE_KEYS with database alone = %#v, want CAPABILITY_DENIED filesystem", failure)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("denied file:// STORE_KEYS changed %s", path)
	}

	reader := newStoreTestVM()
	reader.AllowedCaps = []capability.Capability{capability.Database, capability.Filesystem}
	insts := openStoreInsts("kv", uri)
	insts = append(insts, storeInstruction(bytecode.OpStoreKeys, "STORE_KEYS", "kv", ""))
	reader.run(insts, reader.env)
	if got := reader.pop(bytecode.OpStoreKeys); !reflect.DeepEqual(got, []any{"task:1"}) {
		t.Fatalf("file:// STORE_KEYS with database and filesystem = %#v, want [task:1]", got)
	}
}
