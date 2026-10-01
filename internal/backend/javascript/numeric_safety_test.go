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
