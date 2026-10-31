package checker

import (
	"slices"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/capability"
)

// Sugar such as (not x) nested in an argument of a browser host operation
// must expand like it does under any other value-taking form; otherwise
// (set_value el (not x)) fails to build with "Unknown statement for JS: not".
// The desugarer keeps its own set of these operations (it cannot import the
// table), so the two must name the same operations.
func TestEveryJSHostOperationExpandsNestedSugar(t *testing.T) {
	names := capability.HostOperationNames()
	if got := ast.BrowserHostSugarParents(); !slices.Equal(got, names) {
		t.Fatalf("the desugarer's browser host operations differ from capability.HostOperations:\n got %v\nwant %v", got, names)
	}
	for _, h := range names {
		if !ast.IsSugarExprParent(h) {
			t.Errorf("%s does not expand nested sugar", h)
		}
	}
}
