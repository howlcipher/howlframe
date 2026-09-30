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
	prog, _, _, diags := LowerToBytecodeIncremental(graph, nil)
	return prog, diags
}

// LowerToBytecodeIncremental converts the Phase-1 semantic HFIR subset into a
// BCProgram, reusing cached bytecode fragments for preserved subgraphs.
func LowerToBytecodeIncremental(graph *Graph, preservedCache map[NodeID][]bytecode.BCInstruction) (*bytecode.BCProgram, map[NodeID][]bytecode.BCInstruction, int, []Diagnostic) {
	compiler := &bytecodeLowerer{
		graph:       graph,
		compiling:   make(map[NodeID]bool),
		cachedNodes: preservedCache,
		reusedNodes: make(map[NodeID]bool),
		nodeInsts:   make(map[NodeID][]bytecode.BCInstruction),
		prog: &bytecode.BCProgram{
			Version:   1,
			Functions: make(map[string]*bytecode.BCFunction),
		},
	}
	if graph == nil {
		return nil, nil, 0, []Diagnostic{compiler.diagnostic(nil, "HFIR graph is required")}
	}
	entry := graph.NodeByID(graph.EntryNode)
	if entry == nil {
		return nil, nil, 0, []Diagnostic{compiler.diagnostic(nil, "HFIR entry node is missing")}
	}
	insts, diagnostic := compiler.compile(entry)
	if diagnostic != nil {
		return nil, nil, compiler.loweredCount, []Diagnostic{*diagnostic}
	}
	compiler.prog.Main = insts
	if !compiler.prog.AttachTrustedMainOrigins() || !compiler.prog.BindLocalizationIdentity(GraphHash(graph)) {
		return nil, nil, compiler.loweredCount, []Diagnostic{compiler.diagnostic(entry, "direct HFIR lowering produced incomplete instruction provenance")}
	}
	return compiler.prog, compiler.nodeInsts, compiler.loweredCount, nil
}

type bytecodeLowerer struct {
	graph        *Graph
	prog         *bytecode.BCProgram
	compiling    map[NodeID]bool
	cachedNodes  map[NodeID][]bytecode.BCInstruction
	reusedNodes  map[NodeID]bool
	nodeInsts    map[NodeID][]bytecode.BCInstruction
	loweredCount int
}

