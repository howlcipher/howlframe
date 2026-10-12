package ast_test

import (
	"sort"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/construct"
)

// structuralSugarParents are constructs whose children are not ordinary
// expression slots (names, parameter lists, type annotations, quoted
// instructions), handled by their own walker context or never containing sugar.
var structuralSugarParents = map[string]string{
	"defun":             "name and parameter list are structural; body sugar is handled by desugarDefunExprBody",
	"dict":              "children are (key value) pairs, walked as bindings",
	"neural_circuit":    "first child is a structural argument list, second a quoted instruction",
	"ephemeral_circuit": "first child is a structural argument list, second a quoted instruction",
}

// Every supported construct that compiles its children as expressions must
// expand nested density sugar; otherwise a nested (read_text …) reaches
// lowering unexpanded and the build fails (heldout-v0 h05/h12).
func TestEveryExpressionConstructExpandsNestedSugar(t *testing.T) {
	var missing []string
	for _, e := range construct.Table() {
		if e.Support != construct.Supported || e.Opaque {
			continue
		}
		if _, ok := structuralSugarParents[e.Name]; ok {
			continue
		}
		if !ast.IsSugarExprParent(e.Name) {
			missing = append(missing, e.Name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("constructs that do not expand nested sugar: %v", missing)
	}
}
