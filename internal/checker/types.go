package checker

import (
	"fmt"
	"strings"
	"zero/internal/ast"
)

// Diagnostic is a semantic error that can be reported without terminating a
// library caller. The command-line checker converts these into Zero's normal
// JSON error format in Check.
type Diagnostic struct {
	Reason string
	Line   int
	Column int
}

// FunctionInfo is the typed signature collected from a defun declaration.
type FunctionInfo struct {
	Params []ast.TypeInfo
	Return ast.TypeInfo
}

// Analysis contains the inferred type/layout for every visited AST node.
// The AST nodes are also annotated so later lowering passes can consume the
// metadata without maintaining a second node-keyed lookup table.
type Analysis struct {
	Types       map[*ast.Node]ast.TypeInfo
	Functions   map[string]FunctionInfo
	Diagnostics []Diagnostic
}

type typeEnv map[string]ast.TypeInfo

// Analyze performs the non-fatal semantic pass. Dynamic or unresolved values
// remain Unknown; diagnostics are emitted only when both sides of a rule are
// known well enough to prove the program is invalid.
func Analyze(root *ast.Node) *Analysis {
	a := &Analysis{
		Types:     make(map[*ast.Node]ast.TypeInfo),
		Functions: make(map[string]FunctionInfo),
	}
	if root == nil {
		return a
	}
	a.collectFunctions(root)
	a.inferRoot(root)
	return a
}

func (a *Analysis) collectFunctions(node *ast.Node) {
	if node == nil {
		return
	}
	if node.Type == "List" && len(node.Children) > 0 {
		if node.Children[0].Type == "SYMBOL" && node.Children[0].Value == "defun" && len(node.Children) >= 4 {
			name := node.Children[1].Value
			params, ret := functionSignature(node)
			a.Functions[name] = FunctionInfo{Params: params, Return: ret}
		}
		for _, child := range node.Children {
			a.collectFunctions(child)
		}
	}
}

func (a *Analysis) inferRoot(root *ast.Node) {
	if root.Type != "List" || len(root.Children) == 0 {
		a.infer(root, typeEnv{})
		return
	}
	env := typeEnv{}
	for _, child := range root.Children[1:] {
		if child.Type == "List" && len(child.Children) > 0 && child.Children[0].Type == "SYMBOL" {
			switch child.Children[0].Value {
			case "defun":
				a.inferFunction(child)
				continue
			case "struct", "schema", "import", "intent", "test":
				if child.Children[0].Value == "test" && len(child.Children) > 2 {
					for _, body := range child.Children[2:] {
						a.infer(body, env)
					}
				}
				continue
			}
		}
		a.infer(child, env)
	}
}

func (a *Analysis) inferFunction(node *ast.Node) {
	params, ret := functionSignature(node)
	env := typeEnv{}
	args := node.Children[2]
	for i, arg := range args.Children {
		name := arg.Value
		if arg.Type == "List" && len(arg.Children) > 0 {
			name = arg.Children[0].Value
		}
		if i < len(params) {
			env[name] = params[i]
		}
	}
	body := node.Children[len(node.Children)-1]
	got := a.infer(body, env)
	if ret.Kind != ast.Unknown && ret.Kind != ast.Any && got.Kind != ast.Unknown && got.Kind != ast.Any && !compatible(ret, got) {
		a.add(node, fmt.Sprintf("function %q returns %s but body has type %s", node.Children[1].Value, typeName(ret), typeName(got)))
	}
}

