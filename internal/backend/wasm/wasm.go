package wasm

import (
	"encoding/binary"
	"fmt"
	"strconv"
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
	memory := ""
	data := ""
	if findStaticIntegerList(node.Children[1]) != nil {
		memory = "  (memory (export \"memory\") 1)\n"
		data = staticAggregateData(findStaticIntegerList(node.Children[1]))
	}
	code := fmt.Sprintf("(module\n%s%s  (func (export \"main\") (result %s)\n    %s\n  )\n)\n", memory, data, resultType, generateWasmExpression(node.Children[1]))
	if err := ValidateWAT(code); err != nil {
		ast.ReportError(fmt.Sprintf("invalid generated WAT: %v", err), node.Line, node.Column)
	}
	return code
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
		ops := wasmOps(valueType)
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
	case "list":
		return "(i32.const 8)"
	case "list_get":
		listPointer := generateWasmExpression(ir.Kids[0])
		index := generateWasmExpression(ir.Kids[1])
		byteOffset := fmt.Sprintf("(i32.mul (i32.wrap_i64 %s) (i32.const 8))", index)
		address := fmt.Sprintf("(i32.add %s %s)", listPointer, byteOffset)
		length := "(i64.load (i32.const 0))"
		return fmt.Sprintf("(if (result i64) (i64.lt_u %s %s) (then (i64.load %s)) (else (i64.const 0)))", index, length, address)
	case "to_float":
		child := ir.Kids[0]
		value := generateWasmExpression(child)
		if child.Inferred.Kind == ast.Int {
			return fmt.Sprintf("(f64.convert_s/i64 %s)", value)
		}
		return value
	case "to_int":
		child := ir.Kids[0]
		value := generateWasmExpression(child)
		if child.Inferred.Kind == ast.Float {
			return fmt.Sprintf("(i64.trunc_f64_s %s)", value)
		}
		return value
	default:
		// ast.ReportError(fmt.Sprintf("Wasm backend does not support %q", ir.Kind), source.Line, source.Column)
	}
	return ""
}

func staticAggregateData(node *ast.Node) string {
	if node.Type != "List" || len(node.Children) == 0 || node.Children[0].Value != "list" {
		return ""
	}
	var encoded strings.Builder
	var length [8]byte
	binary.LittleEndian.PutUint64(length[:], uint64(len(node.Children)-1))
	for _, byteValue := range length {
		encoded.WriteString(fmt.Sprintf("\\%02x", byteValue))
	}
	for _, child := range node.Children[1:] {
		value, err := strconv.ParseInt(child.Value, 10, 64)
		if err != nil {
			return ""
		}
		var bytes [8]byte
		binary.LittleEndian.PutUint64(bytes[:], uint64(value))
		for _, byteValue := range bytes {
			encoded.WriteString(fmt.Sprintf("\\%02x", byteValue))
		}
	}
	if encoded.Len() == 0 {
		return ""
	}
	return fmt.Sprintf("  (data (i32.const 0) \"%s\")\n", encoded.String())
}

func findStaticIntegerList(node *ast.Node) *ast.Node {
	if node == nil {
		return nil
	}
	if node.Type == "List" && len(node.Children) > 0 && node.Children[0].Type == "SYMBOL" && node.Children[0].Value == "list" {
		for _, child := range node.Children[1:] {
			if child.Type != "INT" {
				return nil
			}
		}
		return node
	}
	for _, child := range node.Children {
		if found := findStaticIntegerList(child); found != nil {
			return found
		}
	}
	return nil
}

// wasmType consumes the semantic layout selected by the checker. Zero's
// native int is 64-bit, while boolean control-flow values remain i32 in Wasm.
func wasmType(info ast.TypeInfo) string {
	return describeLayout(info).ValueType
}

func wasmOps(valueType string) map[string]string {
	if valueType == "f64" {
		return map[string]string{
			"+": "f64.add", "-": "f64.sub", "*": "f64.mul", "/": "f64.div",
			"<": "f64.lt", ">": "f64.gt", "<=": "f64.le", ">=": "f64.ge",
			"=": "f64.eq", "==": "f64.eq", "!=": "f64.ne",
		}
	}
	return map[string]string{
		"+": valueType + ".add", "-": valueType + ".sub", "*": valueType + ".mul", "/": valueType + ".div_s",
		"<": valueType + ".lt_s", ">": valueType + ".gt_s", "<=": valueType + ".le_s", ">=": valueType + ".ge_s",
		"=": valueType + ".eq", "==": valueType + ".eq", "!=": valueType + ".ne", "and": "i32.and", "or": "i32.or",
	}
}
