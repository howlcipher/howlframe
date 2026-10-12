package javascript

import "testing"

// Held-out correctness fixes (2026-10-12) on the JS target: lists are values,
// and/or short-circuit, and a map_get miss is null (rendered as "").
func TestJSHeldoutSemantics(t *testing.T) {
	const source = `(web_app
  (let (a (list "x"))
    (do
      (append a "y")
      (append a "z")
      (let (b a)
        (do
          (append a "p")
          (append b "q")
          (print (str_join a ",") (str_join b ","))))
      (let (m (dict ("xs" (list "a")) ("n" "1")))
        (let (xs (map_get m "xs"))
          (do
            (append xs "b")
            (print "map" (list_len (map_get m "xs")) "var" (list_len xs))
            (map_set m "xs" xs)
            (print "after" (list_len (map_get m "xs")))
            (print "miss" (map_get m "zz") (is_nil (map_get m "zz")) (to_string (map_get m "zz")))
            (print (and (= 1 2) (= (list_get xs 0) "a")) (or (= 1 1) (= 1 2)))))))))`
	const want = "x,y,z,p x,y,z,q\n" +
		"map 1 var 2\n" +
		"after 2\n" +
		"miss  true \n" +
		"false true\n"
	code := generateCheckedJS(t, source)
	stdout, stderr, err := runNode(t, code)
	if err != nil || stdout != want {
		t.Fatalf("err=%v stdout=%q want %q stderr=%q\n%s", err, stdout, want, stderr, code)
	}
}
