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

	return fmt.Sprintf("(module\n  (func (export \"main\") (result i32)\n    %s\n  )\n)\n", generateWasmExpression(node.Children[1]))
}

func generateWasmExpression(node *ast.Node) string {
	if node.Type == "INT" {
		return fmt.Sprintf("(i32.const %s)", node.Value)
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
		ops := map[string]string{
			"+": "i32.add", "-": "i32.sub", "*": "i32.mul", "/": "i32.div_s",
			"<": "i32.lt_s", ">": "i32.gt_s", "<=": "i32.le_s", ">=": "i32.ge_s",
			"=": "i32.eq", "==": "i32.eq", "!=": "i32.ne", "and": "i32.and", "or": "i32.or",
		}
		return fmt.Sprintf("(%s %s %s)", ops[ir.Op], generateWasmExpression(ir.Kids[0]), generateWasmExpression(ir.Kids[1]))
	case "if":
		if len(ir.Kids) != 3 {
			// ast.ReportError("Wasm backend requires if to include an else branch", source.Line, source.Column)
		}
		return fmt.Sprintf("(if (result i32) %s (then %s) (else %s))", generateWasmExpression(ir.Kids[0]), generateWasmExpression(ir.Kids[1]), generateWasmExpression(ir.Kids[2]))
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
		return fmt.Sprintf("(block (result i32) %s)", strings.Join(parts, " "))
	case "return":
		return fmt.Sprintf("(return %s)", generateWasmExpression(ir.Kids[0]))
	default:
		// ast.ReportError(fmt.Sprintf("Wasm backend does not support %q", ir.Kind), source.Line, source.Column)
	}
	return ""
}
