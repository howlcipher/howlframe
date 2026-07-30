package wasm

import (
	"fmt"
	"strings"
	"zero/internal/ast"
	"zero/internal/ir"
)

func GenerateWasmCode(node *ast.Node) string {
	if node.Type != "List" || len(node.Children) != 2 {
		// ast.ReportError("wasm_app expects exactly one result expression", node.Line, node.Column)
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" || head.Value != "wasm_app" {
		// ast.ReportError("Expected wasm_app as root symbol", head.Line, head.Column)
	}

	resultType := wasmType(node.Children[1].Inferred)
	return fmt.Sprintf("(module\n  (func (export \"main\") (result %s)\n    %s\n  )\n)\n", resultType, generateWasmExpression(node.Children[1]))
}

func generateWasmExpression(node *ast.Node) string {
	if node.Type == "INT" {
		return fmt.Sprintf("(%s.const %s)", wasmType(node.Inferred), node.Value)
	}
	if node.Type == "SYMBOL" {
		switch node.Value {
		case "true":
			return "(i32.const 1)"
		case "false":
			return "(i32.const 0)"
		default:
			// ast.ReportError(fmt.Sprintf("Wasm backend does not support symbol %q", node.Value), node.Line, node.Column)
		}
	}
	if node.Type != "List" {
		// ast.ReportError(fmt.Sprintf("Wasm backend does not support %s literals", node.Type), node.Line, node.Column)
	}

	ir, ok := ir.LowerShared(node)
	if !ok {
		if len(node.Children) == 0 {
			// ast.ReportError("Wasm backend does not support an empty expression", node.Line, node.Column)
		}
		// ast.ReportError(fmt.Sprintf("Wasm backend does not support %q", node.Children[0].Value), node.Line, node.Column)
	}
	return EmitWasmIR(ir, node)
}

func EmitWasmIR(ir *ir.IRNode, source *ast.Node) string {
	switch ir.Kind {
	case "binop":
		if len(ir.Kids) != 2 {
			// ast.ReportError(fmt.Sprintf("%s expects 2 arguments", ir.Op), source.Line, source.Column)
		}
		valueType := wasmType(ir.Kids[0].Inferred)
		if ir.Op == "and" || ir.Op == "or" {
			valueType = "i32"
		}
		ops := map[string]string{
			"+": valueType + ".add", "-": valueType + ".sub", "*": valueType + ".mul", "/": valueType + ".div_s",
			"<": valueType + ".lt_s", ">": valueType + ".gt_s", "<=": valueType + ".le_s", ">=": valueType + ".ge_s",
			"=": valueType + ".eq", "==": valueType + ".eq", "!=": valueType + ".ne", "and": "i32.and", "or": "i32.or",
		}
		return fmt.Sprintf("(%s %s %s)", ops[ir.Op], generateWasmExpression(ir.Kids[0]), generateWasmExpression(ir.Kids[1]))
	case "if":
		if len(ir.Kids) != 3 {
			// ast.ReportError("Wasm backend requires if to include an else branch", source.Line, source.Column)
		}
		return fmt.Sprintf("(if (result %s) %s (then %s) (else %s))", wasmType(ir.Kids[1].Inferred), generateWasmExpression(ir.Kids[0]), generateWasmExpression(ir.Kids[1]), generateWasmExpression(ir.Kids[2]))
	case "do":
		if len(ir.Kids) == 0 {
			// ast.ReportError("Wasm backend requires do to contain a result expression", source.Line, source.Column)
		}
		parts := make([]string, 0, len(ir.Kids))
		for index, kid := range ir.Kids {
			expr := generateWasmExpression(kid)
			if index < len(ir.Kids)-1 {
				expr = fmt.Sprintf("(drop %s)", expr)
			}
			parts = append(parts, expr)
		}
		return fmt.Sprintf("(block (result %s) %s)", wasmType(ir.Kids[len(ir.Kids)-1].Inferred), strings.Join(parts, " "))
	case "return":
		return fmt.Sprintf("(return %s)", generateWasmExpression(ir.Kids[0]))
	default:
		// ast.ReportError(fmt.Sprintf("Wasm backend does not support %q", ir.Kind), source.Line, source.Column)
	}
	return ""
}

// wasmType consumes the semantic layout selected by the checker. Zero's
// native int is 64-bit, while boolean control-flow values remain i32 in Wasm.
func wasmType(info ast.TypeInfo) string {
	switch info.Kind {
	case ast.Int:
		return "i64"
	case ast.Bool:
		return "i32"
	default:
		return "i32"
	}
}
