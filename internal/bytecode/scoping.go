package bytecode

import (
	"strconv"

	"github.com/howlcipher/howlframe/internal/ast"
)

// ScopeBindings gives each binding that shadows a visible variable of the
// same name a fresh name, and rewrites the references inside its scope.
//
// The VM keeps one environment per function call and stores every binding
// in it by name, so without this an inner let, for or try_let overwrote the
// outer variable for the rest of the function. The direct interpreter and
// the other backends scope lexically; this pass makes the bytecode do the
// same without new opcodes. Bindings that shadow nothing keep their names,
// so messages and existing programs are unchanged. Fresh names contain
// "#", which source identifiers cannot.
//
// CompileToBytecode applies it; a path that lowers the same AST to bytecode
// another way (the experimental HFIR lowering) applies it first as well.
func ScopeBindings(root *ast.Node) {
	s := &scoper{}
	s.walk(root, []map[string]string{{}})
}

type scoper struct{ n int }

func (s *scoper) fresh(name string) string {
	s.n++
	return name + "#" + strconv.Itoa(s.n)
}

func lookupName(scopes []map[string]string, name string) (string, bool) {
	for i := len(scopes) - 1; i >= 0; i-- {
		if v, ok := scopes[i][name]; ok {
			return v, true
		}
	}
	return "", false
}

// bind adds name to a new innermost scope, renaming it when it shadows.
func (s *scoper) bind(scopes []map[string]string, nameNode *ast.Node) []map[string]string {
	inner := map[string]string{}
	if nameNode != nil && nameNode.Type == "SYMBOL" {
		name := nameNode.Value
		if _, shadows := lookupName(scopes, name); shadows {
			renamed := s.fresh(name)
			inner[name] = renamed
			nameNode.Value = renamed
		} else {
			inner[name] = name
		}
	}
	return append(append([]map[string]string(nil), scopes...), inner)
}

func (s *scoper) walk(node *ast.Node, scopes []map[string]string) {
	if node == nil {
		return
	}
	if node.Type == "SYMBOL" {
		if v, ok := lookupName(scopes, node.Value); ok && v != node.Value {
			node.Value = v
		}
		return
	}
	if node.Type != "List" || len(node.Children) == 0 {
		return
	}
	head := ""
	if node.Children[0].Type == "SYMBOL" {
		head = node.Children[0].Value
	}
	kids := node.Children
	switch head {
	case "let":
		// (let (name value) body...): the value sees the outer scope.
		if len(kids) >= 2 && kids[1].Type == "List" && len(kids[1].Children) == 2 {
			s.walk(kids[1].Children[1], scopes)
			inner := s.bind(scopes, kids[1].Children[0])
			for _, k := range kids[2:] {
				s.walk(k, inner)
			}
			return
		}
	case "for":
		// (for item list body...)
		if len(kids) >= 3 {
			s.walk(kids[2], scopes)
			inner := s.bind(scopes, kids[1])
			for _, k := range kids[3:] {
				s.walk(k, inner)
			}
			return
		}
	case "try_let":
		// (try_let (name value) (catch err handler) body)
		if len(kids) == 4 && kids[1].Type == "List" && len(kids[1].Children) == 2 && kids[2].Type == "List" && len(kids[2].Children) == 3 {
			s.walk(kids[1].Children[1], scopes)
			s.walk(kids[2].Children[2], s.bind(scopes, kids[2].Children[1]))
			s.walk(kids[3], s.bind(scopes, kids[1].Children[0]))
			return
		}
	case "defun", "lazy_synthesize":
		// A function sees only its parameters (and globals by their names).
		if len(kids) >= 3 && kids[2].Type == "List" {
			params := map[string]string{}
			for _, p := range kids[2].Children {
				if p.Type == "SYMBOL" {
					params[p.Value] = p.Value
				}
			}
			for _, k := range kids[3:] {
				s.walk(k, []map[string]string{params})
			}
			return
		}
	case "call":
		// The function name is not a variable.
		for _, k := range kids[2:] {
			s.walk(k, scopes)
		}
		return
	case "type_hint", "type_hints", "type_param", "dict":
		if head == "dict" {
			for _, pair := range kids[1:] {
				if pair.Type == "List" && len(pair.Children) == 2 {
					s.walk(pair.Children[1], scopes)
				}
			}
		}
		return
	}
	for _, k := range kids[1:] {
		s.walk(k, scopes)
	}
}