func functionSignature(node *ast.Node) ([]ast.TypeInfo, ast.TypeInfo) {
	args := node.Children[2]
	params := make([]ast.TypeInfo, 0, len(args.Children))
	for _, arg := range args.Children {
		if arg.Type == "List" && len(arg.Children) >= 2 {
			params = append(params, typeFromName(arg.Children[1].Value))
		} else {
			params = append(params, ast.Layout(ast.String))
		}
	}
	ret := ast.Layout(ast.String)
	bodyStart := 3
	if len(node.Children) > 4 && node.Children[3].Type == "SYMBOL" {
		ret = typeFromName(node.Children[3].Value)
		bodyStart = 4
	}
	for i := bodyStart; i < len(node.Children)-1; i++ {
		cfg := node.Children[i]
		if cfg.Type != "List" || len(cfg.Children) == 0 {
			continue
		}
		switch cfg.Children[0].Value {
		case "type_hint":
			if len(cfg.Children) >= 3 {
				if cfg.Children[1].Value == "return" {
					ret = typeFromName(cfg.Children[2].Value)
				} else {
					for index, arg := range args.Children {
						name := arg.Value
						if arg.Type == "List" && len(arg.Children) > 0 {
							name = arg.Children[0].Value
						}
						if name == cfg.Children[1].Value && index < len(params) {
							params[index] = typeFromName(cfg.Children[2].Value)
						}
					}
				}
			}
		case "type_hints":
			for _, pair := range cfg.Children[1:] {
				if pair.Type != "List" || len(pair.Children) < 2 {
					continue
				}
				if pair.Children[0].Value == "return" {
					ret = typeFromName(pair.Children[1].Value)
					continue
				}
				for index, arg := range args.Children {
					name := arg.Value
					if arg.Type == "List" && len(arg.Children) > 0 {
						name = arg.Children[0].Value
					}
					if name == pair.Children[0].Value && index < len(params) {
						params[index] = typeFromName(pair.Children[1].Value)
					}
				}
			}
		}
	}
	return params, ret
}

func (a *Analysis) infer(node *ast.Node, env typeEnv) ast.TypeInfo {
	if node == nil {
		return ast.Layout(ast.Unknown)
	}
	if existing, ok := a.Types[node]; ok {
		return existing
	}
	var result ast.TypeInfo
	switch node.Type {
	case "INT":
		result = ast.Layout(ast.Int)
	case "STRING":
		result = ast.Layout(ast.String)
	case "SYMBOL":
		switch node.Value {
		case "true", "false":
			result = ast.Layout(ast.Bool)
		default:
			result = env[node.Value]
			if result.Kind == "" {
				result = ast.Layout(ast.Unknown)
			}
		}
	case "List":
		result = a.inferList(node, env)
	default:
		result = ast.Layout(ast.Unknown)
	}
	a.Types[node] = result
	node.Inferred = result
	return result
}

