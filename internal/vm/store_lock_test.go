package vm

import (
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// storeLockDeadline bounds every follow-up store request in the #190 tests. A
// leaked store.mu makes the follow-up block forever, so the test fails on this
// deadline instead of hanging the whole package.
const storeLockDeadline = 2 * time.Second

// runWithin runs fn on its own goroutine and returns the VMError it raised (or
// nil). It fails the test if fn does not finish within storeLockDeadline.
func runWithin(t *testing.T, what string, fn func()) *VMError {
	t.Helper()
	done := make(chan *VMError, 1)
	go func() { done <- captureVMError(fn) }()
	select {
	case failure := <-done:
		return failure
	case <-time.After(storeLockDeadline):
		t.Fatalf("%s did not finish within %s: store lock leaked by the earlier failure (#190)", what, storeLockDeadline)
		return nil
	}
}

func storeLockKeyInsts(op bytecode.Opcode, name, key string) []bytecode.BCInstruction {
	return []bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		storeInstruction(op, name, "store", ""),
	}
}

func storeLockPutInsts(key string, record map[string]any) []bytecode.BCInstruction {
	return []bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: record},
		storeInstruction(bytecode.OpStorePut, "STORE_PUT", "store", ""),
	}
}

// followUpStoreRequestsSucceed issues every store opcode against the handle
// "store" (each under the deadline) and checks the results.
func followUpStoreRequestsSucceed(t *testing.T, vm *BCVM) {
	t.Helper()
	if failure := runWithin(t, "STORE_PUT after the failure", func() {
		vm.run(storeLockPutInsts("after", map[string]any{"ok": true}), vm.env)
	}); failure != nil {
		t.Fatalf("STORE_PUT after the failure: %v", failure)
	}
	if failure := runWithin(t, "STORE_GET after the failure", func() {
		vm.run(storeLockKeyInsts(bytecode.OpStoreGet, "STORE_GET", "after"), vm.env)
	}); failure != nil {
		t.Fatalf("STORE_GET after the failure: %v", failure)
	}
	if got := vm.pop(bytecode.OpStoreGet); !reflect.DeepEqual(got, map[string]any{"ok": true}) {
		t.Fatalf("STORE_GET after the failure = %#v", got)
	}
	if failure := runWithin(t, "STORE_KEYS after the failure", func() {
		vm.run([]bytecode.BCInstruction{storeInstruction(bytecode.OpStoreKeys, "STORE_KEYS", "store", "")}, vm.env)
	}); failure != nil {
		t.Fatalf("STORE_KEYS after the failure: %v", failure)
	}
	vm.pop(bytecode.OpStoreKeys)
	if failure := runWithin(t, "STORE_DELETE after the failure", func() {
		vm.run(storeLockKeyInsts(bytecode.OpStoreDelete, "STORE_DELETE", "after"), vm.env)
	}); failure != nil {
		t.Fatalf("STORE_DELETE after the failure: %v", failure)
	}
}

// TestStoreLockReleasedAfterPersistFailure (#190): a real persistence failure
// in the middle of a write or delete (the target directory is missing, so the
// atomic replace fails while store.mu is held) must surface as
// STORE_PERSISTENCE_WRITE_FAILED and must not leak the lock: once the
// directory exists, later store requests on the same store succeed promptly.
func TestStoreLockReleasedAfterPersistFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		failOp func(vm *BCVM)
	}{
		{"put", func(vm *BCVM) { vm.run(storeLockPutInsts("k", map[string]any{"v": 1}), vm.env) }},
		{"delete", func(vm *BCVM) { vm.run(storeLockKeyInsts(bytecode.OpStoreDelete, "STORE_DELETE", "k"), vm.env) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "missing")
			vm := newStoreTestVM()
			vm.AllowedCaps = []capability.Capability{capability.Database, capability.Filesystem}
			vm.run(openStoreInsts("store", "file://"+filepath.Join(dir, "store.json")), vm.env)

			failure := runWithin(t, "failing "+tc.name, func() { tc.failOp(vm) })
			if failure == nil || failure.Code != "STORE_PERSISTENCE_WRITE_FAILED" {
				t.Fatalf("failing %s = %#v, want STORE_PERSISTENCE_WRITE_FAILED", tc.name, failure)
			}
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			followUpStoreRequestsSucceed(t, vm)
		})
	}
}

// TestStoreLockReleasedAfterPanicInCriticalSection (#190): an arbitrary panic
// while the lock is held (as a record-copy failure would raise) becomes a
// STORE_OPERATION_FAILED store error, which an HTTP handler answers with 500,
// and every later store request still succeeds promptly.
func TestStoreLockReleasedAfterPanicInCriticalSection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		op     bytecode.Opcode
		failOp func(vm *BCVM)
	}{
		{"put", bytecode.OpStorePut, func(vm *BCVM) { vm.run(storeLockPutInsts("k", map[string]any{"v": 1}), vm.env) }},
		{"delete", bytecode.OpStoreDelete, func(vm *BCVM) { vm.run(storeLockKeyInsts(bytecode.OpStoreDelete, "STORE_DELETE", "k"), vm.env) }},
		{"get", bytecode.OpStoreGet, func(vm *BCVM) { vm.run(storeLockKeyInsts(bytecode.OpStoreGet, "STORE_GET", "k"), vm.env) }},
		{"keys", bytecode.OpStoreKeys, func(vm *BCVM) {
			vm.run([]bytecode.BCInstruction{storeInstruction(bytecode.OpStoreKeys, "STORE_KEYS", "store", "")}, vm.env)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			vm := newStoreTestVM()
			vm.run(openStoreInsts("store", "memory://lock-"+tc.name), vm.env)
			vm.run(storeLockPutInsts("k", map[string]any{"v": 0}), vm.env)

			target := vm.env.vars["store"].(*bcMemoryStore)
			var armed atomic.Bool
			armed.Store(true)
			hook := func(store *bcMemoryStore, op bytecode.Opcode) {
				if store == target && op == tc.op && armed.CompareAndSwap(true, false) {
					var record map[string]any
					record["copy"] = "fails" // nil-map write: a runtime panic mid-operation
				}
			}
			storeCriticalSectionHook.Store(&hook)
			t.Cleanup(func() { storeCriticalSectionHook.Store(nil) })

			failure := runWithin(t, "panicking "+tc.name, func() { tc.failOp(vm) })
			if failure == nil || failure.Code != "STORE_OPERATION_FAILED" {
				t.Fatalf("panicking %s = %#v, want STORE_OPERATION_FAILED", tc.name, failure)
			}
			storeCriticalSectionHook.Store(nil)
			followUpStoreRequestsSucceed(t, vm)
		})
	}
}
