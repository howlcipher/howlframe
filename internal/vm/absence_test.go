package vm

import (
	"bytes"
	"strings"
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
	"s-miss-nil false\n" +
	"a-hit yes\n" +
	"a-n 7\n" +
	"a-empty \n" +
	"a-miss \n" +
	"a-empty-nil false\n" +
	"a-miss-nil false\n" +
	"nested v\n" +
	"nested-miss \n" +
	"nested-miss-nil false\n"

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
	"gone <nil>\n"

func TestMapGetMissIsEmptyString(t *testing.T) {
	t.Run("interpreter", func(t *testing.T) {
		node, _ := parseAndCompile(t, absenceMapGetSource)
		var out, errOut bytes.Buffer
		if exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != absenceMapGetWant {
			t.Fatalf("stdout = %q, want %q", out.String(), absenceMapGetWant)
		}
	})

	t.Run("bytecode", func(t *testing.T) {
		_, prog := parseAndCompile(t, absenceMapGetSource)
		var out, errOut bytes.Buffer
		if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != absenceMapGetWant {
			t.Fatalf("stdout = %q, want %q", out.String(), absenceMapGetWant)
		}
	})
}

func TestStoreGetMissIsNil(t *testing.T) {
	_, prog := parseAndCompile(t, absenceStoreSource)
	var out, errOut bytes.Buffer
	exitCode := RunBytecode(prog, nil, []capability.Capability{capability.Database}, strings.NewReader(""), &out, &errOut)
	if exitCode != 0 {
		t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
	}
	if out.String() != absenceStoreWant {
		t.Fatalf("stdout = %q, want %q", out.String(), absenceStoreWant)
	}
	if strings.Contains(out.String(), "gone \n") || strings.Contains(out.String(), "gone \"\"") {
		t.Fatalf("missing store record printed as an empty string:\n%s", out.String())
	}
}

func TestAbsenceOpcodeValuesAndCapabilities(t *testing.T) {
	if got := bytecode.Registry[bytecode.OpMapGet].Capability; got != capability.None {
		t.Fatalf("map_get capability = %q, want none", got)
	}
	if got := bytecode.Registry[bytecode.OpStoreGet].Capability; got != capability.Database {
		t.Fatalf("store_get capability = %q, want database", got)
	}
	if got := bytecode.Registry[bytecode.OpStoreKeys].Capability; got != capability.Database {
		t.Fatalf("store_keys capability = %q, want database", got)
	}
	if got := capability.ForConstruct("map_get"); got != capability.None {
		t.Fatalf("ForConstruct(map_get) = %q, want none", got)
	}
	if got := capability.ForConstruct("store_get"); got != capability.Database {
		t.Fatalf("ForConstruct(store_get) = %q, want database", got)
	}
	if got := capability.ForConstruct("store_keys"); got != capability.Database {
		t.Fatalf("ForConstruct(store_keys) = %q, want database", got)
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

	if got := runMapGet(t, vm, "strings", "hit"); got != "yes" {
		t.Fatalf("string hit = %#v", got)
	}
	if got := runMapGet(t, vm, "strings", "empty"); got != "" {
		t.Fatalf("present empty string = %#v", got)
	}
	if got := runMapGet(t, vm, "strings", "missing"); got != "" {
		t.Fatalf("string-map miss = %#v, want \"\"", got)
	}
	if got := runMapGet(t, vm, "records", "missing"); got != "" {
		t.Fatalf("map[string]any miss = %#v, want \"\"", got)
	}
	if got := runMapGet(t, vm, "records", "n"); got != int64(7) {
		t.Fatalf("non-string hit = %#v, want 7", got)
	}
	nested, ok := runMapGet(t, vm, "records", "nested").(map[string]any)
	if !ok || nested["k"] != "v" {
		t.Fatalf("nested hit = %#v", nested)
	}
	if got := runMapGet(t, vm, "records", "null"); got != nil {
		t.Fatalf("present nil = %#v, want nil", got)
	}
	if got := runIsNil(t, vm, "strings", "empty"); got {
		t.Fatal("is_nil(present \"\") = true")
	}
	if got := runIsNil(t, vm, "strings", "missing"); got {
		t.Fatal("is_nil(map_get miss) = true, want false")
	}
	if got := runIsNil(t, vm, "records", "null"); !got {
		t.Fatal("is_nil(present nil) = false")
	}

	store := newStoreTestVM()
	putStoreRecord(store, "db", "memory://absence", "blank", map[string]any{"note": ""})
	store.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "blank"},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "db", ""),
		{Op: bytecode.OpIsNil, OpString: "IS_NIL"},
	}, store.env)
	if got := store.pop(bytecode.OpIsNil); got != false {
		t.Fatalf("is_nil(present record) = %#v, want false", got)
	}
	store.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "gone"},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "db", ""),
	}, store.env)
	missing := store.pop(bytecode.OpStoreGet)
	if missing != nil {
		t.Fatalf("store_get miss = %#v, want nil", missing)
	}
	if _, isString := missing.(string); isString {
		t.Fatal("store_get miss is a string")
	}
	store.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: "gone"},
		storeInstruction(bytecode.OpStoreGet, "STORE_GET", "db", ""),
		{Op: bytecode.OpIsNil, OpString: "IS_NIL"},
	}, store.env)
	if got := store.pop(bytecode.OpIsNil); got != true {
		t.Fatalf("is_nil(store_get miss) = %#v, want true", got)
	}
}

func runMapGet(t *testing.T, vm *BCVM, dict string, key string) any {
	t.Helper()
	vm.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		{Op: bytecode.OpMapGet, OpString: "MAP_GET", StringOperand: dict},
	}, vm.env)
	return vm.pop(bytecode.OpMapGet)
}

func runIsNil(t *testing.T, vm *BCVM, dict string, key string) bool {
	t.Helper()
	vm.run([]bytecode.BCInstruction{
		{Op: bytecode.OpLoadConst, OpString: "LOAD_CONST", ValueOperand: key},
		{Op: bytecode.OpMapGet, OpString: "MAP_GET", StringOperand: dict},
		{Op: bytecode.OpIsNil, OpString: "IS_NIL"},
	}, vm.env)
	got := vm.pop(bytecode.OpIsNil)
	flag, ok := got.(bool)
	if !ok {
		t.Fatalf("is_nil result = %#v, want bool", got)
	}
	return flag
}