func (c *bytecodeLowerer) compile(node *Node) (instructions []bytecode.BCInstruction, diagnostic *Diagnostic) {
	if node == nil {
		value := c.diagnostic(nil, "HFIR node is missing")
		return nil, &value
	}
	// A defun registers a BCFunction. That side effect is not stored in the
	// instruction cache, so a preserved defun is lowered again and the
	// function table is rebuilt. while has no function-table side effect.
	if node.Kind != "defun" {
		if c.cachedNodes != nil && c.cachedNodes[node.ID] != nil {
			if c.reusedNodes != nil {
				c.reusedNodes[node.ID] = true
			}
			return append([]bytecode.BCInstruction(nil), c.cachedNodes[node.ID]...), nil
		}
		if c.nodeInsts != nil && c.nodeInsts[node.ID] != nil {
			return append([]bytecode.BCInstruction(nil), c.nodeInsts[node.ID]...), nil
		}
	}
	c.loweredCount++
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
		if c.nodeInsts != nil {
			c.nodeInsts[node.ID] = append([]bytecode.BCInstruction(nil), instructions...)
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
		successors, shapeDiagnostic := c.ifSuccessors(node)
		if shapeDiagnostic != nil {
			return nil, shapeDiagnostic
		}
		condition, childDiagnostic := c.compile(successors[0])
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		thenInsts, childDiagnostic := c.compile(successors[1])
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		// Same relative jumps as bytecode.CompileToBytecode's if case.
		// JUMP_IF_FALSE skips the then-branch. A present else is skipped
		// by JUMP. No new opcode.
		if len(successors) == 2 {
			condition = append(condition, instruction(bytecode.OpJumpIfFalse, "JUMP_IF_FALSE", func(inst *bytecode.BCInstruction) { inst.IntOperand = int64(len(thenInsts) + 1) }))
			return append(condition, thenInsts...), nil
		}
		elseInsts, childDiagnostic := c.compile(successors[2])
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
	case "stderr", "exit", "convert", "list_len", "env", "map_keys":
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
		case "map_keys":
			op, name = bytecode.OpMapKeys, "MAP_KEYS"
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
	case "write_file":
		if len(children) != 2 || node.DataInputs[0].Name != "path" || node.DataInputs[1].Name != "data" {
			diagnostic := c.diagnostic(node, "write_file requires path and data")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileAll()
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		// Same operand order as bytecode.CompileToBytecode: path, then data.
		// WRITE_FILE pops data, then path. No new opcode.
		return append(insts, instruction(bytecode.OpWriteFile, "WRITE_FILE", nil)), nil
	case "mkdir":
		if len(children) != 1 || node.DataInputs[0].Name != "path" {
			diagnostic := c.diagnostic(node, "mkdir requires path")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileChild(0)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpMkdir, "MKDIR", nil)), nil
	case "exec":
		if len(children) < 1 || node.DataInputs[0].Name != "cmd" {
			diagnostic := c.diagnostic(node, "exec requires a command")
			return nil, &diagnostic
		}
		for _, edge := range node.DataInputs[1:] {
			if edge.Name != "arg" {
				diagnostic := c.diagnostic(node, "exec requires a command and then arguments")
				return nil, &diagnostic
			}
		}
		insts, childDiagnostic := compileAll()
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		// Same operand order as bytecode.CompileToBytecode: command, then
		// each argument. EXEC's IntOperand is the argument count. The VM
		// pops the arguments and then the command. No new opcode.
		return append(insts, instruction(bytecode.OpExec, "EXEC", func(inst *bytecode.BCInstruction) {
			inst.IntOperand = int64(len(children) - 1)
		})), nil
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
	case "req_query", "req_header", "req_path":
		if len(children) != 2 || node.DataInputs[0].Name != "request" || node.DataInputs[1].Name != "name" {
			diagnostic := c.diagnostic(node, node.Kind+" requires request and name")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileAll()
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		op, name := bytecode.OpHttpReqQuery, "HTTP_REQ_QUERY"
		switch node.Kind {
		case "req_header":
			op, name = bytecode.OpHttpReqHeader, "HTTP_REQ_HEADER"
		case "req_path":
			op, name = bytecode.OpHttpReqPath, "HTTP_REQ_PATH"
		}
		return append(insts, instruction(op, name, nil)), nil
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
		iterNode, bodyNode, shapeDiagnostic := c.forSuccessors(node)
		if shapeDiagnostic != nil {
			return nil, shapeDiagnostic
		}
		listInsts, childDiagnostic := c.compile(iterNode)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		bodyInsts, childDiagnostic := c.compile(bodyNode)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		// Same relative jumps as bytecode.CompileToBytecode's for case.
		// FOR_NEXT skips the body and the back edge when the iterator is
		// done. JUMP returns to FOR_NEXT. No new opcode.
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
	case "while":
		condNode, bodyNode, shapeDiagnostic := c.whileSuccessors(node)
		if shapeDiagnostic != nil {
			return nil, shapeDiagnostic
		}
		condInsts, childDiagnostic := c.compile(condNode)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		bodyInsts, childDiagnostic := c.compile(bodyNode)
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		// Same relative jumps as bytecode.CompileToBytecode's while case.
		// JUMP_IF_FALSE skips the body and the back edge. JUMP returns to
		// the condition. No new opcode.
		insts := append([]bytecode.BCInstruction{}, condInsts...)
		insts = append(insts, instruction(bytecode.OpJumpIfFalse, "JUMP_IF_FALSE", func(inst *bytecode.BCInstruction) {
			inst.IntOperand = int64(len(bodyInsts) + 2)
		}))
		insts = append(insts, bodyInsts...)
		insts = append(insts, instruction(bytecode.OpJump, "JUMP", func(inst *bytecode.BCInstruction) {
			inst.IntOperand = int64(-(len(condInsts) + 1 + len(bodyInsts)))
		}))
		return insts, nil
	case "defun":
		if node.Value == "" {
			diagnostic := c.diagnostic(node, "defun requires a name")
			return nil, &diagnostic
		}
		params := make([]string, 0)
		var bodies []*Node
		for index, edge := range node.DataInputs {
			child := children[index]
			switch edge.Name {
			case "param":
				if child.Kind != "param" || child.Value == "" {
					diagnostic := c.diagnostic(child, "defun param requires a name")
					return nil, &diagnostic
				}
				params = append(params, child.Value)
			case "body":
				bodies = append(bodies, child)
			default:
				diagnostic := c.diagnostic(node, "defun requires param and body edges")
				return nil, &diagnostic
			}
		}
		var bodyInsts []bytecode.BCInstruction
		for _, body := range bodies {
			insts, childDiagnostic := c.compile(body)
			if childDiagnostic != nil {
				return nil, childDiagnostic
			}
			bodyInsts = append(bodyInsts, insts...)
		}
		if c.prog.Functions == nil {
			c.prog.Functions = make(map[string]*bytecode.BCFunction)
		}
		c.prog.Functions[node.Value] = &bytecode.BCFunction{
			Name:         node.Value,
			Params:       params,
			Instructions: append([]bytecode.BCInstruction(nil), bodyInsts...),
		}
		// A definition emits no value into the caller. The call opcode does.
		return nil, nil
	case "param":
		diagnostic := c.diagnostic(node, "param is a defun binding and has no instructions")
		return nil, &diagnostic
	case "call":
		if node.Value == "" || !allEdgesNamed(node, "arg") {
			diagnostic := c.diagnostic(node, "call requires a function name and arg edges")
			return nil, &diagnostic
		}
		insts, childDiagnostic := compileAll()
		if childDiagnostic != nil {
			return nil, childDiagnostic
		}
		return append(insts, instruction(bytecode.OpCall, "CALL", func(inst *bytecode.BCInstruction) {
			inst.StringOperand = node.Value
			inst.IntOperand = int64(len(children))
		})), nil
	case "return":
		if len(children) > 1 || (len(children) == 1 && node.DataInputs[0].Name != "value") {
			diagnostic := c.diagnostic(node, "return requires an optional value")
			return nil, &diagnostic
		}
		var insts []bytecode.BCInstruction
		if len(children) == 1 {
			var childDiagnostic *Diagnostic
			insts, childDiagnostic = compileChild(0)
			if childDiagnostic != nil {
				return nil, childDiagnostic
			}
		} else {
			insts = []bytecode.BCInstruction{instruction(bytecode.OpLoadConst, "LOAD_CONST", func(inst *bytecode.BCInstruction) {
				inst.ValueOperand = nil
			})}
		}
		return append(insts, instruction(bytecode.OpReturn, "RETURN", nil)), nil
	case "type_hint", "type_hints", "type_param":
		// Annotations have no runtime meaning. The AST bytecode compiler
		// emits nothing for them, including inside a defun body.
		return []bytecode.BCInstruction{}, nil
	default:
		diagnostic := c.diagnostic(node, fmt.Sprintf("node kind %q is not in the Phase-1 executable subset", node.Kind))
		return nil, &diagnostic
	}
}

// forSuccessors is the executable control-edge contract for an iterator
// header. ControlEdges[0] is the iterable and ControlEdges[1] is the body.
// They must be the same nodes as the named data edges, and Value is the
// iterator binding. A for without those edges is not executable. The back
// edge is the JUMP the caller emits.
func (c *bytecodeLowerer) forSuccessors(node *Node) (*Node, *Node, *Diagnostic) {
	if node.Value == "" || len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "iterable" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
		diagnostic := c.diagnostic(node, "for requires an iterable control edge and a body control edge")
		return nil, nil, &diagnostic
	}
	iter := c.graph.NodeByID(node.ControlEdges[0])
	body := c.graph.NodeByID(node.ControlEdges[1])
	if iter == nil || body == nil {
		diagnostic := c.diagnostic(node, "for control edge references a missing node")
		return nil, nil, &diagnostic
	}
	return iter, body, nil
}

