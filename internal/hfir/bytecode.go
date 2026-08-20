package hfir

import (
	"fmt"
	"strconv"

	"github.com/howlcipher/howlframe/internal/bytecode"
)

// BytecodeUnsupportedCode is returned when a semantic HFIR graph uses a node
// outside the deliberately small executable Phase-1 contract. Callers must
// treat it as a hard failure; LowerToBytecode never returns partial bytecode.
const BytecodeUnsupportedCode = "HFIR_BYTECODE_UNSUPPORTED"

// LowerToBytecode converts the Phase-1 semantic HFIR subset directly into a
// BCProgram. It accepts only a graph: it neither reconstructs an AST nor calls
// the legacy AST bytecode compiler.
func LowerToBytecode(graph *Graph) (*bytecode.BCProgram, []Diagnostic) {
	compiler := &bytecodeLowerer{
		graph:     graph,
		compiling: make(map[NodeID]bool),
		prog: &bytecode.BCProgram{
			Version:   1,
			Functions: make(map[string]*bytecode.BCFunction),
		},
	}
	if graph == nil {
		return nil, []Diagnostic{compiler.diagnostic(nil, "HFIR graph is required")}
	}
	entry := graph.NodeByID(graph.EntryNode)
	if entry == nil {
		return nil, []Diagnostic{compiler.diagnostic(nil, "HFIR entry node is missing")}
	}
	insts, diagnostic := compiler.compile(entry)
	if diagnostic != nil {
		return nil, []Diagnostic{*diagnostic}
	}
	compiler.prog.Main = insts
	if !compiler.prog.AttachTrustedMainOrigins() || !compiler.prog.BindLocalizationIdentity(GraphHash(graph)) {
		return nil, []Diagnostic{compiler.diagnostic(entry, "direct HFIR lowering produced incomplete instruction provenance")}
	}
	return compiler.prog, nil
}

type bytecodeLowerer struct {
	graph     *Graph
	prog      *bytecode.BCProgram
	compiling map[NodeID]bool
}

