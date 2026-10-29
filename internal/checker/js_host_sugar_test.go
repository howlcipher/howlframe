package checker

import (
	"sort"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
)

// jsOnlyValueHeads are the JavaScript-target forms checked outside the
// jsHostOperations table that take value arguments.
var jsOnlyValueHeads = []string{
	"dom_query", "dom_text", "dom_value", "download_file", "on_event", "set_attr", "set_html", "set_text",
	"toggle_class", "url_encode", "zip_build",
}

// Sugar such as (not x) nested in an argument of a browser host operation
// must expand like it does under any other value-taking form; otherwise
// (set_value el (not x)) fails to build with "Unknown statement for JS: not".
func TestEveryJSHostOperationExpandsNestedSugar(t *testing.T) {
	var missing []string
	heads := append([]string(nil), jsOnlyValueHeads...)
	for h := range jsHostOperations {
		heads = append(heads, h)
	}
	for h := range jsSubscriptionOperations {
		heads = append(heads, h)
	}
	for _, h := range heads {
		if !ast.IsSugarExprParent(h) {
			missing = append(missing, h)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("host operations that do not expand nested sugar: %v", missing)
	}
}
