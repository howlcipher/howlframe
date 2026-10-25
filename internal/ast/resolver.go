package ast

import (
	"fmt"
	"strings"
)

// moduleNames records which names a used module defines, so a call that
// spells one of them as alias.name instead of alias/name can be refused
// with a precise diagnostic (#224).
type moduleNames struct {
	exported map[string]bool
	private  map[string]bool
}

func ResolveModules(root *Node) {
	if root == nil || root.Type != "List" {
		return
	}

	globalRenames := make(map[string]string)
	modules := make(map[string]moduleNames)
	var newChildren []*Node

	for _, child := range root.Children {
		if child.Type == "List" && len(child.Children) >= 3 && child.Children[0].Value == "module" {
			alias := child.Children[1].Value
			modules[alias] = resolveModule(child, alias, globalRenames)
			newChildren = append(newChildren, child.Children[2:]...)
		} else {
			newChildren = append(newChildren, child)
		}
	}

	// Refuse alias.name calls to module functions before renaming, while
	// local bindings are still visible.
	if len(modules) > 0 {
		for _, child := range newChildren {
			rejectDottedModuleCalls(child, modules, make(map[string]bool))
		}
	}

	// Apply global renames (e.g. math/add -> math_add) to all remaining nodes
	for _, child := range newChildren {
		renameInTree(child, globalRenames, make(map[string]bool))
	}

	root.Children = newChildren
}

// rejectDottedModuleCalls reports a (call alias.name ...) whose alias is a
// used module and whose name is a function that module defines. The module
// resolver only links alias/name, so the dotted spelling never resolves and
// used to reach the VM as a missing function. A dotted call whose name is not
// one of the module's functions is left alone: it may be Go interop, and a
// bytecode build still refuses an unresolved call before writing an artifact.
// A local binding named like the alias shadows it, scoped the way each binding
// form evaluates: an initializer, iterable or try_let value is outside its own
// binding, and a catch handler sees only its error name.
func rejectDottedModuleCalls(node *Node, modules map[string]moduleNames, locals map[string]bool) {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return
	}
	kids := node.Children
	walkAll := func(nodes []*Node, scope map[string]bool) {
		for _, child := range nodes {
			rejectDottedModuleCalls(child, modules, scope)
		}
	}
	with := func(names ...string) map[string]bool {
		scope := cloneSet(locals)
		for _, name := range names {
			scope[name] = true
		}
		return scope
	}
	switch kids[0].Value {
	case "let":
		if len(kids) >= 2 && isBindingList(kids[1]) {
			scope := cloneSet(locals)
			for i := 0; i+1 < len(kids[1].Children); i += 2 {
				rejectDottedModuleCalls(kids[1].Children[i+1], modules, scope)
				scope[kids[1].Children[i].Value] = true
			}
			walkAll(kids[2:], scope)
			return
		}
	case "try_let":
		if len(kids) >= 2 && isBindingList(kids[1]) && len(kids[1].Children) == 2 {
			rejectDottedModuleCalls(kids[1].Children[1], modules, locals)
			for _, rest := range kids[2:] {
				if rest.Type == "List" && len(rest.Children) >= 2 && rest.Children[0].Value == "catch" && rest.Children[1].Type == "SYMBOL" {
					walkAll(rest.Children[2:], with(rest.Children[1].Value))
					continue
				}
				rejectDottedModuleCalls(rest, modules, with(kids[1].Children[0].Value))
			}
			return
		}
	case "for":
		if len(kids) >= 3 && kids[1].Type == "SYMBOL" {
			rejectDottedModuleCalls(kids[2], modules, locals)
			walkAll(kids[3:], with(kids[1].Value))
			return
		}
	case "defun", "lazy_synthesize":
		if len(kids) >= 3 && kids[2].Type == "List" {
			walkAll(kids[3:], with(paramNames(kids[2])...))
			return
		}
	case "lambda":
		if len(kids) >= 2 && kids[1].Type == "List" {
			walkAll(kids[2:], with(paramNames(kids[1])...))
			return
		}
	case "call":
		if len(kids) >= 2 && kids[1].Type == "SYMBOL" {
			reportDottedModuleCall(kids[1], modules, locals)
		}
	}
	walkAll(kids, locals)
}

func reportDottedModuleCall(symbol *Node, modules map[string]moduleNames, locals map[string]bool) {
	alias, member, ok := moduleAliasPrefix(symbol.Value, modules)
	if !ok || locals[alias] || locals[strings.SplitN(alias, ".", 2)[0]] {
		return
	}
	reason := fmt.Sprintf("undefined reference to function %q", symbol.Value)
	if modules[alias].exported[member] {
		ReportError(fmt.Sprintf("%s: call a function exported by module %s with a slash, (call %s/%s ...), not a dot", reason, alias, alias, member), symbol.Line, symbol.Column)
	}
	if modules[alias].private[member] {
		ReportError(fmt.Sprintf("%s: module %s does not export %s; wrap its defun in (export ...) in the module file, then call it as (call %s/%s ...)", reason, alias, member, alias, member), symbol.Line, symbol.Column)
	}
}

