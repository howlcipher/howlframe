package vm

import (
	"bytes"
	"strings"
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

	t.Run("interpreter", func(t *testing.T) {
		node, _ := parseAndCompile(t, source)
		var out, errOut bytes.Buffer
		if exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != want {
			t.Fatalf("stdout = %q, want %q", out.String(), want)
		}
	})

	t.Run("bytecode", func(t *testing.T) {
		_, prog := parseAndCompile(t, source)
		var out, errOut bytes.Buffer
		if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != want {
			t.Fatalf("stdout = %q, want %q", out.String(), want)
		}
	})
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

	t.Run("interpreter", func(t *testing.T) {
		node, _ := parseAndCompile(t, source)
		var out, errOut bytes.Buffer
		exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut)
		if exitCode == 0 || !strings.Contains(errOut.String(), "map_keys expected dict") {
			t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, out.String(), errOut.String())
		}
	})

	t.Run("bytecode", func(t *testing.T) {
		_, prog := parseAndCompile(t, source)
		var out, errOut bytes.Buffer
		evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errOut, 0)
		if evidence.RuntimeFailure == nil || evidence.RuntimeFailure.Code != "TYPE_ERROR" {
			t.Fatalf("failure = %#v, want TYPE_ERROR", evidence.RuntimeFailure)
		}
		if !strings.Contains(evidence.RuntimeFailure.Message, "map_keys expected dict") {
			t.Fatalf("message = %q", evidence.RuntimeFailure.Message)
		}
	})
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
