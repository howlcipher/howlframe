package wasm

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"zero/internal/ast"
	"zero/internal/ir"
)

type wasmGenerator struct {
	aggregates map[*ast.Node]aggregateRegion
	memorySize uint64
	root       *ast.Node
	strings    map[string]uint32
	stringList []string
}

type aggregateRegion struct {
	base uint32
	size uint64
}

func GenerateWasmCode(node *ast.Node) string {
	if node.Type != "List" || len(node.Children) != 2 {
		// ast.ReportError("wasm_app expects exactly one result expression", node.Line, node.Column)
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" || head.Value != "wasm_app" {
		// ast.ReportError("Expected wasm_app as root symbol", head.Line, head.Column)
	}

	resultType := wasmType(node.Children[1].Inferred)
	generator := newWasmGenerator(node.Children[1])
	memory := ""
	data := generator.dataSegments()
	if generator.memorySize > 0 {
		pages := (generator.memorySize + wasmPage - 1) / wasmPage
		memory = fmt.Sprintf("  (memory (export \"memory\") %d)\n", pages)
	}
	code := fmt.Sprintf("(module\n%s%s  (func (export \"main\") (result %s)\n    %s\n  )\n)\n", memory, data, resultType, generator.expression(node.Children[1]))
	if err := ValidateWAT(code); err != nil {
		ast.ReportError(fmt.Sprintf("invalid generated WAT: %v", err), node.Line, node.Column)
	}
	return code
}

func newWasmGenerator(root *ast.Node) *wasmGenerator {
	generator := &wasmGenerator{
		aggregates: make(map[*ast.Node]aggregateRegion),
		root:       root,
		strings:    make(map[string]uint32),
	}
	var allocate func(*ast.Node)
	allocate = func(node *ast.Node) {
		if node == nil {
			return
		}
		if node.Type == "STRING" {
			generator.allocateString(node.Value)
			return
		}
		if isStaticAggregate(node) {
			generator.alignMemory(8)
			size := staticAggregateSize(node)
			generator.aggregates[node] = aggregateRegion{base: uint32(generator.memorySize), size: size}
			generator.memorySize += size
			generator.alignMemory(8)
		}
		for _, child := range node.Children {
			allocate(child)
		}
	}
	allocate(root)
	return generator
}

func (g *wasmGenerator) alignMemory(alignment uint64) {
	if alignment == 0 {
		return
	}
	if remainder := g.memorySize % alignment; remainder != 0 {
		g.memorySize += alignment - remainder
	}
}

func (g *wasmGenerator) allocateString(value string) uint32 {
	if offset, ok := g.strings[value]; ok {
		return offset
	}
	offset := uint32(g.memorySize)
	g.strings[value] = offset
	g.stringList = append(g.stringList, value)
	g.memorySize += uint64(len(value) + 1)
	return offset
}

func (g *wasmGenerator) dataSegments() string {
	if len(g.aggregates) == 0 && len(g.stringList) == 0 {
		return ""
	}
	var data strings.Builder
	var emit func(*ast.Node)
	emit = func(node *ast.Node) {
		if node == nil {
			return
		}
		if region, ok := g.aggregates[node]; ok {
			data.WriteString(g.staticAggregateData(node, region.base))
		}
		for _, child := range node.Children {
			emit(child)
		}
	}
	emit(g.root)
	for _, value := range g.stringList {
		offset := g.strings[value]
		data.WriteString(fmt.Sprintf("  (data (i32.const %d) \"%s\")\n", offset, watEncodeBytes(append([]byte(value), 0))))
	}
	return data.String()
}

