package checker

import (
	"fmt"

	"github.com/howlcipher/howlframe/internal/ast"
)

// builtinSignature records the statically known argument kinds and result of
// a builtin whose accepted input types match what the VM enforces at runtime.
// A zero Kind accepts anything. Arguments whose type is unknown are never
// rejected, so the table only reports provable mismatches.
type builtinSignature struct {
	Params []ast.ValueKind
	Result ast.TypeInfo
}

var builtinSignatures = map[string]builtinSignature{
	"str_split":   {Params: []ast.ValueKind{ast.String, ast.String}, Result: stringList()},
	"str_join":    {Params: []ast.ValueKind{ast.List, ast.String}, Result: ast.Layout(ast.String)},
	"regex_match": {Params: []ast.ValueKind{ast.String, ast.String}, Result: ast.Layout(ast.Bool)},
	"list_len":    {Params: []ast.ValueKind{ast.List}, Result: ast.Layout(ast.Int)},
}

func stringList() ast.TypeInfo {
	result := ast.Layout(ast.List)
	element := ast.Layout(ast.String)
	result.Element = &element
	return result
}

// checkBuiltin validates arity and statically known argument kinds against the
// builtin's signature and returns its result type.
func (a *Analysis) checkBuiltin(node *ast.Node, name string, sig builtinSignature, env typeEnv) ast.TypeInfo {
	args := node.Children[1:]
	if len(args) != len(sig.Params) {
		a.add(node, fmt.Sprintf("%s expects %d arguments, got %d", name, len(sig.Params), len(args)))
	}
	for i, arg := range args {
		got := a.requireValue(arg, a.infer(arg, env), fmt.Sprintf("argument %d to %s", i+1, name))
		if i < len(sig.Params) && sig.Params[i] != "" && known(got) && got.Kind != sig.Params[i] {
			a.add(arg, fmt.Sprintf("%s argument %d must be %s, got %s", name, i+1, sig.Params[i], typeName(got)))
		}
	}
	return sig.Result
}