func (c *bytecodeLowerer) compile(node *Node) (instructions []bytecode.BCInstruction, diagnostic *Diagnostic) {
	if node == nil {
		value := c.diagnostic(nil, "HFIR node is missing")
		return nil, &value
	}
	defer func() {
		if diagnostic != nil {
			return
		}
		for index := range instructions {
			if instructions[index].OpString == "" {
				continue
			}
			// Children return with their own marker. Instructions emitted by this
			// node receive its canonical semantic identity here.
			if !bytecode.HasSemanticOrigin(instructions[index]) {
				bytecode.SetSemanticOrigin(&instructions[index], string(node.ID))
			}
		}
	}()
	if c.compiling[node.ID] {
		value := c.diagnostic(node, "cyclic data dependency cannot be lowered to bytecode")
		return nil, &value
	}
	c.compiling[node.ID] = true
	defer delete(c.compiling, node.ID)
	children, diagnostic := c.children(node)
	if diagnostic != nil {
		return nil, diagnostic
	}
	compileChild := func(index int) ([]bytecode.BCInstruction, *Diagnostic) {
		return c.compile(children[index])
	}
	compileAll := func() ([]bytecode.BCInstruction, *Diagnostic) {
		var insts []bytecode.BCInstruction
		for _, child := range children {
			childInsts, childDiagnostic := c.compile(child)
			if childDiagnostic != nil {
				return nil, childDiagnostic
			}
			insts = append(insts, childInsts...)
		}
		return insts, nil
	}

	switch node.Kind {
	case "program", "sequence":
		if !allEdgesNamed(node, "body") {
			diagnostic := c.diagnostic(node, node.Kind+" requires body edges")
			return nil, &diagnostic
		}
		return compileAll()
	case "const":
		value, err := literalValue(node)
		if err != nil {
			diagnostic := c.diagnostic(node, err.Error())
			return nil, &diagnostic
		}
		return []bytecode.BCInstruction{instruction(bytecode.OpLoadConst, "LOAD_CONST", func(inst *bytecode.BCInstruction) { inst.ValueOperand = value })}, nil
	case "symbol":
		if node.Value == "" {
			diagnostic := c.diagnostic(node, "symbol node has no name")
			return nil, &diagnostic
		}
		return []bytecode.BCInstruction{instruction(bytecode.OpLoadVar, "LOAD_VAR", func(inst *bytecode.BCInstruction) { inst.StringOperand = node.Value })}, nil
	case "let":
		if node.Value == "" || len(children) != 2 || node.DataInputs[0].Name != "value" || node.DataInputs[1].Name != "body" {
			diagnostic := c.diagnostic(node, "let requires a name, value, and body")
			return nil, &diagnostic
		}
		value, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		body, childDiagnostic := compileChild(1)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		value = append(value, instruction(bytecode.OpStoreVar, "STORE_VAR", func(inst *bytecode.BCInstruction) { inst.StringOperand = node.Value }))
		return append(value, body...), nil
	case "set":
		if node.Value == "" || len(children) != 1 || node.DataInputs[0].Name != "value" {
			diagnostic := c.diagnostic(node, "set requires a name and value")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpSetVar, "SET_VAR", func(inst *bytecode.BCInstruction) { inst.StringOperand = node.Value })), nil
	case "if":
		if len(children) != 2 && len(children) != 3 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "then" || len(children) == 3 && node.DataInputs[2].Name != "else" {
			diagnostic := c.diagnostic(node, "if requires condition, then, and optional else")
			return nil, &diagnostic
		}
		condition, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		thenInsts, childDiagnostic := compileChild(1)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		if len(children) == 2 {
			condition = append(condition, instruction(bytecode.OpJumpIfFalse, "JUMP_IF_FALSE", func(inst *bytecode.BCInstruction) { inst.IntOperand = int64(len(thenInsts) + 1) }))
			return append(condition, thenInsts...), nil
		}
		elseInsts, childDiagnostic := compileChild(2)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		condition = append(condition, instruction(bytecode.OpJumpIfFalse, "JUMP_IF_FALSE", func(inst *bytecode.BCInstruction) { inst.IntOperand = int64(len(thenInsts) + 2) }))
		condition = append(condition, thenInsts...)
		condition = append(condition, instruction(bytecode.OpJump, "JUMP", func(inst *bytecode.BCInstruction) { inst.IntOperand = int64(len(elseInsts) + 1) }))
		return append(condition, elseInsts...), nil
	case "binary":
		if len(children) != 2 || node.DataInputs[0].Name != "left" || node.DataInputs[1].Name != "right" || node.Value == "" {
			diagnostic := c.diagnostic(node, "binary operation requires operator, left, and right")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileAll()
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpBinop, "BINOP", func(inst *bytecode.BCInstruction) { inst.StringOperand = node.Value })), nil
	case "list":
		if !allEdgesNamed(node, "item") {
			diagnostic := c.diagnostic(node, "list requires item edges")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileAll()
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpMakeList, "MAKE_LIST", func(inst *bytecode.BCInstruction) { inst.IntOperand = int64(len(children)) })), nil
	case "dict":
		if !allEdgesNamed(node, "entry") {
			diagnostic := c.diagnostic(node, "dict requires entry edges")
			return nil, &diagnostic
		}
		var insts []bytecode.BCInstruction
		for _, entry := range children {
			if entry.Kind != "dict_entry" || len(entry.DataInputs) != 2 || entry.DataInputs[0].Name != "key" || entry.DataInputs[1].Name != "value" {
				diagnostic := c.diagnostic(entry, "dict entry requires key and value")
				return nil, &diagnostic
			}
			entryChildren, entryDiagnostic := c.children(entry)
			if entryDiagnostic != nil {
				return nil, entryDiagnostic
			}
			for _, child := range entryChildren {
				childInsts, childDiagnostic := c.compile(child)
				if childDiagnostic != nil {
					return nil, childDiagnostic
				}
				insts = append(insts, childInsts...)
			}
		}
		return append(insts, instruction(bytecode.OpMakeDict, "MAKE_DICT", func(inst *bytecode.BCInstruction) { inst.IntOperand = int64(len(children)) })), nil
	case "print":
		if !allEdgesNamed(node, "value") {
			diagnostic := c.diagnostic(node, "print requires value edges")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileAll()
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpPrint, "PRINT", func(inst *bytecode.BCInstruction) { inst.IntOperand = int64(len(children)) })), nil
	case "stderr", "exit", "convert", "list_len", "env":
		if len(children) != 1 || node.DataInputs[0].Name != "value" {
			diagnostic := c.diagnostic(node, node.Kind+" requires one value")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		var op bytecode.Opcode
		var name string
		switch node.Kind {
		case "stderr":
			op, name = bytecode.OpStderr, "STDERR"
		case "exit":
			op, name = bytecode.OpExit, "EXIT"
		case "convert":
			if node.Value == "encode_json" {
				op, name = bytecode.OpEncodeJson, "ENCODE_JSON"
			} else {
				op, name = bytecode.OpConvert, "CONVERT"
			}
		case "list_len":
			op, name = bytecode.OpListLen, "LIST_LEN"
		case "env":
			op, name = bytecode.OpEnv, "ENV"
		}
		return append(insts, instruction(op, name, func(inst *bytecode.BCInstruction) { inst.StringOperand = node.Value })), nil
	case "read_file":
		if len(children) != 1 || node.DataInputs[0].Name != "path" {
			diagnostic := c.diagnostic(node, "read_file requires path")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpReadFile, "READ_FILE", nil)), nil
	case "parse_json":
		if len(children) != 1 || node.DataInputs[0].Name != "content" {
			diagnostic := c.diagnostic(node, "parse_json requires content")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpParseJson, "PARSE_JSON", func(inst *bytecode.BCInstruction) { inst.StringOperand = node.Value })), nil
	case "is_nil":
		if len(children) != 1 || node.DataInputs[0].Name != "value" {
			diagnostic := c.diagnostic(node, "is_nil requires value")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpIsNil, "IS_NIL", nil)), nil
	case "cli_args":
		if len(children) == 1 && node.DataInputs[0].Name == "index" {
			insts, childDiagnostic := compileChild(0)
			if childDiagnostic != nil {
				return nil, childDiagnostic
			}
			return append(insts, instruction(bytecode.OpCliArgsGet, "CLI_ARGS_GET", nil)), nil
		} else if len(children) == 0 {
			return []bytecode.BCInstruction{instruction(bytecode.OpCliArgs, "CLI_ARGS", nil)}, nil
		}
		diagnostic := c.diagnostic(node, "cli_args requires either zero arguments or one index")
		return nil, &diagnostic
	case "str_split", "str_join":
		if len(children) != 2 || node.DataInputs[0].Name != "value" || node.DataInputs[1].Name != "separator" {
			diagnostic := c.diagnostic(node, node.Kind+" requires value and separator")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileAll()
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		op, name := bytecode.OpStrSplit, "STR_SPLIT"
		if node.Kind == "str_join" {
			op, name = bytecode.OpStrJoin, "STR_JOIN"
		}
		return append(insts, instruction(op, name, nil)), nil
	case "map_get", "list_get", "map_delete", "append":
		edgeName := "key"
		if node.Kind == "list_get" {
			edgeName = "index"
		} else if node.Kind == "append" {
			edgeName = "item"
		}
		if node.Value == "" || len(children) != 1 || node.DataInputs[0].Name != edgeName {
			diagnostic := c.diagnostic(node, node.Kind+" requires a target name and "+edgeName)
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		op, name := bytecode.OpMapGet, "MAP_GET"
		switch node.Kind {
		case "list_get":
			op, name = bytecode.OpListGet, "LIST_GET"
		case "map_delete":
			op, name = bytecode.OpMapDelete, "MAP_DELETE"
		case "append":
			op, name = bytecode.OpAppend, "APPEND"
		}
		return append(insts, instruction(op, name, func(inst *bytecode.BCInstruction) { inst.StringOperand = node.Value })), nil
	case "map_set":
		if node.Value == "" || len(children) != 2 || node.DataInputs[0].Name != "key" || node.DataInputs[1].Name != "value" {
			diagnostic := c.diagnostic(node, "map_set requires a target name, key, and value")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileAll()
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpMapSet, "MAP_SET", func(inst *bytecode.BCInstruction) { inst.StringOperand = node.Value })), nil
	case "try":
		if node.Value == "" || len(children) != 3 || node.DataInputs[0].Name != "expression" || node.DataInputs[1].Name != "success_body" || node.DataInputs[2].Name != "catch" {
			diagnostic := c.diagnostic(node, "try requires expression, success_body, and catch edges")
			return nil, &diagnostic
		}
		exprInsts, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		successInsts, childDiagnostic := compileChild(1)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		catchNode := children[2]
		if catchNode.Kind != "catch" || catchNode.Value == "" || len(catchNode.DataInputs) != 1 || catchNode.DataInputs[0].Name != "body" {
			diagnostic := c.diagnostic(catchNode, "catch requires a body edge and error binding")
			return nil, &diagnostic
		}
		catchChildren, catchDiagnostic := c.children(catchNode)
		if catchDiagnostic != nil {
			return nil, catchDiagnostic
		}
		catchBodyInsts, catchChildDiag := c.compile(catchChildren[0])
		if catchChildDiag != nil {
			return nil, catchChildDiag
		}

		insts := []bytecode.BCInstruction{instruction(bytecode.OpTryLet, "TRY_LET", func(inst *bytecode.BCInstruction) {
			inst.StringOperand = node.Value
			inst.StringOperand2 = catchNode.Value
			inst.IntOperand = int64(len(exprInsts))
			inst.IntOperand2 = int64(len(catchBodyInsts))
			inst.IntOperand3 = int64(len(successInsts))
		})}
		insts = append(insts, exprInsts...)
		insts = append(insts, catchBodyInsts...)
		insts = append(insts, successInsts...)
		return insts, nil
	case "for":
		if node.Value == "" || len(children) != 2 || node.DataInputs[0].Name != "iterable" || node.DataInputs[1].Name != "body" {
			diagnostic := c.diagnostic(node, "for requires iterable and body edges")
			return nil, &diagnostic
		}
		listInsts, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		bodyInsts, childDiagnostic := compileChild(1)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}

		var insts []bytecode.BCInstruction
		insts = append(insts, listInsts...)
		insts = append(insts, instruction(bytecode.OpForInit, "FOR_INIT", nil))
		loopStart := len(insts)
		insts = append(insts, instruction(bytecode.OpForNext, "FOR_NEXT", func(inst *bytecode.BCInstruction) {
			inst.StringOperand = node.Value
			inst.IntOperand = int64(len(bodyInsts) + 2)
		}))
		insts = append(insts, bodyInsts...)
		insts = append(insts, instruction(bytecode.OpJump, "JUMP", func(inst *bytecode.BCInstruction) {
			inst.IntOperand = int64(-(len(insts) - loopStart))
		}))
		return insts, nil
	default:
		diagnostic := c.diagnostic(node, fmt.Sprintf("node kind %q is not in the Phase-1 executable subset", node.Kind))
		return nil, &diagnostic
	}
}

