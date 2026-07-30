package checker

import (
	"fmt"
	"strings"
	"zero/internal/ast"
	"zero/internal/ir"
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

// SchemaBridge records a source expression whose output is constrained to a
// declared Zero struct at a model or other structured-output boundary.
type SchemaBridge struct {
	Target     string
	Constraint ast.TypeInfo
}

// OptimizationCandidate is one labeled prompt or program variant recorded by
// an optimize_signature declaration.
type OptimizationCandidate struct {
	Label   string
	Payload string
}

// OptimizationSignature records compile-time optimization intent. Test
// commands and candidates are metadata only; the compiler never executes them.
type OptimizationSignature struct {
	Name       string
	Metric     string
	Tests      []string
	Candidates []OptimizationCandidate
	Line       int
	Column     int
	BodyType   ast.TypeInfo
}

// Analysis contains the inferred type/layout for every visited AST node.
// The AST nodes are also annotated so later lowering passes can consume the
// metadata without maintaining a second node-keyed lookup table.
type Analysis struct {
	Types                  map[*ast.Node]ast.TypeInfo
	Functions              map[string]FunctionInfo
	Structs                map[string]ast.TypeInfo
	Bridges                []SchemaBridge
	OptimizationSignatures []OptimizationSignature
	Diagnostics            []Diagnostic
}

type typeEnv map[string]ast.TypeInfo

// Analyze performs the non-fatal semantic pass. Dynamic or unresolved values
// remain Unknown; diagnostics are emitted only when both sides of a rule are
// known well enough to prove the program is invalid.
func Analyze(root *ast.Node) *Analysis {
	a := &Analysis{
		Types:     make(map[*ast.Node]ast.TypeInfo),
		Functions: make(map[string]FunctionInfo),
		Structs:   make(map[string]ast.TypeInfo),
	}
	if root == nil {
		return a
	}
	a.collectStructs(root)
	a.collectFunctions(root)
	a.inferRoot(root)
	return a
}

func (a *Analysis) collectStructs(node *ast.Node) {
	for stack := []*ast.Node{node}; len(stack) > 0; {
		node = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil {
			continue
		}
		if node.Type == "List" && len(node.Children) > 1 && node.Children[0].Type == "SYMBOL" && (node.Children[0].Value == "struct" || node.Children[0].Value == "schema") {
			name := node.Children[1].Value
			info := ast.Layout(ast.Struct)
			info.Name = name
			info.Fields = make(map[string]ast.TypeInfo)
			var fieldOrder []string
			for _, field := range node.Children[2:] {
				if field.Type != "List" || len(field.Children) < 2 {
					continue
				}
				start := 0
				if node.Children[0].Value == "schema" && len(field.Children) == 3 && field.Children[0].Value == "column" {
					start = 1
				}
				fieldName := field.Children[start].Value
				fieldType := a.resolveType(field.Children[start+1].Value)
				info.Fields[fieldName] = fieldType
				fieldOrder = append(fieldOrder, fieldName)
				if len(fieldName) > 0 {
					capitalized := strings.ToUpper(fieldName[:1]) + fieldName[1:]
					info.Fields[capitalized] = fieldType
				}
			}
			info.Size, info.Align, info.FieldOffsets = structLayout(info.Fields, fieldOrder)
			a.Structs[name] = info
		}
		if node.Type == "List" {
			for i := len(node.Children) - 1; i >= 0; i-- {
				stack = append(stack, node.Children[i])
			}
		}
	}
}

func (a *Analysis) collectFunctions(node *ast.Node) {
	for stack := []*ast.Node{node}; len(stack) > 0; {
		node = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil {
			continue
		}
		if node.Type == "List" && len(node.Children) > 0 {
			if node.Children[0].Type == "SYMBOL" && node.Children[0].Value == "defun" && len(node.Children) >= 4 {
				name := node.Children[1].Value
				params, ret := a.functionSignature(node)
				a.Functions[name] = FunctionInfo{Params: params, Return: ret}
			}
			for i := len(node.Children) - 1; i >= 0; i-- {
				stack = append(stack, node.Children[i])
			}
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
	params, ret := a.functionSignature(node)
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

func (a *Analysis) functionSignature(node *ast.Node) ([]ast.TypeInfo, ast.TypeInfo) {
	args := node.Children[2]
	params := make([]ast.TypeInfo, 0, len(args.Children))
	for _, arg := range args.Children {
		if arg.Type == "List" && len(arg.Children) >= 2 {
			params = append(params, a.resolveType(arg.Children[1].Value))
		} else {
			params = append(params, ast.Layout(ast.String))
		}
	}
	ret := ast.Layout(ast.String)
	bodyStart := 3
	if len(node.Children) > 4 && node.Children[3].Type == "SYMBOL" {
		ret = a.resolveType(node.Children[3].Value)
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
					ret = a.resolveType(cfg.Children[2].Value)
				} else {
					for index, arg := range args.Children {
						name := arg.Value
						if arg.Type == "List" && len(arg.Children) > 0 {
							name = arg.Children[0].Value
						}
						if name == cfg.Children[1].Value && index < len(params) {
							params[index] = a.resolveType(cfg.Children[2].Value)
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
					ret = a.resolveType(pair.Children[1].Value)
					continue
				}
				for index, arg := range args.Children {
					name := arg.Value
					if arg.Type == "List" && len(arg.Children) > 0 {
						name = arg.Children[0].Value
					}
					if name == pair.Children[0].Value && index < len(params) {
						params[index] = a.resolveType(pair.Children[1].Value)
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
			if result.Kind == "" && strings.Contains(node.Value, ".") {
				parts := strings.SplitN(node.Value, ".", 2)
				base := env[parts[0]]
				if base.Fields != nil {
					result = base.Fields[parts[1]]
				}
			}
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
		a.checkCall(node, head, node.Children[1:], info, env)
		return info.Return
	}
	if reason, shared := ir.ValidateShared(node); shared && reason != "" {
		a.add(node, reason)
		return ast.Layout(ast.Unknown)
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
		if known(left) && known(right) {
			if head == "+" && left.Kind == ast.String && right.Kind == ast.String {
				return ast.Layout(ast.String)
			}
			if !numeric(left) || !numeric(right) {
				a.add(node, fmt.Sprintf("%s requires numeric operands, got %s and %s", head, typeName(left), typeName(right)))
				return ast.Layout(ast.Unknown)
			}
			if left.Kind != right.Kind {
				a.add(node, fmt.Sprintf("%s requires matching numeric types, got %s and %s", head, typeName(left), typeName(right)))
				return ast.Layout(ast.Unknown)
			}
		}
		if left.Kind == ast.Float || right.Kind == ast.Float {
			return ast.Layout(ast.Float)
		}
		if left.Kind == ast.Int && right.Kind == ast.Int {
			return ast.Layout(ast.Int)
		}
		return ast.Layout(ast.Unknown)
	}
	switch head {
	case "let":
		return a.inferLetChain(node, env)
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
			elseType := a.inferChild(node, 3, env)
			if known(thenType) && known(elseType) && !compatible(thenType, elseType) {
				a.add(node, fmt.Sprintf("if branches have incompatible types %s and %s", typeName(thenType), typeName(elseType)))
			}
			return join(thenType, elseType)
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
		for index, child := range node.Children[1:] {
			value := a.infer(child, env)
			if result.Element == nil && known(value) {
				copy := value
				result.Element = &copy
			} else if result.Element != nil && known(value) && !compatible(*result.Element, value) {
				a.add(node, fmt.Sprintf("list element %d has type %s, want %s", index+1, typeName(value), typeName(*result.Element)))
			}
		}
		return result
	case "dict":
		result := ast.Layout(ast.Dict)
		keyType := ast.Layout(ast.String)
		result.Key = &keyType
		for index, pair := range node.Children[1:] {
			if pair.Type == "List" && len(pair.Children) == 2 {
				key := a.infer(pair.Children[0], env)
				if known(key) && key.Kind != ast.String {
					a.add(node, fmt.Sprintf("dict key %d has type %s, want string", index+1, typeName(key)))
				}
				value := a.infer(pair.Children[1], env)
				if result.Element == nil && known(value) {
					copy := value
					result.Element = &copy
				} else if result.Element != nil && known(value) && !compatible(*result.Element, value) {
					a.add(node, fmt.Sprintf("dict value %d has type %s, want %s", index+1, typeName(value), typeName(*result.Element)))
				}
			}
		}
		return result
	case "list_get":
		list := a.inferChild(node, 1, env)
		index := a.inferChild(node, 2, env)
		if known(list) && list.Kind != ast.List {
			a.add(node, fmt.Sprintf("list_get target must be list, got %s", typeName(list)))
		}
		if known(index) && index.Kind != ast.Int {
			a.add(node, fmt.Sprintf("list_get index must be int, got %s", typeName(index)))
		}
		if list.Element != nil {
			return *list.Element
		}
		return ast.Layout(ast.Unknown)
	case "map_get":
		dict := a.inferChild(node, 1, env)
		key := a.inferChild(node, 2, env)
		if known(dict) && dict.Kind != ast.Dict {
			a.add(node, fmt.Sprintf("map_get target must be dict, got %s", typeName(dict)))
		}
		if known(key) && key.Kind != ast.String {
			a.add(node, fmt.Sprintf("map_get key must be string, got %s", typeName(key)))
		}
		if dict.Element != nil {
			return *dict.Element
		}
		return ast.Layout(ast.Unknown)
	case "append":
		list := a.inferChild(node, 1, env)
		item := a.inferChild(node, 2, env)
		if known(list) && list.Kind != ast.List {
			a.add(node, fmt.Sprintf("append target must be list, got %s", typeName(list)))
		} else if list.Element != nil && known(item) && !compatible(*list.Element, item) {
			a.add(node, fmt.Sprintf("append item has type %s, want %s", typeName(item), typeName(*list.Element)))
		}
		return ast.Layout(ast.Void)
	case "map_set":
		dict := a.inferChild(node, 1, env)
		key := a.inferChild(node, 2, env)
		value := a.inferChild(node, 3, env)
		if known(dict) && dict.Kind != ast.Dict {
			a.add(node, fmt.Sprintf("map_set target must be dict, got %s", typeName(dict)))
		}
		if known(key) && key.Kind != ast.String {
			a.add(node, fmt.Sprintf("map_set key must be string, got %s", typeName(key)))
		}
		if dict.Element != nil && known(value) && !compatible(*dict.Element, value) {
			a.add(node, fmt.Sprintf("map_set value has type %s, want %s", typeName(value), typeName(*dict.Element)))
		}
		return ast.Layout(ast.Void)
	case "map_delete":
		dict := a.inferChild(node, 1, env)
		key := a.inferChild(node, 2, env)
		if known(dict) && dict.Kind != ast.Dict {
			a.add(node, fmt.Sprintf("map_delete target must be dict, got %s", typeName(dict)))
		}
		if known(key) && key.Kind != ast.String {
			a.add(node, fmt.Sprintf("map_delete key must be string, got %s", typeName(key)))
		}
		return ast.Layout(ast.Void)
	case "to_int":
		a.inferChild(node, 1, env)
		return ast.Layout(ast.Int)
	case "to_float", "confidence":
		a.inferChild(node, 1, env)
		return ast.Layout(ast.Float)
	case "to_string", "bytes_to_string":
		a.inferChild(node, 1, env)
		return ast.Layout(ast.String)
	case "parse_json":
		a.inferChild(node, 2, env)
		if len(node.Children) >= 2 {
			if result, ok := a.Structs[node.Children[1].Value]; ok {
				return result
			}
		}
		return ast.Layout(ast.Unknown)
	case "schema_bridge":
		if len(node.Children) != 3 {
			for _, child := range node.Children[1:] {
				a.infer(child, env)
			}
			a.add(node, fmt.Sprintf("schema_bridge expects a struct name and source expression, got %d arguments", len(node.Children)-1))
			return ast.Layout(ast.Unknown)
		}

		// Infer the source even when the target is invalid so errors nested in
		// the boundary remain visible to the author.
		a.infer(node.Children[2], env)
		target := node.Children[1]
		if target.Type != "SYMBOL" {
			a.add(target, "schema_bridge target must be a declared struct name")
			return ast.Layout(ast.Unknown)
		}
		constraint, ok := a.Structs[target.Value]
		if !ok {
			a.add(target, fmt.Sprintf("schema_bridge target %q is not a declared struct", target.Value))
			return ast.Layout(ast.Unknown)
		}
		a.Bridges = append(a.Bridges, SchemaBridge{
			Target:     target.Value,
			Constraint: constraint,
		})
		return constraint
	case "optimize_signature":
		return a.inferOptimizationSignature(node, env)
	case "env":
		a.inferChild(node, 1, env)
		return ast.Layout(ast.String)
	case "read_file":
		a.inferChild(node, 1, env)
		return ast.Layout(ast.Bytes)
	case "fuzzy_cast":
		a.inferChild(node, 2, env)
		if len(node.Children) >= 2 {
			if result, ok := a.Structs[node.Children[1].Value]; ok {
				return result
			}
		}
		return ast.Layout(ast.Unknown)
	case "write_file", "mkdir", "exec":
		for _, child := range node.Children[1:] {
			a.infer(child, env)
		}
		return ast.Layout(ast.Void)
	case "print", "sleep", "test":
		for _, child := range node.Children[1:] {
			a.infer(child, env)
		}
		return ast.Layout(ast.Void)
	case "call":
		for _, child := range node.Children[2:] {
			a.infer(child, env)
		}
		if len(node.Children) >= 2 {
			name := node.Children[1].Value
			if info, ok := a.Functions[name]; ok {
				a.checkCall(node, name, node.Children[2:], info, env)
				return info.Return
			}
		}
		return ast.Layout(ast.Unknown)
	default:
		for _, child := range node.Children[1:] {
			a.infer(child, env)
		}
		return ast.Layout(ast.Unknown)
	}
}

func (a *Analysis) inferOptimizationSignature(node *ast.Node, env typeEnv) ast.TypeInfo {
	if len(node.Children) < 6 {
		for _, child := range node.Children[1:] {
			a.infer(child, env)
		}
		a.add(node, "optimize_signature expects a name, metric, one or more tests, one or more candidates, and a body")
		return ast.Layout(ast.Unknown)
	}

	valid := true
	nameNode := node.Children[1]
	if nameNode.Type != "SYMBOL" {
		a.add(nameNode, "optimize_signature name must be a symbol")
		valid = false
	}

	metricNode := node.Children[2]
	metric := ""
	if !isMetadataForm(metricNode, "metric", 2) || metricNode.Children[1].Type != "STRING" {
		a.add(metricNode, `optimize_signature metric expects (metric "name")`)
		valid = false
	} else {
		metric = metricNode.Children[1].Value
	}

	index := 3
	tests := []string{}
	for index < len(node.Children)-1 && metadataHead(node.Children[index]) == "test" {
		testNode := node.Children[index]
		if !isMetadataForm(testNode, "test", 2) || testNode.Children[1].Type != "STRING" {
			a.add(testNode, `optimize_signature test expects (test "command")`)
			valid = false
		} else {
			tests = append(tests, testNode.Children[1].Value)
		}
		index++
	}
	if len(tests) == 0 {
		a.add(node, `optimize_signature requires at least one (test "command") form`)
		valid = false
	}

	candidates := []OptimizationCandidate{}
	for index < len(node.Children)-1 && metadataHead(node.Children[index]) == "candidate" {
		candidateNode := node.Children[index]
		if !isMetadataForm(candidateNode, "candidate", 3) {
			a.add(candidateNode, `optimize_signature candidate expects (candidate "label" "payload")`)
			valid = false
		} else {
			labelNode := candidateNode.Children[1]
			payloadNode := candidateNode.Children[2]
			if labelNode.Type != "STRING" {
				a.add(labelNode, "optimize_signature candidate label must be a string")
				valid = false
			}
			if payloadNode.Type != "STRING" {
				a.add(payloadNode, "optimize_signature candidate payload must be a string")
				valid = false
			}
			if labelNode.Type == "STRING" && payloadNode.Type == "STRING" {
				candidates = append(candidates, OptimizationCandidate{
					Label:   labelNode.Value,
					Payload: payloadNode.Value,
				})
			}
		}
		index++
	}
	if len(candidates) == 0 {
		a.add(node, `optimize_signature requires at least one (candidate "label" "payload") form`)
		valid = false
	}
	if index != len(node.Children)-1 {
		a.add(node.Children[index], "optimize_signature metadata must be metric, then tests, then candidates, followed by one body")
		valid = false
	}

	body := node.Children[len(node.Children)-1]
	bodyType := a.infer(body, env)
	if valid {
		a.OptimizationSignatures = append(a.OptimizationSignatures, OptimizationSignature{
			Name:       nameNode.Value,
			Metric:     metric,
			Tests:      tests,
			Candidates: candidates,
			Line:       node.Line,
			Column:     node.Column,
			BodyType:   bodyType,
		})
	}
	return bodyType
}

func metadataHead(node *ast.Node) string {
	if node == nil || node.Type != "List" || len(node.Children) == 0 ||
		node.Children[0].Type != "SYMBOL" {
		return ""
	}
	return node.Children[0].Value
}

func isMetadataForm(node *ast.Node, head string, length int) bool {
	return metadataHead(node) == head && len(node.Children) == length
}

func (a *Analysis) inferLetChain(node *ast.Node, env typeEnv) ast.TypeInfo {
	bindings, body := ast.LetChain(node)
	if len(bindings) == 0 {
		return ast.Layout(ast.Unknown)
	}
	chain := make([]*ast.Node, 0, len(bindings))
	current := node
	childEnv := env
	for _, binding := range bindings {
		value := a.infer(binding.Children[1], childEnv)
		childEnv = cloneEnv(childEnv)
		childEnv[binding.Children[0].Value] = value
		chain = append(chain, current)
		current = current.Children[2]
	}
	result := a.infer(body, childEnv)
	for _, letNode := range chain {
		a.Types[letNode] = result
		letNode.Inferred = result
	}
	return result
}

func (a *Analysis) checkCall(node *ast.Node, name string, args []*ast.Node, info FunctionInfo, env typeEnv) {
	if len(args) != len(info.Params) {
		noun := "arguments"
		if len(info.Params) == 1 {
			noun = "argument"
		}
		a.add(node, fmt.Sprintf("function %q expects %d %s, got %d", name, len(info.Params), noun, len(args)))
	}
	for i, arg := range args {
		got := a.infer(arg, env)
		if i < len(info.Params) && known(got) && known(info.Params[i]) && !compatible(info.Params[i], got) {
			a.add(node, fmt.Sprintf("argument %d to %q has type %s, want %s", i+1, name, typeName(got), typeName(info.Params[i])))
		}
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
		key := ast.Layout(ast.String)
		element := ast.Layout(ast.String)
		result.Key = &key
		result.Element = &element
		result.Name = name
		return result
	default:
		return ast.TypeInfo{Kind: ast.Unknown, Name: name}
	}
}

func (a *Analysis) resolveType(name string) ast.TypeInfo {
	if result := typeFromName(name); result.Kind != ast.Unknown {
		return result
	}
	if result, ok := a.Structs[name]; ok {
		return result
	}
	return ast.TypeInfo{Kind: ast.Unknown, Name: name}
}

func structLayout(fields map[string]ast.TypeInfo, order []string) (uint64, uint64, map[string]uint64) {
	var size, alignment uint64 = 0, 1
	offsets := make(map[string]uint64)
	for _, name := range order {
		field := fields[name]
		fieldAlign := field.Align
		if fieldAlign == 0 {
			fieldAlign = 1
		}
		if remainder := size % fieldAlign; remainder != 0 {
			size += fieldAlign - remainder
		}
		offsets[name] = size
		if len(name) > 0 {
			offsets[strings.ToUpper(name[:1])+name[1:]] = size
		}
		size += field.Size
		if fieldAlign > alignment {
			alignment = fieldAlign
		}
	}
	if remainder := size % alignment; remainder != 0 {
		size += alignment - remainder
	}
	return size, alignment, offsets
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
	if left.Kind == ast.Struct || right.Kind == ast.Struct {
		return left.Kind == right.Kind && left.Name == right.Name
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
