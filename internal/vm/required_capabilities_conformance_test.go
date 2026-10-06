package vm

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestRequiredCapabilitiesSourceConformance(t *testing.T) {
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "vm.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	render := func(n ast.Node) string {
		var b bytes.Buffer
		if err := format.Node(&b, fs, n); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || render(fn.Recv.List[0].Type) != "*BCVM" {
			continue
		}
		var walk func(ast.Node, string)
		walk = func(n ast.Node, rule string) {
			if n == nil {
				return
			}
			ast.Inspect(n, func(child ast.Node) bool {
				if c, ok := child.(*ast.CaseClause); ok {
					label := ""
					for _, v := range c.List {
						label += render(v)
					}
					for _, stmt := range c.Body {
						walk(stmt, label)
					}
					return false
				}
				if call, ok := child.(*ast.CallExpr); ok && render(call.Fun) == "vm.requireCapability" {
					got[fn.Name.Name+":"+rule+":"+render(call.Args[0])]++
				}
				return true
			})
		}
		walk(fn.Body, "")
	}
	want := map[string]int{"requireStoreCapabilities::requiredCap": 1, "run::spec.Capability": 1, "run:bytecode.OpStorePut:capability.Filesystem": 1, "run:bytecode.OpStoreGet:capability.Filesystem": 1, "run:bytecode.OpStoreKeys:capability.Filesystem": 1, "run:bytecode.OpStoreDelete:capability.Filesystem": 1, "run:bytecode.OpCall:capability.Network": 1}
	// Registry, STORE_OPEN/private handle provenance, and lazy CALL are C1 rules.
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BCVM capability sites changed: update bytecode.RequiredCapabilities and this rule mapping; got %v want %v", got, want)
	}
}