func (g *wasmGenerator) expression(node *ast.Node) string {
	if node.Type == "INT" {
		return fmt.Sprintf("(%s.const %s)", wasmType(node.Inferred), node.Value)
	}
	if node.Type == "STRING" {
		return fmt.Sprintf("(i32.const %d)", g.allocateString(node.Value))
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
	return g.emitIR(ir, node)
}

func (g *wasmGenerator) emitIR(ir *ir.IRNode, source *ast.Node) string {
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
		return fmt.Sprintf("(%s %s %s)", ops[ir.Op], g.expression(ir.Kids[0]), g.expression(ir.Kids[1]))
	case "if":
		if len(ir.Kids) != 3 {
			// ast.ReportError("Wasm backend requires if to include an else branch", source.Line, source.Column)
		}
		return fmt.Sprintf("(if (result %s) %s (then %s) (else %s))", wasmType(ir.Kids[1].Inferred), g.expression(ir.Kids[0]), g.expression(ir.Kids[1]), g.expression(ir.Kids[2]))
	case "do":
		if len(ir.Kids) == 0 {
			// ast.ReportError("Wasm backend requires do to contain a result expression", source.Line, source.Column)
		}
		parts := make([]string, 0, len(ir.Kids))
		for index, kid := range ir.Kids {
			expr := g.expression(kid)
			if index < len(ir.Kids)-1 {
				expr = fmt.Sprintf("(drop %s)", expr)
			}
			parts = append(parts, expr)
		}
		return fmt.Sprintf("(block (result %s) %s)", wasmType(ir.Kids[len(ir.Kids)-1].Inferred), strings.Join(parts, " "))
	case "return":
		return fmt.Sprintf("(return %s)", g.expression(ir.Kids[0]))
	case "list":
		region := g.aggregateRegion(source)
		dataPointer := region.base + 8
		if ir.Type.Element != nil && ir.Type.Element.Kind == ast.Int {
			var stores []string
			for index, kid := range ir.Kids {
				if kid.Type == "INT" {
					continue
				}
				stores = append(stores, fmt.Sprintf("(i64.store (i32.const %d) %s)", dataPointer+uint32(index*8), g.expression(kid)))
			}
			if len(stores) > 0 {
				return fmt.Sprintf("(block (result i32) %s (i32.const %d))", strings.Join(stores, " "), dataPointer)
			}
		}
		if ir.Type.Element != nil && ir.Type.Element.Kind == ast.String {
			var stores []string
			for index, kid := range ir.Kids {
				if kid.Type == "STRING" {
					continue
				}
				stores = append(stores, fmt.Sprintf("(i32.store (i32.const %d) %s)", dataPointer+uint32(index*4), g.expression(kid)))
			}
			if len(stores) > 0 {
				return fmt.Sprintf("(block (result i32) %s (i32.const %d))", strings.Join(stores, " "), dataPointer)
			}
		}
		return fmt.Sprintf("(i32.const %d)", dataPointer)
	case "list_get":
		region := g.aggregateRegion(ir.Kids[0])
		listPointer := g.expression(ir.Kids[0])
		index := g.expression(ir.Kids[1])
		length := fmt.Sprintf("(i64.load (i32.const %d))", region.base)
		if ir.Kids[0].Inferred.Element != nil && ir.Kids[0].Inferred.Element.Kind == ast.String {
			byteOffset := fmt.Sprintf("(i32.mul (i32.wrap_i64 %s) (i32.const 4))", index)
			address := fmt.Sprintf("(i32.add (i32.const %d) %s)", region.base+8, byteOffset)
			return fmt.Sprintf("(block (result i32) (drop %s) (if (result i32) (i64.lt_u %s %s) (then (i32.load %s)) (else (i32.const 0))))", listPointer, index, length, address)
		}
		byteOffset := fmt.Sprintf("(i32.mul (i32.wrap_i64 %s) (i32.const 8))", index)
		address := fmt.Sprintf("(i32.add %s %s)", listPointer, byteOffset)
		return fmt.Sprintf("(if (result i64) (i64.lt_u %s %s) (then (i64.load %s)) (else (i64.const 0)))", index, length, address)
	case "dict":
		region := g.aggregateRegion(source)
		dataPointer := region.base + 8
		if ir.Type.Element != nil && ir.Type.Element.Kind == ast.Int {
			var stores []string
			for _, pair := range ir.Kids {
				value := pair.Children[1]
				if value.Type == "INT" {
					continue
				}
				_, offset, ok := staticDictValue(source, pair.Children[0].Value)
				if !ok {
					continue
				}
				stores = append(stores, fmt.Sprintf("(i64.store (i32.const %d) %s)", region.base+offset, g.expression(value)))
			}
			if len(stores) > 0 {
				return fmt.Sprintf("(block (result i32) %s (i32.const %d))", strings.Join(stores, " "), dataPointer)
			}
		}
		if ir.Type.Element != nil && ir.Type.Element.Kind == ast.String {
			var stores []string
			for index, pair := range ir.Kids {
				value := pair.Children[1]
				if value.Type == "STRING" {
					continue
				}
				stores = append(stores, fmt.Sprintf("(i32.store (i32.const %d) %s)", dataPointer+uint32(index*8+4), g.expression(value)))
			}
			if len(stores) > 0 {
				return fmt.Sprintf("(block (result i32) %s (i32.const %d))", strings.Join(stores, " "), dataPointer)
			}
		}
		return fmt.Sprintf("(i32.const %d)", dataPointer)
	case "map_get":
		dict := ir.Kids[0]
		key := ir.Kids[1]
		if key.Type == "STRING" {
			if value, offset, ok := staticDictValue(dict, key.Value); ok {
				region := g.aggregateRegion(dict)
				if value.Type == "STRING" {
					return fmt.Sprintf("(i32.const %d)", g.stringOffset(value.Value))
				}
				dictPointer := g.expression(dict)
				if value.Inferred.Kind == ast.String {
					return fmt.Sprintf("(block (result i32) (drop %s) (i32.load (i32.const %d)))", dictPointer, region.base+offset)
				}
				return fmt.Sprintf("(block (result i64) (drop %s) (i64.load (i32.const %d)))", dictPointer, region.base+offset)
			}
		}
		if ir.Type.Kind == ast.String {
			return g.dynamicDictLookup(dict, key, "i32")
		}
		return g.dynamicDictLookup(dict, key, "i64")
	case "to_float":
		child := ir.Kids[0]
		value := g.expression(child)
		if child.Inferred.Kind == ast.Int {
			return fmt.Sprintf("(f64.convert_i64_s %s)", value)
		}
		return value
	case "to_int":
		child := ir.Kids[0]
		value := g.expression(child)
		if child.Inferred.Kind == ast.Float {
			return fmt.Sprintf("(i64.trunc_f64_s %s)", value)
		}
		return value
	default:
		// ast.ReportError(fmt.Sprintf("Wasm backend does not support %q", ir.Kind), source.Line, source.Column)
	}
	return ""
}

