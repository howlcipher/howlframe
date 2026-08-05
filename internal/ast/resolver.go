package ast

func ResolveModules(root *Node) {
	if root == nil || root.Type != "List" {
		return
	}

	globalRenames := make(map[string]string)
	var newChildren []*Node

	for _, child := range root.Children {
		if child.Type == "List" && len(child.Children) >= 3 && child.Children[0].Value == "module" {
			alias := child.Children[1].Value
			resolveModule(child, alias, globalRenames)
			newChildren = append(newChildren, child.Children[2:]...)
		} else {
			newChildren = append(newChildren, child)
		}
	}

	// Apply global renames (e.g. math/add -> math_add) to all remaining nodes
	for _, child := range newChildren {
		renameInTree(child, globalRenames, make(map[string]bool))
	}

	root.Children = newChildren
}

func resolveModule(module *Node, alias string, globalRenames map[string]string) {
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
				}
			} else if stmt.Children[0].Value == "defun" || stmt.Children[0].Value == "struct" || stmt.Children[0].Value == "lazy_synthesize" {
				name := stmt.Children[1].Value
				mangled := alias + "_private_" + name
				renames[name] = mangled
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
