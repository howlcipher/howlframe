package javascript

import (
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
)

// Every browser host operation in capability.HostOperations compiles to
// code, and is awaited exactly when the table calls it asynchronous.
func TestHostOperationsMatchRegistry(t *testing.T) {
	for _, op := range capability.HostOperations() {
		args := []string{op.Name}
		for _, p := range op.Params {
			if p.Kind == capability.HostLambda {
				args = append(args, "(lambda ("+strings.Join(p.Lambda, " ")+") (print 1))")
			} else {
				args = append(args, `"x"`)
			}
		}
		call := "(" + strings.Join(args, " ") + ")"
		code := compileWebApp(t, "(web_app "+call+")")
		at := strings.Index(code, ";(async () => {\n")
		if at < 0 {
			t.Errorf("%s: no top-level code in\n%s", op.Name, code)
			continue
		}
		body := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(code[at+len(";(async () => {\n"):]), "})();"))
		if body == "" {
			t.Errorf("%s compiled to nothing", call)
			continue
		}
		if awaited := strings.HasPrefix(body, "(await "); awaited != op.Async {
			t.Errorf("%s: awaited %v, but the table says asynchronous %v:\n%s", op.Name, awaited, op.Async, body)
		}
	}
}
