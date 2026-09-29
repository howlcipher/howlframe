package vm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// A path read is nested map_get. No intermediate let, no new capability.
// A missing key is still "". map_get of that sentinel is TYPE_ERROR.
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
	t.Run("interpreter", func(t *testing.T) {
		node, _ := parseAndCompile(t, mapGetPathHitSource)
		var out, errOut bytes.Buffer
		if exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != mapGetPathHitWant {
			t.Fatalf("stdout = %q, want %q", out.String(), mapGetPathHitWant)
		}
	})

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
		var out, errOut bytes.Buffer
		if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != mapGetPathHitWant {
			t.Fatalf("stdout = %q, want %q", out.String(), mapGetPathHitWant)
		}
	})
}

func TestChainedMapGetMissingIntermediate(t *testing.T) {
	t.Run("interpreter", func(t *testing.T) {
		node, _ := parseAndCompile(t, mapGetPathMissingIntermediateSource)
		var out, errOut bytes.Buffer
		exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut)
		assertMapGetPathTypeError(t, exitCode, out.String(), errOut.String())
	})

	t.Run("bytecode", func(t *testing.T) {
		_, prog := parseAndCompile(t, mapGetPathMissingIntermediateSource)
		var out, errOut bytes.Buffer
		evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errOut, 0)
		if evidence.RuntimeFailure == nil || evidence.RuntimeFailure.Code != "TYPE_ERROR" {
			t.Fatalf("failure = %#v, want TYPE_ERROR", evidence.RuntimeFailure)
		}
		if !strings.Contains(evidence.RuntimeFailure.Message, "map_get expected dict, got string") {
			t.Fatalf("message = %q", evidence.RuntimeFailure.Message)
		}
		if out.String() != "sentinel \n" {
			t.Fatalf("stdout = %q, want the #103 empty-string sentinel and no chained print", out.String())
		}
		if strings.Contains(out.String(), "chained") {
			t.Fatalf("chained read of the sentinel printed %q", out.String())
		}
	})
}

func TestChainedMapGetNonDictIntermediate(t *testing.T) {
	t.Run("interpreter", func(t *testing.T) {
		node, _ := parseAndCompile(t, mapGetPathNonDictSource)
		var out, errOut bytes.Buffer
		exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut)
		if exitCode == 0 || !strings.Contains(errOut.String(), "TYPE_ERROR: map_get expected dict, got string") {
			t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, out.String(), errOut.String())
		}
		if strings.Contains(out.String(), "bad") {
			t.Fatalf("non-dict path printed %q", out.String())
		}
	})

	t.Run("bytecode", func(t *testing.T) {
		_, prog := parseAndCompile(t, mapGetPathNonDictSource)
		var out, errOut bytes.Buffer
		evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errOut, 0)
		if evidence.RuntimeFailure == nil || evidence.RuntimeFailure.Code != "TYPE_ERROR" {
			t.Fatalf("failure = %#v, want TYPE_ERROR", evidence.RuntimeFailure)
		}
		if !strings.Contains(evidence.RuntimeFailure.Message, "map_get expected dict, got string") {
			t.Fatalf("message = %q", evidence.RuntimeFailure.Message)
		}
		if strings.Contains(out.String(), "bad") {
			t.Fatalf("non-dict path printed %q", out.String())
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

func assertMapGetPathTypeError(t *testing.T, exitCode int, stdout, stderr string) {
	t.Helper()
	if exitCode == 0 || !strings.Contains(stderr, "TYPE_ERROR: map_get expected dict, got string") {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, stdout, stderr)
	}
	if stdout != "sentinel \n" {
		t.Fatalf("stdout = %q, want the #103 empty-string sentinel and no chained print", stdout)
	}
	if strings.Contains(stdout, "chained") {
		t.Fatalf("chained read of the sentinel printed %q", stdout)
	}
}
