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
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{name: "append string", source: dynamicCollectionJS(`(append s "z")`), want: "TYPE_ERROR: append expected list"},
		{name: "append dict", source: dynamicCollectionJS(`(append d "z")`), want: "TYPE_ERROR: append expected list"},
		{name: "map_set string", source: dynamicCollectionJS(`(map_set s "k" "v")`), want: "TYPE_ERROR: map_set expected dict"},
		{name: "map_set list", source: dynamicCollectionJS(`(map_set xs "k" "v")`), want: "TYPE_ERROR: map_set expected dict"},
		{name: "map_delete string", source: dynamicCollectionJS(`(map_delete s "k")`), want: "TYPE_ERROR: map_delete expected dict"},
		{name: "map_delete list", source: dynamicCollectionJS(`(map_delete xs "k")`), want: "TYPE_ERROR: map_delete expected dict"},
		{name: "map_get string", source: dynamicCollectionJS(`(print (map_get s "k"))`), want: "TYPE_ERROR: map_get expected dict, got string"},
		{name: "map_get list", source: dynamicCollectionJS(`(print (map_get xs "0"))`), want: "TYPE_ERROR: map_get expected dict, got list"},
		{name: "list_get string", source: dynamicCollectionJS(`(print (list_get s 0))`), want: "TYPE_ERROR: list_get expected list"},
		{name: "list_get dict", source: dynamicCollectionJS(`(print (list_get d 0))`), want: "TYPE_ERROR: list_get expected list"},
		{name: "list_get bad index", source: dynamicCollectionJS(`(print (list_get xs s))`), want: "TYPE_ERROR: list_get index must be a number"},
		{name: "list_len string", source: dynamicCollectionJS(`(print (list_len s))`), want: "TYPE_ERROR: list_len expected list"},
		{name: "list_len dict", source: dynamicCollectionJS(`(print (list_len d))`), want: "TYPE_ERROR: list_len expected list"},
		{name: "map_keys string", source: dynamicCollectionJS(`(print (map_keys s))`), want: "TYPE_ERROR: map_keys expected dict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := generateCheckedJS(t, tc.source)
			stdout, stderr, err := runNode(t, code)
			if err == nil {
				t.Fatalf("wrong type succeeded, stdout=%q stderr=%q\nsource:\n%s", stdout, stderr, code)
			}
			if strings.TrimSpace(stdout) != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr = %q, want %q", stderr, tc.want)
			}
		})
	}
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
