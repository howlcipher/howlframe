package gogen

import "testing"

// Held-out correctness fixes (2026-10-12) on the Go target: lists are values,
// and/or short-circuit, and a map_get miss is nil (rendered as ""). Go dict
// literals are string-valued, so lists-in-maps are covered on the VM paths
// and the JS target; here the value rule is checked on list aliasing.
func TestGoHeldoutSemantics(t *testing.T) {
	const source = `(cli_app
  (let (a (list "x"))
    (do
      (append a "y")
      (append a "z")
      (let (b a)
        (do
          (append a "p")
          (append b "q")
          (print (str_join a ",") (str_join b ","))))
      (let (m (dict ("n" "1")))
        (do
          (print "miss" (map_get m "zz") (is_nil (map_get m "zz")) (to_string (map_get m "zz")))
          (print (and (= 1 2) (= (list_get a 0) "x")) (or (= 1 1) (= 1 2))))))))`
	const want = "x,y,z,p x,y,z,q\n" +
		"miss  true \n" +
		"false true\n"
	code := generateCheckedGo(t, source)
	if got := goRun(t, "main.go", code); got != want {
		t.Fatalf("got %q want %q\n%s", got, want, code)
	}
}
