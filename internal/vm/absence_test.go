package vm

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// Absence contract: a missing map_get key is "" and a missing store_get
// record is nil. A present empty string stays a string, so is_nil on the
// store result is what separates a blank field from a missing record.
const absenceMapGetSource = `(cli_app
  (let (strings (dict ("hit" "yes") ("empty" "")))
    (let (records (dict ("hit" "yes") ("empty" "") ("n" 7) ("nested" (dict ("k" "v")))))
      (let (nested (map_get records "nested"))
        (do
          (print "s-hit" (map_get strings "hit"))
          (print "s-empty" (map_get strings "empty"))
          (print "s-miss" (map_get strings "missing"))
          (print "s-empty-nil" (is_nil (map_get strings "empty")))
          (print "s-miss-nil" (is_nil (map_get strings "missing")))
          (print "a-hit" (map_get records "hit"))
          (print "a-n" (map_get records "n"))
          (print "a-empty" (map_get records "empty"))
          (print "a-miss" (map_get records "missing"))
          (print "a-empty-nil" (is_nil (map_get records "empty")))
          (print "a-miss-nil" (is_nil (map_get records "missing")))
          (print "nested" (map_get nested "k"))
          (print "nested-miss" (map_get nested "missing"))
          (print "nested-miss-nil" (is_nil (map_get nested "missing")))
        )))))`

const absenceMapGetWant = "s-hit yes\n" +
	"s-empty \n" +
	"s-miss \n" +
	"s-empty-nil false\n" +
	"s-miss-nil true\n" +
	"a-hit yes\n" +
	"a-n 7\n" +
	"a-empty \n" +
	"a-miss \n" +
	"a-empty-nil false\n" +
	"a-miss-nil true\n" +
	"nested v\n" +
	"nested-miss \n" +
	"nested-miss-nil true\n"

const absenceStoreSource = `(cli_app
  (let (strings (dict ("empty" "")))
    (do
      (store_open db "memory://absence")
      (store_put db "blank" (dict ("note" "")))
      (print "field-nil" (is_nil (map_get strings "empty")))
      (print "record-nil" (is_nil (store_get db "blank")))
      (print "gone-nil" (is_nil (store_get db "gone")))
      (print "gone" (store_get db "gone"))
    )))`

const absenceStoreWant = "field-nil false\n" +
	"record-nil false\n" +
	"gone-nil true\n" +
	"gone \n"

func TestMapGetMissIsNil(t *testing.T) {
	t.Run("interpreter", func(t *testing.T) { assertInterpretStdout(t, absenceMapGetSource, absenceMapGetWant) })
	t.Run("bytecode", func(t *testing.T) { assertBytecodeStdout(t, absenceMapGetSource, absenceMapGetWant) })
}

func TestStoreGetMissIsNil(t *testing.T) {
	assertRuntimeResult(t, absenceStoreSource, []capability.Capability{capability.Database}, "", absenceStoreWant, "", 0, true)
	// nil renders as "" in print (2026-10-12); absence is observed with is_nil.
}