func (g *wasmGenerator) aggregateRegion(node *ast.Node) aggregateRegion {
	if region, ok := g.aggregates[node]; ok {
		return region
	}
	return aggregateRegion{}
}

func (g *wasmGenerator) stringOffset(value string) uint32 {
	if offset, ok := g.strings[value]; ok {
		return offset
	}
	return g.allocateString(value)
}

func (g *wasmGenerator) dynamicDictLookup(dict, key *ast.Node, resultType string) string {
	region := g.aggregateRegion(dict)
	keyExpr := g.expression(key)
	dictPointer := g.expression(dict)
	result := "(i64.const 0)"
	if resultType == "i32" {
		result = "(i32.const 0)"
	}
	for index := len(dict.Children) - 1; index >= 1; index-- {
		pair := dict.Children[index]
		if pair.Type != "List" || len(pair.Children) != 2 || pair.Children[0].Type != "STRING" {
			continue
		}
		keyPointer := g.stringOffset(pair.Children[0].Value)
		valueExpr := "(i64.const 0)"
		if resultType == "i32" {
			valueExpr = fmt.Sprintf("(i32.load (i32.const %d))", region.base+uint32(8+(index-1)*8+4))
		} else if _, offset, ok := staticDictValue(dict, pair.Children[0].Value); ok {
			valueExpr = fmt.Sprintf("(i64.load (i32.const %d))", region.base+offset)
		}
		result = fmt.Sprintf("(if (result %s) (i32.eq %s (i32.const %d)) (then %s) (else %s))", resultType, keyExpr, keyPointer, valueExpr, result)
	}
	return fmt.Sprintf("(block (result %s) (drop %s) %s)", resultType, dictPointer, result)
}

func (g *wasmGenerator) staticAggregateData(node *ast.Node, base uint32) string {
	if node.Type != "List" || len(node.Children) == 0 {
		return ""
	}
	if node.Children[0].Value == "dict" {
		return g.staticDictData(node, base)
	}
	if node.Children[0].Value != "list" {
		return ""
	}
	if node.Inferred.Element != nil && node.Inferred.Element.Kind == ast.String {
		payload := make([]byte, 8+4*(len(node.Children)-1))
		binary.LittleEndian.PutUint64(payload[:8], uint64(len(node.Children)-1))
		for index, child := range node.Children[1:] {
			if child.Type == "STRING" {
				binary.LittleEndian.PutUint32(payload[8+index*4:], g.stringOffset(child.Value))
			}
		}
		return fmt.Sprintf("  (data (i32.const %d) \"%s\")\n", base, watEncodeBytes(payload))
	}
	payload := make([]byte, 8+8*(len(node.Children)-1))
	binary.LittleEndian.PutUint64(payload[:8], uint64(len(node.Children)-1))
	for index, child := range node.Children[1:] {
		if child.Type != "INT" {
			continue
		}
		value, err := strconv.ParseInt(child.Value, 10, 64)
		if err == nil {
			binary.LittleEndian.PutUint64(payload[8+index*8:], uint64(value))
		}
	}
	return fmt.Sprintf("  (data (i32.const %d) \"%s\")\n", base, watEncodeBytes(payload))
}

