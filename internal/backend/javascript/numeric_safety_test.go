package javascript

import (
	"os/exec"
	"strings"
	"testing"
)

func TestJSToIntTruncatesAndParseJSONFailsClosed(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not available")
	}
	stdout, stderr, err := runNode(t, generateCheckedJS(t, `(web_app (print (to_int (/ 9 2)) (to_int 3.9) (to_int (- 0 3.9))))`))
	if err != nil || strings.TrimSpace(stdout) != "4 3 -3" {
		t.Fatalf("to_int truncation: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	// An integer token beyond 2^53 must fail rather than round and flip a decision.
	src := `(web_app (let (raw "{\"a\":9007199254740993}") (let (obj (parse_json Doc raw)) (print (map_get obj "a")))))`
	stdout, stderr, err = runNode(t, generateCheckedJS(t, src))
	if err == nil || !strings.Contains(stderr, "CONVERSION_ERROR") {
		t.Fatalf("unsafe JSON integer accepted: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	src = `(web_app (let (raw "{\"a\":9007199254740991}") (let (obj (parse_json Doc raw)) (print (map_get obj "a")))))`
	stdout, stderr, err = runNode(t, generateCheckedJS(t, src))
	if err != nil || strings.TrimSpace(stdout) != "9007199254740991" {
		t.Fatalf("safe JSON integer: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
}

// Computed int64 results beyond 2^53 must fail closed in JavaScript instead of
// rounding and flipping a comparison-based decision.
func TestJSIntegerArithmeticBeyondSafeRangeFailsClosed(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not available")
	}
	decision := `(web_app (let (a 9007199254740991) (let (b (+ a 2)) (if (> b (+ a 1)) (print "ALLOW") (print "DENY")))))`
	stdout, stderr, err := runNode(t, generateCheckedJS(t, decision))
	if err == nil || !strings.Contains(stderr, "RUNTIME_ERROR") || strings.Contains(stdout, "ALLOW") || strings.Contains(stdout, "DENY") {
		t.Fatalf("unsafe integer arithmetic did not fail closed: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	// Float arithmetic and in-range integer arithmetic are unaffected.
	stdout, stderr, err = runNode(t, generateCheckedJS(t, `(web_app (print (* 100000000000000000000.0 2.0) (+ 9007199254740990 1) (- 1 2)))`))
	if err != nil || strings.TrimSpace(stdout) != "200000000000000000000 9007199254740991 -1" {
		t.Fatalf("in-range arithmetic changed: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
}

// Operands whose types are not statically known (for example JSON fields) are
// range-checked at runtime, and string concatenation still works.
func TestJSDynamicIntegerArithmeticFailsClosed(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not available")
	}
	body := `(web_app (let (raw "{\"a\":9007199254740991,\"b\":2,\"s\":\"x\"}") (let (obj (parse_json Doc raw)) %s)))`
	stdout, stderr, err := runNode(t, generateCheckedJS(t, strings.Replace(body, "%s", `(print (+ (map_get obj "a") (map_get obj "b")))`, 1)))
	if err == nil || !strings.Contains(stderr, "RUNTIME_ERROR") || stdout != "" {
		t.Fatalf("dynamic unsafe sum did not fail closed: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
	stdout, stderr, err = runNode(t, generateCheckedJS(t, strings.Replace(body, "%s", `(print (+ (map_get obj "s") "y") (+ (map_get obj "b") 3))`, 1)))
	if err != nil || strings.TrimSpace(stdout) != "xy 5" {
		t.Fatalf("safe dynamic arithmetic changed: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
}

// Engines without JSON.parse source access cannot tell integer tokens from
// whole-valued floats, so the helper must fail closed rather than round.
func TestJSParseJSONFailsClosedWithoutSourceAccess(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not available")
	}
	src := `(web_app (let (raw "{\"a\":9007199254740993}") (let (obj (parse_json Doc raw)) (print (map_get obj "a")))))`
	shim := "const nativeParse = JSON.parse;\nJSON.parse = function (text, reviver) { return nativeParse(text, function (k, v) { return reviver.call(this, k, v); }); };\n"
	stdout, stderr, err := runNode(t, shim+generateCheckedJS(t, src))
	if err == nil || !strings.Contains(stderr, "CONVERSION_ERROR") {
		t.Fatalf("source-less engine accepted an unsafe integer: stdout=%q stderr=%q err=%v", stdout, stderr, err)
	}
}
