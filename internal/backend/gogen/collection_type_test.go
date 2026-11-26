package gogen

import (
	"strings"
	"testing"
)

func TestGoCollectionOpsAgreeOnMissAndMutation(t *testing.T) {
	code := generateCheckedGo(t, collectionHappyGo)
	if !strings.Contains(code, `howlFrameMapGet(row, "missing")`) {
		t.Fatalf("named map_get left the absence helper:\n%s", code)
	}
	if !strings.Contains(code, "howlFrameMapGetValue(") {
		t.Fatalf("dynamic map_get did not fail closed:\n%s", code)
	}
	if !strings.Contains(code, "howlFrameAppend(") || !strings.Contains(code, "howlFrameListLen(") || !strings.Contains(code, "howlFrameListGet(") {
		t.Fatalf("dynamic list ops did not use the fail-closed helpers:\n%s", code)
	}
	if got := goRun(t, "collections.go", code); got != collectionHappyWantGo {
		t.Fatalf("stdout = %q, want %q\nsource:\n%s", got, collectionHappyWantGo, code)
	}
}

func TestGoConcreteListAndDictStayDirect(t *testing.T) {
	const source = `(cli_app
  (let (items (list "a"))
    (let (d (dict ("k" "v")))
      (do
        (append items "b")
        (print (list_get items 1))
        (print (list_len items))
        (print (map_get d "missing"))
        (map_set d "k2" "v2")
        (print (map_get d "k2"))
        (map_delete d "k")
        (print (map_get d "k"))
      ))))`
	code := generateCheckedGo(t, source)
	if !strings.Contains(code, `items = append(items[:len(items):len(items)], "b")`) {
		t.Fatalf("concrete append changed shape:\n%s", code)
	}
	if !strings.Contains(code, `d["k2"] = "v2"`) || !strings.Contains(code, `delete(d, "k")`) {
		t.Fatalf("concrete dict mutation changed shape:\n%s", code)
	}
	if !strings.Contains(code, `howlFrameMapGet(d, "missing")`) {
		t.Fatalf("concrete map_get left the absence helper:\n%s", code)
	}
	const want = "b\n2\n\nv2\n\n"
	if got := goRun(t, "concrete.go", code); got != want {
		t.Fatalf("stdout = %q, want %q\nsource:\n%s", got, want, code)
	}
}

func TestGoCollectionOpsFailClosed(t *testing.T) {
	runGoTypeErrorCases(t, []goTypeErrCase{
		{"append string", dynamicCollectionGo(`(append s "z")`), "TYPE_ERROR: append expected list"},
		{"append dict", dynamicCollectionGo(`(append d "z")`), "TYPE_ERROR: append expected list"},
		{"map_set string", dynamicCollectionGo(`(map_set s "k" "v")`), "TYPE_ERROR: map_set expected dict"},
		{"map_set list", dynamicCollectionGo(`(map_set xs "k" "v")`), "TYPE_ERROR: map_set expected dict"},
		{"map_delete string", dynamicCollectionGo(`(map_delete s "k")`), "TYPE_ERROR: map_delete expected dict"},
		{"map_delete list", dynamicCollectionGo(`(map_delete xs "k")`), "TYPE_ERROR: map_delete expected dict"},
		{"map_get string", dynamicCollectionGo(`(print (map_get s "k"))`), "TYPE_ERROR: map_get expected dict"},
		{"map_get list", dynamicCollectionGo(`(print (map_get xs "0"))`), "TYPE_ERROR: map_get expected dict"},
		{"list_get string", dynamicCollectionGo(`(print (list_get s 0))`), "TYPE_ERROR: list_get expected list"},
		{"list_get dict", dynamicCollectionGo(`(print (list_get d 0))`), "TYPE_ERROR: list_get expected list"},
		{"list_get bad index", dynamicCollectionGo(`(print (list_get xs s))`), "TYPE_ERROR: list_get index must be a number"},
		{"list_len string", dynamicCollectionGo(`(print (list_len s))`), "TYPE_ERROR: list_len expected list"},
		{"list_len dict", dynamicCollectionGo(`(print (list_len d))`), "TYPE_ERROR: list_len expected list"},
		{"map_keys string", dynamicCollectionGo(`(print (map_keys s))`), "TYPE_ERROR: map_keys expected dict"},
	})
}

const collectionHappyGo = `(cli_app
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

const collectionHappyWantGo = "miss \n" +
	"empty \n" +
	"hit v\n" +
	"leaf-miss \n" +
	"get0 a\n" +
	"oob \n" +
	"len 2\n" +
	"len2 3\n" +
	"set v2\n" +
	"del \n"

func dynamicCollectionGo(body string) string {
	return `(cli_app
  (let (row (dict ("s" "ada") ("xs" (list "a" "b")) ("d" (dict ("k" "v")))))
    (let (s (map_get row "s"))
      (let (xs (map_get row "xs"))
        (let (d (map_get row "d"))
          ` + body + `)))))`
}