func TestAbsenceOpcodeValuesAndCapabilities(t *testing.T) {
	for _, spec := range []struct {
		op        bytecode.Opcode
		construct string
		want      capability.Capability
	}{
		{bytecode.OpMapGet, "map_get", capability.None},
		{bytecode.OpStoreGet, "store_get", capability.Database},
		{bytecode.OpStoreKeys, "store_keys", capability.Database},
	} {
		if got := bytecode.Registry[spec.op].Capability; got != spec.want {
			t.Fatalf("%s capability = %q, want %q", spec.construct, got, spec.want)
		}
		if got := capability.ForConstruct(spec.construct); got != spec.want {
			t.Fatalf("ForConstruct(%s) = %q, want %q", spec.construct, got, spec.want)
		}
	}

	mapGet := bytecode.BCInstruction{Op: bytecode.OpMapGet, OpString: "MAP_GET", StringOperand: "records"}
	if code := capabilityDenialCode(mapGet, nil); code == "CAPABILITY_DENIED" {
		t.Fatal("map_get was denied with an empty grant")
	}
	storeGet := bytecode.BCInstruction{Op: bytecode.OpStoreGet, OpString: "STORE_GET", StringOperand: "db"}
	if code := capabilityDenialCode(storeGet, nil); code != "CAPABILITY_DENIED" {
		t.Fatalf("store_get without database: code = %q, want CAPABILITY_DENIED", code)
	}

	vm := &BCVM{
		env:         NewBcEnv(nil),
		Limits:      DefaultLimits,
		stores:      newBCStoreRegistry(),
		AllowedCaps: nil,
	}
	vm.env.vars["strings"] = map[string]any{"hit": "yes", "empty": ""}
	vm.env.vars["records"] = map[string]any{
		"hit":    "yes",
		"empty":  "",
		"n":      int64(7),
		"nested": map[string]any{"k": "v"},
		"null":   nil,
	}

	if got := runMapOp(t, vm, "strings", "hit", false); got != "yes" {
		t.Fatalf("string hit = %#v", got)
	}
	if got := runMapOp(t, vm, "strings", "empty", false); got != "" {
		t.Fatalf("present empty string = %#v", got)
	}
	if got := runMapOp(t, vm, "strings", "missing", false); got != nil {
		t.Fatalf("string-map miss = %#v, want nil", got)
	}
	if got := runMapOp(t, vm, "records", "missing", false); got != nil {
		t.Fatalf("map[string]any miss = %#v, want nil", got)
	}
	if got := runMapOp(t, vm, "records", "n", false); got != int64(7) {
		t.Fatalf("non-string hit = %#v, want 7", got)
	}
	nested, ok := runMapOp(t, vm, "records", "nested", false).(map[string]any)
	if !ok || nested["k"] != "v" {
		t.Fatalf("nested hit = %#v", nested)
	}
	if got := runMapOp(t, vm, "records", "null", false); got != nil {
		t.Fatalf("present nil = %#v, want nil", got)
	}
	if runMapOp(t, vm, "strings", "empty", true).(bool) {
		t.Fatal("is_nil(present \"\") = true")
	}
	if !runMapOp(t, vm, "strings", "missing", true).(bool) {
		t.Fatal("is_nil(map_get miss) = false, want true")
	}
	if !runMapOp(t, vm, "records", "null", true).(bool) {
		t.Fatal("is_nil(present nil) = false")
	}

	store := newStoreTestVM()
	putStoreRecord(store, "db", "memory://absence", "blank", map[string]any{"note": ""})
	if got := runStoreLookup(store, "blank", true); got != false {
		t.Fatalf("is_nil(present record) = %#v, want false", got)
	}
	missing := runStoreLookup(store, "gone", false)
	if missing != nil {
		t.Fatalf("store_get miss = %#v, want nil", missing)
	}
	if _, isString := missing.(string); isString {
		t.Fatal("store_get miss is a string")
	}
	if got := runStoreLookup(store, "gone", true); got != true {
		t.Fatalf("is_nil(store_get miss) = %#v, want true", got)
	}
}

func runStoreLookup(store *BCVM, key string, checkNil bool) any {
	insts := []bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "db", ""),
	}
	resultOp := bytecode.OpStoreGet
	if checkNil {
		insts = append(insts, bytecode.BCInstruction{Op: bytecode.OpIsNil, OpString: "IS_NIL"})
		resultOp = bytecode.OpIsNil
	}
	store.run(insts, store.env)
	return store.pop(resultOp)
}

func runMapOp(t *testing.T, vm *BCVM, dict, key string, checkNil bool) any {
	t.Helper()
	insts := []bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		{Op: bytecode.OpMapGet, OpString: "MAP_GET", StringOperand: dict},
	}
	resultOp := bytecode.OpMapGet
	if checkNil {
		insts = append(insts, bytecode.BCInstruction{Op: bytecode.OpIsNil, OpString: "IS_NIL"})
		resultOp = bytecode.OpIsNil
	}
	vm.run(insts, vm.env)
	got := vm.pop(resultOp)
	if checkNil {
		flag, ok := got.(bool)
		if !ok {
			t.Fatalf("is_nil result = %#v, want bool", got)
		}
		return flag
	}
	return got
}
