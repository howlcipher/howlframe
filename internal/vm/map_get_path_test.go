package vm

import (
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// A path read is nested map_get. No intermediate let, no new capability.
// A missing key is nil (is_nil true); print/to_string still render it as "".
// map_get of that nil sentinel is TYPE_ERROR.
const mapGetPathHitSource = `(cli_app
  (let (row (dict ("user" (dict ("name" "ada") ("city" (dict ("id" "pdx")))))))
    (do
      (print "two" (map_get (map_get row "user") "name"))
      (print "three" (map_get (map_get (map_get row "user") "city") "id"))
      (print "leaf-miss" (map_get (map_get row "user") "missing"))
    )))`

const mapGetPathHitWant = "two ada\nthree pdx\nleaf-miss \n"

const mapGetPathMissingIntermediateSource = `(cli_app
  (let (row (dict ("user" (dict ("name" "ada")))))
    (do
      (print "sentinel" (map_get row "missing"))
      (print "chained" (map_get (map_get row "missing") "name"))
    )))`

const mapGetPathNonDictSource = `(cli_app
  (let (row (dict ("user" "ada") ("profile" (dict ("name" "ada")))))
    (print "bad" (map_get (map_get row "user") "name"))))`

func TestChainedMapGetPath(t *testing.T) {
	t.Run("interpreter", func(t *testing.T) { assertInterpretStdout(t, mapGetPathHitSource, mapGetPathHitWant) })

	t.Run("bytecode", func(t *testing.T) {
		_, prog := parseAndCompile(t, mapGetPathHitSource)
		var named, value int
		for _, inst := range prog.Main {
			if inst.Op != bytecode.OpMapGet {
				continue
			}
			if inst.StringOperand == "" {
				value++
			} else {
				named++
			}
		}
		if named == 0 || value == 0 {
			t.Fatalf("MAP_GET named=%d value=%d, want both forms", named, value)
		}
		assertBytecodeStdout(t, mapGetPathHitSource, mapGetPathHitWant)
	})
}

func TestChainedMapGetMissingIntermediate(t *testing.T) {
	t.Run("interpreter", func(t *testing.T) {
		out := assertInterpretErrorContains(t, mapGetPathMissingIntermediateSource, "TYPE_ERROR", "map_get expected dict, got")
		if out != "sentinel \n" || strings.Contains(out, "chained") {
			t.Fatalf("stdout = %q, want the nil miss and no chained print", out)
		}
	})

	t.Run("bytecode", func(t *testing.T) {
		out := assertBytecodeTypeErrorContains(t, mapGetPathMissingIntermediateSource, "TYPE_ERROR", "map_get expected dict, got <nil>")
		if out != "sentinel \n" || strings.Contains(out, "chained") {
			t.Fatalf("stdout = %q, want the nil miss and no chained print", out)
		}
	})
}

func TestChainedMapGetNonDictIntermediate(t *testing.T) {
	t.Run("interpreter", func(t *testing.T) {
		out := assertInterpretErrorContains(t, mapGetPathNonDictSource, "TYPE_ERROR", "map_get expected dict, got string")
		if strings.Contains(out, "bad") {
			t.Fatalf("non-dict path printed %q", out)
		}
	})

	t.Run("bytecode", func(t *testing.T) {
		out := assertBytecodeTypeErrorContains(t, mapGetPathNonDictSource, "TYPE_ERROR", "map_get expected dict, got string")
		if strings.Contains(out, "bad") {
			t.Fatalf("non-dict path printed %q", out)
		}
	})
}

func TestChainedMapGetDeclaresNoCapability(t *testing.T) {
	if got, want := bytecode.Registry[bytecode.OpMapGet].Capability, bytecode.Registry[bytecode.OpMapKeys].Capability; got != want || got != capability.None {
		t.Fatalf("map_get capability = %q, map_keys capability = %q, want both empty", got, want)
	}
	if got := capability.ForConstruct("map_get"); got != capability.None || got != capability.ForConstruct("map_keys") {
		t.Fatalf("ForConstruct(map_get) = %q, map_keys = %q", got, capability.ForConstruct("map_keys"))
	}
	mapGet := bytecode.BCInstruction{Op: bytecode.OpMapGet, OpString: "MAP_GET", StringOperand: "row"}
	if code := capabilityDenialCode(mapGet, nil); code == "CAPABILITY_DENIED" {
		t.Fatal("named map_get was denied with an empty grant")
	}
	valueGet := bytecode.BCInstruction{Op: bytecode.OpMapGet, OpString: "MAP_GET"}
	if code := capabilityDenialCode(valueGet, nil); code == "CAPABILITY_DENIED" {
		t.Fatal("value map_get was denied with an empty grant")
	}
}