// whileSuccessors is the executable control-edge contract for a loop header.
// ControlEdges[0] is the condition and ControlEdges[1] is the body. They must
// be the same nodes as the named data edges. A while without those edges is
// not executable.
func (c *bytecodeLowerer) whileSuccessors(node *Node) (*Node, *Node, *Diagnostic) {
	if len(node.ControlEdges) != 2 || len(node.DataInputs) != 2 || node.DataInputs[0].Name != "condition" || node.DataInputs[1].Name != "body" || node.ControlEdges[0] != node.DataInputs[0].SourceNode || node.ControlEdges[1] != node.DataInputs[1].SourceNode {
		diagnostic := c.diagnostic(node, "while requires a condition control edge and a body control edge")
		return nil, nil, &diagnostic
	}
	cond := c.graph.NodeByID(node.ControlEdges[0])
	body := c.graph.NodeByID(node.ControlEdges[1])
	if cond == nil || body == nil {
		diagnostic := c.diagnostic(node, "while control edge references a missing node")
		return nil, nil, &diagnostic
	}
	return cond, body, nil
}

// ifSuccessors is the executable control-edge contract for a branch.
// ControlEdges are the condition, then the then-branch, and an optional
// else. They must be the same nodes as the named data edges, in that order.
// An if whose control edges are missing or swapped is not executable.
func (c *bytecodeLowerer) ifSuccessors(node *Node) ([]*Node, *Diagnostic) {
	n := len(node.DataInputs)
	shapeOK := (n == 2 || n == 3) && len(node.ControlEdges) == n && node.DataInputs[0].Name == "condition" && node.DataInputs[1].Name == "then" && (n != 3 || node.DataInputs[2].Name == "else")
	if shapeOK {
		for index := range node.ControlEdges {
			if node.ControlEdges[index] != node.DataInputs[index].SourceNode {
				shapeOK = false
				break
			}
		}
	}
	if !shapeOK {
		diagnostic := c.diagnostic(node, "if requires a condition control edge, a then control edge, and an optional else control edge")
		return nil, &diagnostic
	}
	successors := make([]*Node, n)
	for index, id := range node.ControlEdges {
		successors[index] = c.graph.NodeByID(id)
		if successors[index] == nil {
			diagnostic := c.diagnostic(node, "if control edge references a missing node")
			return nil, &diagnostic
		}
	}
	return successors, nil
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