// isBindingList reports whether node is a let binding list: an even number of
// children alternating a symbol name and its initializer.
func isBindingList(node *Node) bool {
	if node.Type != "List" || len(node.Children) == 0 || len(node.Children)%2 != 0 {
		return false
	}
	for i := 0; i < len(node.Children); i += 2 {
		if node.Children[i].Type != "SYMBOL" {
			return false
		}
	}
	return true
}

// paramNames lists defun or lambda parameter names; a typed parameter is a
// list whose first element is the name.
func paramNames(params *Node) []string {
	var names []string
	for _, param := range params.Children {
		if param.Type == "List" && len(param.Children) > 0 {
			names = append(names, param.Children[0].Value)
		} else {
			names = append(names, param.Value)
		}
	}
	return names
}

// moduleAliasPrefix splits alias.member for the longest used-module alias
// that prefixes name, so a dotted alias such as m.sub is matched whole.
func moduleAliasPrefix(name string, modules map[string]moduleNames) (string, string, bool) {
	best := ""
	for alias := range modules {
		if len(alias) > len(best) && len(name) > len(alias)+1 && strings.HasPrefix(name, alias+".") {
			best = alias
		}
	}
	if best == "" {
		return "", "", false
	}
	return best, name[len(best)+1:], true
}

func resolveModule(module *Node, alias string, globalRenames map[string]string) moduleNames {
	names := moduleNames{exported: map[string]bool{}, private: map[string]bool{}}
	renames := make(map[string]string)

	for _, stmt := range module.Children[2:] {
		if stmt.Type == "List" && len(stmt.Children) > 0 {
			if stmt.Children[0].Value == "export" && len(stmt.Children) == 2 {
				inner := stmt.Children[1]
				if inner.Type == "List" && len(inner.Children) > 1 && (inner.Children[0].Value == "defun" || inner.Children[0].Value == "struct" || inner.Children[0].Value == "lazy_synthesize") {
					name := inner.Children[1].Value
					mangled := alias + "_" + name
					renames[name] = mangled
					globalRenames[alias+"/"+name] = mangled
					if inner.Children[0].Value != "struct" {
						names.exported[name] = true
					}
				}
			} else if (stmt.Children[0].Value == "defun" || stmt.Children[0].Value == "struct" || stmt.Children[0].Value == "lazy_synthesize") && len(stmt.Children) > 1 {
				name := stmt.Children[1].Value
				mangled := alias + "_private_" + name
				renames[name] = mangled
				if stmt.Children[0].Value != "struct" {
					names.private[name] = true
				}
			}
		}
	}

	for _, stmt := range module.Children[2:] {
		renameInTree(stmt, renames, make(map[string]bool))
	}

	// Unpack exports
	for i, stmt := range module.Children {
		if i >= 2 && stmt.Type == "List" && len(stmt.Children) == 2 && stmt.Children[0].Value == "export" {
			module.Children[i] = stmt.Children[1]
		}
	}
	return names
}

func renameInTree(node *Node, renames map[string]string, locals map[string]bool) {
	if node == nil {
		return
	}
	if node.Type == "SYMBOL" {
		if !locals[node.Value] {
			if newName, ok := renames[node.Value]; ok {
				node.Value = newName
			} else if hasPrefix(node.Value, renames) {
				for k, v := range renames {
					if len(node.Value) > len(k)+1 && node.Value[:len(k)+1] == k+"." {
						node.Value = v + node.Value[len(k):]
						break
					}
				}
			}
		}
		return
	}

	if node.Type == "List" && len(node.Children) > 0 {
		head := node.Children[0].Value
		newLocals := cloneSet(locals)

		if head == "let" || head == "try_let" {
			bindings, _ := LetChain(node)
			for _, b := range bindings {
				if b.Type == "List" && len(b.Children) > 0 {
					newLocals[b.Children[0].Value] = true
				}
			}
		} else if head == "defun" || head == "lazy_synthesize" {
			if len(node.Children) >= 3 {
				args := node.Children[2]
				if args.Type == "List" {
					for _, arg := range args.Children {
						if arg.Type == "List" && len(arg.Children) > 0 {
							newLocals[arg.Children[0].Value] = true
						} else {
							newLocals[arg.Value] = true
						}
					}
				}
			}
		} else if head == "lambda" {
			if len(node.Children) >= 2 {
				args := node.Children[1]
				if args.Type == "List" {
					for _, arg := range args.Children {
						newLocals[arg.Value] = true
					}
				}
			}
		} else if head == "for" && len(node.Children) >= 2 {
			newLocals[node.Children[1].Value] = true
		}

		for _, child := range node.Children {
			renameInTree(child, renames, newLocals)
		}
	}
}

func hasPrefix(s string, renames map[string]string) bool {
	for k := range renames {
		if len(s) > len(k)+1 && s[:len(k)+1] == k+"." {
			return true
		}
	}
	return false
}

func cloneSet(s map[string]bool) map[string]bool {
	c := make(map[string]bool)
	for k, v := range s {
		c[k] = v
	}
	return c
}
