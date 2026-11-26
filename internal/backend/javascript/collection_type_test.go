package javascript

import (
	"strings"
	"testing"
)

func TestJSCollectionOpsAgreeOnMissAndMutation(t *testing.T) {
	code := generateCheckedJS(t, collectionHappyJS)
	if !strings.Contains(code, `howlFrameMapGet(row, "missing")`) || !strings.Contains(code, `?? ""`) {
		t.Fatalf("named map_get lost the empty-string miss:\n%s", code)
	}
	for _, want := range []string{
		"TYPE_ERROR: map_get expected dict",
		"TYPE_ERROR: map_set expected dict",
		"TYPE_ERROR: map_delete expected dict",
		"TYPE_ERROR: append expected list",
		"TYPE_ERROR: list_get expected list",
		"TYPE_ERROR: list_len expected list",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated JavaScript is missing %q:\n%s", want, code)
		}
	}
	stdout, stderr, err := runNode(t, code)
	if err != nil {
		t.Fatalf("node: %v\nstderr=%s\nsource:\n%s", err, stderr, code)
	}
	if stdout != collectionHappyWantJS {
		t.Fatalf("stdout = %q, want %q\nsource:\n%s", stdout, collectionHappyWantJS, code)
	}
}

func TestJSCollectionOpsFailClosed(t *testing.T) {
	runJSTypeErrorCases(t, []jsTypeErrCase{
		{"append string", dynamicCollectionJS(`(append s "z")`), "TYPE_ERROR: append expected list"},
		{"append dict", dynamicCollectionJS(`(append d "z")`), "TYPE_ERROR: append expected list"},
		{"map_set string", dynamicCollectionJS(`(map_set s "k" "v")`), "TYPE_ERROR: map_set expected dict"},
		{"map_set list", dynamicCollectionJS(`(map_set xs "k" "v")`), "TYPE_ERROR: map_set expected dict"},
		{"map_delete string", dynamicCollectionJS(`(map_delete s "k")`), "TYPE_ERROR: map_delete expected dict"},
		{"map_delete list", dynamicCollectionJS(`(map_delete xs "k")`), "TYPE_ERROR: map_delete expected dict"},
		{"map_get string", dynamicCollectionJS(`(print (map_get s "k"))`), "TYPE_ERROR: map_get expected dict, got string"},
		{"map_get list", dynamicCollectionJS(`(print (map_get xs "0"))`), "TYPE_ERROR: map_get expected dict, got list"},
		{"list_get string", dynamicCollectionJS(`(print (list_get s 0))`), "TYPE_ERROR: list_get expected list"},
		{"list_get dict", dynamicCollectionJS(`(print (list_get d 0))`), "TYPE_ERROR: list_get expected list"},
		{"list_get bad index", dynamicCollectionJS(`(print (list_get xs s))`), "TYPE_ERROR: list_get index must be a number"},
		{"list_len string", dynamicCollectionJS(`(print (list_len s))`), "TYPE_ERROR: list_len expected list"},
		{"list_len dict", dynamicCollectionJS(`(print (list_len d))`), "TYPE_ERROR: list_len expected list"},
		{"map_keys string", dynamicCollectionJS(`(print (map_keys s))`), "TYPE_ERROR: map_keys expected dict"},
	})
}

const collectionHappyJS = `(web_app
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

const collectionHappyWantJS = "miss \n" +
	"empty \n" +
	"hit v\n" +
	"leaf-miss \n" +
	"get0 a\n" +
	"oob \n" +
	"len 2\n" +
	"len2 3\n" +
	"set v2\n" +
	"del \n"

func dynamicCollectionJS(body string) string {
	return `(web_app
  (let (row (dict ("s" "ada") ("xs" (list "a" "b")) ("d" (dict ("k" "v")))))
    (let (s (map_get row "s"))
      (let (xs (map_get row "xs"))
        (let (d (map_get row "d"))
          ` + body + `)))))`
}
