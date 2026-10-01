package javascript

import (
	"os/exec"
	"strings"
	"testing"
)

// HFREC-049: typed (name type) parameters must keep their names in generated
// JavaScript, identical to untyped parameters.
func TestTypedAndUntypedParametersEmitSameNames(t *testing.T) {
	typed := generateCheckedJS(t, `(web_app (defun echo ((message string) (n int) plain) string (return message)))`)
	if !strings.Contains(typed, "async function echo(message, n, plain)") {
		t.Fatalf("typed parameter names lost:\n%s", typed)
	}
	untyped := generateCheckedJS(t, `(web_app (defun echo (message n plain) (return message)))`)
	if !strings.Contains(untyped, "async function echo(message, n, plain)") {
		t.Fatalf("untyped parameter names changed:\n%s", untyped)
	}
}

func TestTypedParametersExecuteInNode(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not available")
	}
	tests := map[string]string{
		"single typed":      `(web_app (defun echo ((message string)) string (return message)) (print (call echo "ok")))`,
		"multiple typed":    `(web_app (defun join2 ((a string) (b string)) string (return (+ a b))) (print (call join2 "o" "k")))`,
		"mixed typed/plain": `(web_app (defun pick ((a string) b) string (return (+ a b))) (print (call pick "o" "k")))`,
	}
	for name, source := range tests {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, err := runNode(t, generateCheckedJS(t, source))
			if err != nil || strings.TrimSpace(stdout) != "ok" {
				t.Fatalf("stdout=%q stderr=%q err=%v", stdout, stderr, err)
			}
		})
	}
}
