package javascript

import "testing"

// to_int and to_float of a value of the wrong type name its type as the VM
// does (%T: <nil>, bool, []interface {}, map[string]interface {}), not
// JavaScript's typeof (bugs.md #71).
func TestConversionErrorsNameVMTypes(t *testing.T) {
	got := runWebAppOutput(t, `(web_app (do
  (try_let (v (to_int (map_get (dict) "x"))) (catch e (print (to_string e))) (print v))
  (try_let (v (to_float (map_get (dict) "x"))) (catch e (print (to_string e))) (print v))
  (try_let (v (to_int true)) (catch e (print (to_string e))) (print v))
  (try_let (v (to_float false)) (catch e (print (to_string e))) (print v))
  (try_let (v (to_int (list 1))) (catch e (print (to_string e))) (print v))
  (try_let (v (to_float (dict ("a" 1)))) (catch e (print (to_string e))) (print v))))`)
	want := `Error: CONVERSION_ERROR: cannot convert <nil> to int
Error: CONVERSION_ERROR: cannot convert <nil> to float
Error: CONVERSION_ERROR: cannot convert bool to int
Error: CONVERSION_ERROR: cannot convert bool to float
Error: CONVERSION_ERROR: cannot convert []interface {} to int
Error: CONVERSION_ERROR: cannot convert map[string]interface {} to float`
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
