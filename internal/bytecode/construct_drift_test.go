package bytecode

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"testing"

	"github.com/howlcipher/howlframe/internal/construct"
)

// compileNodeHeads extracts the head symbols compileNode actually dispatches
// on, by parsing bytecode.go and reading the `switch head` statement's cases.
//
// It deliberately reads the source rather than a hand-maintained slice: a
// duplicated list is exactly the drift this test exists to prevent (bugs.md
// #45 mirrors the internal/capability precedent from improvements.md #94).
func compileNodeHeads(t *testing.T) map[string]bool {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bytecode.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse bytecode.go: %v", err)
	}

	heads := make(map[string]bool)
	found := false

	ast.Inspect(file, func(node ast.Node) bool {
		swtch, ok := node.(*ast.SwitchStmt)
		if !ok {
			return true
		}
		tag, ok := swtch.Tag.(*ast.Ident)
		if !ok || tag.Name != "head" {
			return true
		}
		found = true
		for _, stmt := range swtch.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			// clause.List == nil is the default: backstop.
			for _, expr := range clause.List {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Errorf("non-literal case in compileNode's head switch: %#v", expr)
					continue
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Errorf("could not unquote case %q: %v", lit.Value, err)
					continue
				}
				heads[value] = true
			}
		}
		return false
	})

	if !found {
		t.Fatal("could not find compileNode's `switch head` statement in bytecode.go")
	}
	if len(heads) == 0 {
		t.Fatal("compileNode's head switch has no string cases")
	}
	return heads
}

// TestConstructRegistryMatchesCompileNode is the anti-regression mechanism for
// bug #45. If someone adds a compileNode case without registering it, or marks
// a construct Supported without implementing it, this fails loudly.
func TestConstructRegistryMatchesCompileNode(t *testing.T) {
	heads := compileNodeHeads(t)

	for _, name := range construct.SupportedNames() {
		if !heads[name] {
			t.Errorf("construct %q is registered Supported but compileNode has no case for it", name)
		}
	}

	var unregistered []string
	for head := range heads {
		entry, ok := construct.Lookup(head)
		if !ok {
			unregistered = append(unregistered, head)
			continue
		}
		if entry.Support == construct.Unsupported {
			t.Errorf("compileNode has a case for %q but the registry calls it Unsupported", head)
		}
	}
	sort.Strings(unregistered)
	for _, head := range unregistered {
		t.Errorf("compileNode has a case for %q but internal/construct does not classify it", head)
	}
}

// TestCompileTimeOnlyConstructsReachingCompileNodeHaveCases pins the other
// half of the contract. An annotation that can still reach compileNode must
// have an explicit case, because the default backstop added for bug #45 now
// rejects anything it does not recognize - silently falling through is no
// longer an option. Annotations consumed by an earlier pass (patch,
// with_context, include) legitimately have no case.
func TestCompileTimeOnlyConstructsReachingCompileNodeHaveCases(t *testing.T) {
	heads := compileNodeHeads(t)
	consumedBeforeLowering := map[string]bool{
		"patch":        true, // ast.ApplyPatches
		"with_context": true, // ast.ApplyWithContext
		"include":      true, // parser.ExpandIncludes
		"use":          true, // parser.ExpandIncludes / ast.ResolveModules (improvements.md #95)
		"export":       true, // parser.ExpandIncludes / ast.ResolveModules (improvements.md #95)
		"module":       true, // parser.ExpandIncludes / ast.ResolveModules (improvements.md #95)
	}

	for _, entry := range construct.Table() {
		if entry.Support != construct.CompileTimeOnly {
			continue
		}
		if consumedBeforeLowering[entry.Name] {
			if heads[entry.Name] {
				t.Errorf("construct %q is documented as consumed before lowering but compileNode has a case for it", entry.Name)
			}
			continue
		}
		if !heads[entry.Name] {
			t.Errorf("CompileTimeOnly construct %q can reach compileNode but has no case, so the default backstop would reject it", entry.Name)
		}
	}
}