func (a *Analysis) inferList(node *ast.Node, env typeEnv) ast.TypeInfo {
	if len(node.Children) == 0 || node.Children[0].Type != "SYMBOL" {
		return ast.Layout(ast.Unknown)
	}
	head := node.Children[0].Value
	if info, ok := a.Functions[head]; ok {
		for i, arg := range node.Children[1:] {
			got := a.infer(arg, env)
			if i < len(info.Params) && known(got) && known(info.Params[i]) && !compatible(info.Params[i], got) {
				a.add(node, fmt.Sprintf("argument %d to %q has type %s, want %s", i+1, head, typeName(got), typeName(info.Params[i])))
			}
		}
		return info.Return
	}
	if isBinary(head) {
		left, right := a.inferChild(node, 1, env), a.inferChild(node, 2, env)
		if head == "and" || head == "or" {
			if known(left) && left.Kind != ast.Bool {
				a.add(node, fmt.Sprintf("%s requires bool operands, got %s", head, typeName(left)))
			}
			if known(right) && right.Kind != ast.Bool {
				a.add(node, fmt.Sprintf("%s requires bool operands, got %s", head, typeName(right)))
			}
			return ast.Layout(ast.Bool)
		}
		if head == "<" || head == ">" || head == "<=" || head == ">=" || head == "==" || head == "!=" || head == "=" {
			if known(left) && known(right) && !compatible(left, right) {
				a.add(node, fmt.Sprintf("%s compares incompatible types %s and %s", head, typeName(left), typeName(right)))
			}
			return ast.Layout(ast.Bool)
		}
		if known(left) && known(right) && !numeric(left) && !compatible(left, right) {
			a.add(node, fmt.Sprintf("%s cannot combine %s and %s", head, typeName(left), typeName(right)))
			return ast.Layout(ast.Unknown)
		}
		if left.Kind == ast.Float || right.Kind == ast.Float {
			return ast.Layout(ast.Float)
		}
		if left.Kind == ast.Int && right.Kind == ast.Int {
			return ast.Layout(ast.Int)
		}
		if left.Kind == ast.String && right.Kind == ast.String && head == "+" {
			return ast.Layout(ast.String)
		}
		return ast.Layout(ast.Unknown)
	}
	switch head {
	case "let":
		if len(node.Children) < 3 || node.Children[1].Type != "List" || len(node.Children[1].Children) < 2 {
			return ast.Layout(ast.Unknown)
		}
		binding := node.Children[1]
		value := a.infer(binding.Children[1], env)
		childEnv := cloneEnv(env)
		childEnv[binding.Children[0].Value] = value
		return a.infer(node.Children[2], childEnv)
	case "do":
		result := ast.Layout(ast.Void)
		for _, child := range node.Children[1:] {
			result = a.infer(child, env)
		}
		return result
	case "if":
		condition := a.inferChild(node, 1, env)
		if known(condition) && condition.Kind != ast.Bool {
			a.add(node, fmt.Sprintf("if condition must be bool, got %s", typeName(condition)))
		}
		thenType := a.inferChild(node, 2, env)
		if len(node.Children) == 4 {
			return join(thenType, a.inferChild(node, 3, env))
		}
		return thenType
	case "while":
		condition := a.inferChild(node, 1, env)
		if known(condition) && condition.Kind != ast.Bool {
			a.add(node, fmt.Sprintf("while condition must be bool, got %s", typeName(condition)))
		}
		a.inferChild(node, 2, env)
		return ast.Layout(ast.Void)
	case "try_let":
		if len(node.Children) < 4 || node.Children[1].Type != "List" || len(node.Children[1].Children) < 2 {
			return ast.Layout(ast.Unknown)
		}
		binding := node.Children[1]
		value := a.infer(binding.Children[1], env)
		catchEnv := cloneEnv(env)
		if node.Children[2].Type == "List" && len(node.Children[2].Children) >= 3 {
			catchEnv[node.Children[2].Children[1].Value] = ast.Layout(ast.Unknown)
		}
		catchType := a.inferChild(node.Children[2], 2, catchEnv)
		successEnv := cloneEnv(env)
		successEnv[binding.Children[0].Value] = value
		successType := a.infer(node.Children[3], successEnv)
		return join(catchType, successType)
	case "for":
		if len(node.Children) < 4 {
			return ast.Layout(ast.Void)
		}
		listType := a.infer(node.Children[2], env)
		loopEnv := cloneEnv(env)
		if listType.Element != nil {
			loopEnv[node.Children[1].Value] = *listType.Element
		} else {
			loopEnv[node.Children[1].Value] = ast.Layout(ast.Unknown)
		}
		a.infer(node.Children[1], loopEnv)
		a.infer(node.Children[3], loopEnv)
		return ast.Layout(ast.Void)
	case "spawn":
		if len(node.Children) >= 2 && node.Children[1].Type == "List" && len(node.Children[1].Children) >= 3 {
			a.infer(node.Children[1].Children[2], cloneEnv(env))
		}
		return ast.Layout(ast.Void)
	case "match":
		a.inferChild(node, 1, env)
		result := ast.Layout(ast.Unknown)
		for _, caseNode := range node.Children[2:] {
			if caseNode.Type != "List" || len(caseNode.Children) < 2 {
				continue
			}
			a.infer(caseNode.Children[0], env)
			result = join(result, a.infer(caseNode.Children[1], env))
		}
		return result
	case "return":
		return a.inferChild(node, 1, env)
	case "set":
		if len(node.Children) >= 3 {
			value := a.infer(node.Children[2], env)
			if current, ok := env[node.Children[1].Value]; ok && known(current) && known(value) && !compatible(current, value) {
				a.add(node, fmt.Sprintf("cannot assign %s to %s of type %s", typeName(value), node.Children[1].Value, typeName(current)))
			}
			env[node.Children[1].Value] = value
		}
		return ast.Layout(ast.Void)
	case "list":
		result := ast.Layout(ast.List)
		for _, child := range node.Children[1:] {
			value := a.infer(child, env)
			if result.Element == nil && known(value) {
				copy := value
				result.Element = &copy
			}
		}
		return result
	case "dict":
		result := ast.Layout(ast.Dict)
		for _, pair := range node.Children[1:] {
			if pair.Type == "List" && len(pair.Children) == 2 {
				a.infer(pair.Children[0], env)
				value := a.infer(pair.Children[1], env)
				if result.Element == nil && known(value) {
					copy := value
					result.Element = &copy
				}
			}
		}
		return result
	case "list_get":
		list := a.inferChild(node, 1, env)
		a.inferChild(node, 2, env)
		if list.Element != nil {
			return *list.Element
		}
		return ast.Layout(ast.Unknown)
	case "map_get":
		dict := a.inferChild(node, 1, env)
		a.inferChild(node, 2, env)
		if dict.Element != nil {
			return *dict.Element
		}
		return ast.Layout(ast.Unknown)
	case "to_int":
		a.inferChild(node, 1, env)
		return ast.Layout(ast.Int)
	case "to_float", "confidence":
		a.inferChild(node, 1, env)
		return ast.Layout(ast.Float)
	case "to_string", "bytes_to_string":
		a.inferChild(node, 1, env)
		return ast.Layout(ast.String)
	case "print", "append", "map_set", "map_delete", "sleep", "test":
		for _, child := range node.Children[1:] {
			a.infer(child, env)
		}
		return ast.Layout(ast.Void)
	case "call":
		for _, child := range node.Children[2:] {
			a.infer(child, env)
		}
		return ast.Layout(ast.Unknown)
	default:
		for _, child := range node.Children[1:] {
			a.infer(child, env)
		}
		return ast.Layout(ast.Unknown)
	}
}