func (c *bytecodeLowerer) children(node *Node) ([]*Node, *Diagnostic) {
	children := make([]*Node, 0, len(node.DataInputs))
	for _, edge := range node.DataInputs {
		child := c.graph.NodeByID(edge.SourceNode)
		if child == nil {
			diagnostic := c.diagnostic(node, fmt.Sprintf("data input %q references missing node %q", edge.Name, edge.SourceNode))
			return nil, &diagnostic
		}
		children = append(children, child)
	}
	return children, nil
}

func (c *bytecodeLowerer) diagnostic(node *Node, message string) Diagnostic {
	diagnostic := Diagnostic{
		Code:            BytecodeUnsupportedCode,
		Severity:        SeverityError,
		Message:         message,
		ContractVersion: DiagnosticContractVersion,
		Target:          TargetBytecode,
	}
	if node != nil {
		diagnostic.Location = node.Provenance
		diagnostic.RelatedNode = node.ID
	}
	return diagnostic
}

func allEdgesNamed(node *Node, name string) bool {
	for _, edge := range node.DataInputs {
		if edge.Name != name {
			return false
		}
	}
	return true
}

func instruction(op bytecode.Opcode, name string, apply func(*bytecode.BCInstruction)) bytecode.BCInstruction {
	inst := bytecode.BCInstruction{Op: op, OpString: name}
	if apply != nil {
		apply(&inst)
	}
	return inst
}

func literalValue(node *Node) (any, error) {
	switch node.LiteralKind {
	case "BOOL":
		if node.Value != "true" && node.Value != "false" {
			return nil, fmt.Errorf("invalid boolean literal %q", node.Value)
		}
		return node.Value == "true", nil
	case "INT":
		value, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid integer literal %q", node.Value)
		}
		return float64(value), nil
	case "FLOAT":
		value, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float literal %q", node.Value)
		}
		return value, nil
	case "STRING":
		return node.Value, nil
	default:
		return nil, fmt.Errorf("unsupported literal kind %q", node.LiteralKind)
	}
}
