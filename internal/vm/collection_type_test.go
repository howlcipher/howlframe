package vm

import (
	"bytes"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// collectionHappy is the #103 miss sentinel plus the dict and list ops that
// used to disagree once a record held more than one kind of value.
const collectionHappy = `(cli_app
  (let (row (dict ("s" "ada") ("xs" (list "a" "b")) ("d" (dict ("k" "v"))) ("empty" "")))
    (let (xs (map_get row "xs"))
      (let (d (map_get row "d"))
        (do
          (print "miss" (map_get row "missing"))
          (print "empty" (map_get row "empty"))
          (print "hit" (map_get d "k"))
          (print "leaf-miss" (map_get d "missing"))
          (print "get0" (list_get xs 0))
          (print "oob" (list_get xs 9))
          (print "len" (list_len xs))
          (append xs "c")
          (print "len2" (list_len xs))
          (map_set d "k2" "v2")
          (print "set" (map_get d "k2"))
          (map_delete d "k")
          (print "del" (map_get d "k"))
        )))))`

const collectionHappyWant = "miss \n" +
	"empty \n" +
	"hit v\n" +
	"leaf-miss \n" +
	"get0 a\n" +
	"oob \n" +
	"len 2\n" +
	"len2 3\n" +
	"set v2\n" +
	"del \n"

func TestCollectionOpsAgreeOnMissAndMutation(t *testing.T) {
	t.Run("interpreter", func(t *testing.T) {
		node, _ := parseAndCompile(t, collectionHappy)
		var out, errOut bytes.Buffer
		if exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != collectionHappyWant {
			t.Fatalf("stdout = %q, want %q", out.String(), collectionHappyWant)
		}
	})

	t.Run("bytecode", func(t *testing.T) {
		_, prog := parseAndCompile(t, collectionHappy)
		var out, errOut bytes.Buffer
		if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d, stderr = %s", exitCode, errOut.String())
		}
		if out.String() != collectionHappyWant {
			t.Fatalf("stdout = %q, want %q", out.String(), collectionHappyWant)
		}
	})
}

func TestCollectionOpsFailClosed(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{name: "append string", source: dynamicCollection(`(append s "z")`), want: "append expected list"},
		{name: "append dict", source: dynamicCollection(`(append d "z")`), want: "append expected list"},
		{name: "map_set string", source: dynamicCollection(`(map_set s "k" "v")`), want: "map_set expected dict"},
		{name: "map_set list", source: dynamicCollection(`(map_set xs "k" "v")`), want: "map_set expected dict"},
		{name: "map_delete string", source: dynamicCollection(`(map_delete s "k")`), want: "map_delete expected dict"},
		{name: "map_delete list", source: dynamicCollection(`(map_delete xs "k")`), want: "map_delete expected dict"},
		{name: "map_get string", source: dynamicCollection(`(print (map_get s "k"))`), want: "map_get expected dict"},
		{name: "map_get list", source: dynamicCollection(`(print (map_get xs "0"))`), want: "map_get expected dict"},
		{name: "list_get string", source: dynamicCollection(`(print (list_get s 0))`), want: "list_get expected list"},
		{name: "list_get dict", source: dynamicCollection(`(print (list_get d 0))`), want: "list_get expected list"},
		{name: "list_get bad index", source: dynamicCollection(`(print (list_get xs s))`), want: "list_get index must be a number"},
		{name: "list_len string", source: dynamicCollection(`(print (list_len s))`), want: "list_len expected list"},
		{name: "list_len dict", source: dynamicCollection(`(print (list_len d))`), want: "list_len expected list"},
		{name: "list_len string literal", source: `(cli_app (let (s "hello") (print (list_len s))))`, want: "list_len expected list"},
		{name: "list_len dict literal", source: `(cli_app (let (d (dict ("a" "b"))) (print (list_len d))))`, want: "list_len expected list"},
		{name: "map_keys string", source: dynamicCollection(`(print (map_keys s))`), want: "map_keys expected dict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("interpreter", func(t *testing.T) {
				node, _ := parseAndCompile(t, tc.source)
				var out, errOut bytes.Buffer
				exitCode := Interpret(node, nil, nil, strings.NewReader(""), &out, &errOut)
				if exitCode == 0 || !strings.Contains(errOut.String(), "TYPE_ERROR") || !strings.Contains(errOut.String(), tc.want) {
					t.Fatalf("exit = %d, stdout = %q, stderr = %q", exitCode, out.String(), errOut.String())
				}
				if strings.TrimSpace(out.String()) != "" {
					t.Fatalf("stdout = %q, want empty", out.String())
				}
			})
			t.Run("bytecode", func(t *testing.T) {
				_, prog := parseAndCompile(t, tc.source)
				var out, errOut bytes.Buffer
				evidence := RunBytecodeWithEvidence(prog, nil, DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errOut, 0)
				if evidence.RuntimeFailure == nil || evidence.RuntimeFailure.Code != "TYPE_ERROR" {
					t.Fatalf("failure = %#v, want TYPE_ERROR", evidence.RuntimeFailure)
				}
				if !strings.Contains(evidence.RuntimeFailure.Message, tc.want) {
					t.Fatalf("message = %q, want %q", evidence.RuntimeFailure.Message, tc.want)
				}
				if strings.TrimSpace(out.String()) != "" {
					t.Fatalf("stdout = %q, want empty", out.String())
				}
			})
		})
	}
}

func TestCollectionOpsGrantNothing(t *testing.T) {
	ops := []bytecode.Opcode{
		bytecode.OpAppend,
		bytecode.OpMapSet,
		bytecode.OpMapDelete,
		bytecode.OpMapGet,
		bytecode.OpListGet,
		bytecode.OpListLen,
		bytecode.OpMapKeys,
	}
	names := []string{"append", "map_set", "map_delete", "map_get", "list_get", "list_len", "map_keys"}
	for i, op := range ops {
		if got := bytecode.Registry[op].Capability; got != capability.None {
			t.Fatalf("%s capability = %q, want none", bytecode.Registry[op].Name, got)
		}
		if capability.ForConstruct(names[i]) != capability.None {
			t.Fatalf("%s construct declares a capability", names[i])
		}
	}
	_, prog := parseAndCompile(t, collectionHappy)
	var out, errOut bytes.Buffer
	if exitCode := RunBytecode(prog, nil, nil, strings.NewReader(""), &out, &errOut); exitCode != 0 {
		t.Fatalf("empty grant rejected a pure collection program: exit = %d, stderr = %s", exitCode, errOut.String())
	}
}

func dynamicCollection(body string) string {
	return `(cli_app
  (let (row (dict ("s" "ada") ("xs" (list "a" "b")) ("d" (dict ("k" "v")))))
    (let (s (map_get row "s"))
      (let (xs (map_get row "xs"))
        (let (d (map_get row "d"))
          ` + body + `)))))`
}