func (a *Analysis) inferChild(node *ast.Node, index int, env typeEnv) ast.TypeInfo {
	if index >= 0 && index < len(node.Children) {
		return a.infer(node.Children[index], env)
	}
	return ast.Layout(ast.Unknown)
}

func (a *Analysis) add(node *ast.Node, reason string) {
	a.Diagnostics = append(a.Diagnostics, Diagnostic{Reason: reason, Line: node.Line, Column: node.Column})
}

func cloneEnv(env typeEnv) typeEnv {
	copy := typeEnv{}
	for name, value := range env {
		copy[name] = value
	}
	return copy
}

func typeFromName(name string) ast.TypeInfo {
	name = strings.TrimSpace(name)
	switch name {
	case "bool":
		return ast.Layout(ast.Bool)
	case "int", "int32", "int64":
		return ast.Layout(ast.Int)
	case "float", "float32", "float64":
		return ast.Layout(ast.Float)
	case "string":
		return ast.Layout(ast.String)
	case "[]byte":
		return ast.Layout(ast.Bytes)
	case "void":
		return ast.Layout(ast.Void)
	case "any":
		return ast.Layout(ast.Any)
	case "[]string":
		result := ast.Layout(ast.List)
		element := ast.Layout(ast.String)
		result.Element = &element
		result.Name = name
		return result
	case "map[string]string":
		result := ast.Layout(ast.Dict)
		result.Name = name
		return result
	default:
		return ast.TypeInfo{Kind: ast.Unknown, Name: name}
	}
}

func isBinary(head string) bool {
	return irBinops[head]
}

var irBinops = map[string]bool{
	"+": true, "-": true, "*": true, "/": true,
	"<": true, ">": true, "<=": true, ">=": true,
	"and": true, "or": true, "==": true, "!=": true, "=": true,
}

func known(value ast.TypeInfo) bool {
	return value.Kind != ast.Unknown && value.Kind != ast.Any && value.Kind != ""
}

func numeric(value ast.TypeInfo) bool {
	return value.Kind == ast.Int || value.Kind == ast.Float
}

func compatible(left, right ast.TypeInfo) bool {
	if !known(left) || !known(right) {
		return true
	}
	return left.Kind == right.Kind
}

func join(left, right ast.TypeInfo) ast.TypeInfo {
	if !known(left) {
		return right
	}
	if !known(right) || left.Kind == right.Kind {
		return left
	}
	return ast.Layout(ast.Any)
}

func typeName(value ast.TypeInfo) string {
	if value.Name != "" {
		return value.Name
	}
	return string(value.Kind)
}