func isStaticAggregate(node *ast.Node) bool {
	if node == nil {
		return false
	}
	if node.Type == "List" && len(node.Children) > 0 && node.Children[0].Type == "SYMBOL" && (node.Children[0].Value == "list" || node.Children[0].Value == "dict") {
		if node.Children[0].Value == "dict" {
			for _, pair := range node.Children[1:] {
				if pair.Type != "List" || len(pair.Children) != 2 || pair.Children[0].Type != "STRING" {
					return false
				}
			}
			return true
		}
		return true
	}
	return false
}

func (g *wasmGenerator) staticDictData(node *ast.Node, base uint32) string {
	valueStart := aggregatePayloadStart(8 + 8*(len(node.Children)-1))
	valueSlots := 0
	if node.Inferred.Element != nil && node.Inferred.Element.Kind == ast.Int {
		valueSlots = 8 * (len(node.Children) - 1)
	}
	payload := make([]byte, valueStart+valueSlots)
	binary.LittleEndian.PutUint64(payload[:8], uint64(len(node.Children)-1))
	for index, pair := range node.Children[1:] {
		value := pair.Children[1]
		binary.LittleEndian.PutUint32(payload[8+index*8:], g.stringOffset(pair.Children[0].Value))
		if node.Inferred.Element != nil && node.Inferred.Element.Kind == ast.String {
			if value.Type == "STRING" {
				binary.LittleEndian.PutUint32(payload[8+index*8+4:], g.stringOffset(value.Value))
			}
			continue
		}
		if value.Type == "STRING" {
			binary.LittleEndian.PutUint32(payload[8+index*8+4:], g.stringOffset(value.Value))
		} else {
			valueOffset := valueStart + index*8
			binary.LittleEndian.PutUint32(payload[8+index*8+4:], uint32(valueOffset))
			if value.Type == "INT" {
				parsed, err := strconv.ParseInt(value.Value, 10, 64)
				if err == nil {
					binary.LittleEndian.PutUint64(payload[valueOffset:], uint64(parsed))
				}
			}
		}
	}
	return fmt.Sprintf("  (data (i32.const %d) \"%s\")\n", base, watEncodeBytes(payload))
}

func staticDictValue(dict *ast.Node, key string) (*ast.Node, uint32, bool) {
	if dict.Type != "List" || len(dict.Children) == 0 || dict.Children[0].Value != "dict" {
		return nil, 0, false
	}
	valueStart := uint32(aggregatePayloadStart(8 + 8*(len(dict.Children)-1)))
	for index, pair := range dict.Children[1:] {
		if pair.Type != "List" || len(pair.Children) != 2 {
			return nil, 0, false
		}
		keyNode, valueNode := pair.Children[0], pair.Children[1]
		if keyNode.Value == key {
			if valueNode.Inferred.Kind == ast.String {
				return valueNode, uint32(8 + index*8 + 4), true
			}
			return valueNode, valueStart + uint32(index*8), true
		}
	}
	return nil, 0, false
}

func staticAggregateSize(node *ast.Node) uint64 {
	if node == nil || node.Type != "List" || len(node.Children) == 0 {
		return 0
	}
	if node.Children[0].Value == "list" {
		if node.Inferred.Element != nil && node.Inferred.Element.Kind == ast.String {
			return uint64(8 + 4*(len(node.Children)-1))
		}
		return uint64(8 + 8*(len(node.Children)-1))
	}
	if node.Children[0].Value == "dict" {
		size := uint64(aggregatePayloadStart(8 + 8*(len(node.Children)-1)))
		if node.Inferred.Element != nil && node.Inferred.Element.Kind == ast.Int {
			size += uint64(8 * (len(node.Children) - 1))
		}
		return size
	}
	return 0
}

func aggregatePayloadStart(tableEnd int) int {
	const minimumPayloadStart = 256
	if tableEnd <= minimumPayloadStart {
		return minimumPayloadStart
	}
	return (tableEnd + 7) &^ 7
}

func watEncodeBytes(data []byte) string {
	var encoded strings.Builder
	for _, byteValue := range data {
		encoded.WriteString(fmt.Sprintf("\\%02x", byteValue))
	}
	return encoded.String()
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
