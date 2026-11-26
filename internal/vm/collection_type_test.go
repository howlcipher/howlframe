package vm

import (
	"bytes"
	"os"
	"path/filepath"
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
	t.Run("interpreter", func(t *testing.T) { assertInterpretStdout(t, collectionHappy, collectionHappyWant) })
	t.Run("bytecode", func(t *testing.T) { assertBytecodeStdout(t, collectionHappy, collectionHappyWant) })
}

func TestCollectionOpsFailClosed(t *testing.T) {
	runVMTypeErrorCases(t, []vmTypeErrCase{
		{"append string", dynamicCollection(`(append s "z")`), "append expected list"},
		{"append dict", dynamicCollection(`(append d "z")`), "append expected list"},
		{"map_set string", dynamicCollection(`(map_set s "k" "v")`), "map_set expected dict"},
		{"map_set list", dynamicCollection(`(map_set xs "k" "v")`), "map_set expected dict"},
		{"map_delete string", dynamicCollection(`(map_delete s "k")`), "map_delete expected dict"},
		{"map_delete list", dynamicCollection(`(map_delete xs "k")`), "map_delete expected dict"},
		{"map_get string", dynamicCollection(`(print (map_get s "k"))`), "map_get expected dict"},
		{"map_get list", dynamicCollection(`(print (map_get xs "0"))`), "map_get expected dict"},
		{"list_get string", dynamicCollection(`(print (list_get s 0))`), "list_get expected list"},
		{"list_get dict", dynamicCollection(`(print (list_get d 0))`), "list_get expected list"},
		{"list_get bad index", dynamicCollection(`(print (list_get xs s))`), "list_get index must be a number"},
		{"list_len dict", dynamicCollection(`(print (list_len d))`), "list_len expected list, string, or bytes"},
		{"list_len dict literal", `(cli_app (let (d (dict ("a" "b"))) (print (list_len d))))`, "list_len expected list, string, or bytes"},
		{"map_keys string", dynamicCollection(`(print (map_keys s))`), "map_keys expected dict"},
	})
}

func TestListLenAcceptsStringAndBytes(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		assertBytecodeStdout(t, `(cli_app (print (list_len "hello")))`, "5\n")
	})
	t.Run("dynamic string", func(t *testing.T) {
		assertBytecodeStdout(t, dynamicCollection(`(print (list_len s))`), "3\n")
	})
	t.Run("read_file bytes", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sample.bin")
		if err := os.WriteFile(path, []byte("abcd"), 0o644); err != nil {
			t.Fatal(err)
		}
		src := `(cli_app (try_let (raw (read_file "` + path + `")) (catch e (print e)) (print (list_len raw))))`
		_, prog := parseAndCompile(t, src)
		var out, errOut bytes.Buffer
		if exitCode := RunBytecode(prog, nil, []capability.Capability{capability.Filesystem}, strings.NewReader(""), &out, &errOut); exitCode != 0 {
			t.Fatalf("exit = %d stderr=%s", exitCode, errOut.String())
		}
		if out.String() != "4\n" {
			t.Fatalf("stdout=%q", out.String())
		}
	})
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
