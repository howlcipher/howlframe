package vm

import (
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// map_keys is the dict counterpart of store_keys. HowlBoard had to reshape
// object APIs into lists because a HowlFrame client could not enumerate the
// keys of a record it received. Order is sorted because Go map iteration is
// not, and the operation is pure: both runs pass a nil grant. Growing
// OpMapKeys's capability must fail this test.
func TestMapKeysSortedWithoutCapabilityGrant(t *testing.T) {
	const source = `(cli_app
  (let (counts (dict ("beta" "2") ("alpha" "1") ("gamma" "3")))
    (do
      (print (str_join (map_keys counts) ","))
      (print (list_len (map_keys (dict))))
      (for name (map_keys counts) (print name)))))`
	const want = "alpha,beta,gamma\n0\nalpha\nbeta\ngamma\n"

	t.Run("interpreter", func(t *testing.T) { assertInterpretStdout(t, source, want) })
	t.Run("bytecode", func(t *testing.T) { assertBytecodeStdout(t, source, want) })
}

// TestMapKeysDeclaresNoCapability fails if OpMapKeys grows a capability,
// including a change that also updates the empty-grant run above.
func TestMapKeysDeclaresNoCapability(t *testing.T) {
	if got := bytecode.Registry[bytecode.OpMapKeys].Capability; got != capability.None {
		t.Fatalf("OpMapKeys capability = %q, want none", got)
	}
	if got := capability.ForConstruct("map_keys"); got != capability.None {
		t.Fatalf("ForConstruct(map_keys) = %q, want none", got)
	}
}

func TestMapKeysRejectsNonDict(t *testing.T) {
	const source = `(cli_app (print (map_keys (list "a"))))`

	t.Run("interpreter", func(t *testing.T) { assertInterpretErrorContains(t, source, "", "map_keys expected dict") })
	t.Run("bytecode", func(t *testing.T) { assertBytecodeTypeErrorContains(t, source, "TYPE_ERROR", "map_keys expected dict") })
}

func TestMapKeysEmptyDictIsEmptyList(t *testing.T) {
	vm := newStoreTestVM()
	vm.run([]bytecode.BCInstruction{
		{Op: bytecode.OpMakeDict, OpString: "MAKE_DICT", IntOperand: 0},
		{Op: bytecode.OpMapKeys, OpString: "MAP_KEYS"},
	}, vm.env)
	got, ok := vm.pop(bytecode.OpMapKeys).([]any)
	if !ok || got == nil || len(got) != 0 {
		t.Fatalf("empty dict keys = %#v, want non-nil empty list", got)
	}
}
