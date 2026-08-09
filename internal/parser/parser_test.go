package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/lexer"
)

func TestParserCreatesFloatAtom(t *testing.T) {
	parser := NewParser(lexer.NewLexer("(> score 0.8)"), "float.howl")
	root := parser.ParseExpression()
	if got := root.Children[2]; got.Type != "FLOAT" || got.Value != "0.8" {
		t.Fatalf("got float node %#v", got)
	}
}

// expandWithModule parses src and expands its includes against a temp dir that
// holds modName with the given contents. ExpandIncludes only reads the included
// file from disk, so the importing source never needs to be written out.
func expandWithModule(t *testing.T, src, modName, modSrc string) *ast.Node {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, modName), []byte(modSrc), 0o644); err != nil {
		t.Fatalf("failed to write module fixture: %v", err)
	}

	root := NewParser(lexer.NewLexer(src), "main.howl").ParseExpression()
	ExpandIncludes(root, dir, 0)
	return root
}

// heads returns the head symbol of each child list, for terse structural asserts.
func heads(nodes []*ast.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Type == "List" && len(n.Children) > 0 {
			out = append(out, n.Children[0].Value)
		} else {
			out = append(out, n.Value)
		}
	}
	return out
}

// A source module is unnamed — (module stmt...) — so its statements start at
// Children[1]. The synthetic module ExpandIncludes builds is named — (module
// "alias" stmt...) — matching what ast.ResolveModules consumes at Children[2:].
// Slicing the source module at [2:] silently dropped its first statement
// (bugs.md #43).
func TestExpandIncludesRetainsModuleStatements(t *testing.T) {
	root := expandWithModule(t, `(list (use "mod.howl" as m))`, "mod.howl", `(module
		(export (defun first () 1))
		(defun second () 2)
	)`)

	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children, got %d: %v", len(root.Children), heads(root.Children))
	}
	mod := root.Children[1]
	if mod.Children[0].Value != "module" || mod.Children[1].Value != "m" {
		t.Fatalf("expected (module \"m\" ...), got (%s %s ...)", mod.Children[0].Value, mod.Children[1].Value)
	}
	if got, want := heads(mod.Children[2:]), []string{"export", "defun"}; !equalStrings(got, want) {
		t.Fatalf("expected module statements %v, got %v", want, got)
	}
}

// The first statement is retained whether or not it is an export.
func TestExpandIncludesRetainsModuleFirstStatementNonExport(t *testing.T) {
	root := expandWithModule(t, `(list (use "mod.howl" as m))`, "mod.howl", `(module
		(defun hidden () 1)
		(export (defun first () 1))
	)`)

	if len(root.Children) != 2 {
		t.Fatalf("expected 2 children, got %d: %v", len(root.Children), heads(root.Children))
	}
	mod := root.Children[1]
	if got, want := heads(mod.Children[2:]), []string{"defun", "export"}; !equalStrings(got, want) {
		t.Fatalf("expected module statements %v, got %v", want, got)
	}
}

// The deprecated (include ...) path splices the module's statements straight
// into the parent list rather than wrapping them, but must not drop the first
// one either.
func TestExpandIncludesIncludeDeprecated(t *testing.T) {
	root := expandWithModule(t, `(list (include "mod.howl"))`, "mod.howl", `(module
		(defun hidden () 1)
		(export (defun first () 1))
	)`)

	if got, want := heads(root.Children), []string{"list", "defun", "export"}; !equalStrings(got, want) {
		t.Fatalf("expected children %v, got %v", want, got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
