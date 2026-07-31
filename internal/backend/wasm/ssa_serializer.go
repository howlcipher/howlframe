package wasm

import (
	"fmt"
	"strconv"
	"strings"
	"zero/internal/ast"
	"zero/internal/ir"
)

// SerializeSSA emits a standalone WAT module from a typed SSA graph.
// Supports primitive numeric/boolean values, canonical if/phi control flow,
// and structured while loops (back edges produced by lowerWhile in ssa.go).
// Unsupported graph shapes return an error instead of falling back silently.
func SerializeSSA(graph *ir.Graph) (string, error) {
	if err := graph.Validate(); err != nil {
		return "", fmt.Errorf("invalid SSA graph: %w", err)
	}

	serializer := newSSASerializer(graph, nil)
	body, resultType, err := serializer.buildBody()
	if err != nil {
		return "", err
	}

	wat := fmt.Sprintf(
		"(module\n  (func (export \"main\") (result %s)\n    %s\n  )\n)\n",
		resultType,
		body,
	)
	if err := ValidateWAT(wat); err != nil {
		return "", fmt.Errorf("invalid SSA WAT: %w", err)
	}
	return wat, nil
}

// Function is one named, typed function ready for multi-function Wasm
// program serialization via SerializeSSAProgram. Graph is produced by
// ir.LowerSSAFunction from the same Params.
type Function struct {
	Name   string
	Params []ir.Param
	Return ast.TypeInfo
	Graph  *ir.Graph
}

// signatureInfo is the arity/type signature of one function in a program,
// used to validate and generate code for ir.OpCall instructions.
type signatureInfo struct {
	params []ast.TypeInfo
	result ast.TypeInfo
}

// SerializeSSAProgram emits a standalone WAT module containing one function
// per entry in functions plus an exported "main" entrypoint for entry.
// Calls between functions (including recursion) are resolved by name via
// ir.OpCall. With no functions, this produces byte-identical output to
// SerializeSSA(entry).
func SerializeSSAProgram(functions []Function, entry *ir.Graph) (string, error) {
	if len(functions) == 0 {
		return SerializeSSA(entry)
	}

	signatures := make(map[string]signatureInfo, len(functions))
	for _, function := range functions {
		if function.Graph == nil {
			return "", fmt.Errorf("function %q has no lowered graph", function.Name)
		}
		if _, exists := signatures[function.Name]; exists {
			return "", fmt.Errorf("duplicate function %q", function.Name)
		}
		paramTypes := make([]ast.TypeInfo, len(function.Params))
		for index, param := range function.Params {
			paramTypes[index] = param.Type
		}
		signatures[function.Name] = signatureInfo{params: paramTypes, result: function.Return}
	}

	var declarations []string
	for _, function := range functions {
		declaration, err := serializeFunctionDeclaration(function.Name, "", function.Graph, function.Params, function.Return, signatures)
		if err != nil {
			return "", fmt.Errorf("function %q: %w", function.Name, err)
		}
		declarations = append(declarations, declaration)
	}
	mainDeclaration, err := serializeFunctionDeclaration("main", "main", entry, nil, ast.TypeInfo{}, signatures)
	if err != nil {
		return "", fmt.Errorf("entry expression: %w", err)
	}
	declarations = append(declarations, mainDeclaration)

	wat := fmt.Sprintf("(module\n%s\n)\n", strings.Join(declarations, "\n"))
	if err := ValidateWAT(wat); err != nil {
		return "", fmt.Errorf("invalid SSA WAT: %w", err)
	}
	return wat, nil
}

// serializeFunctionDeclaration validates graph and emits its WAT (func ...)
// declaration, with one (param $name type) clause per params entry and an
// (export exportName) clause when exportName is non-empty. returnType is
// checked against the graph's inferred result type when its Kind is set
// (i.e. for a real defun signature); the zero value skips that check and
// trusts the graph's own inferred result type (used for the entry function,
// which has no separately-declared signature).
func serializeFunctionDeclaration(name, exportName string, graph *ir.Graph, params []ir.Param, returnType ast.TypeInfo, signatures map[string]signatureInfo) (string, error) {
	if err := graph.Validate(); err != nil {
		return "", fmt.Errorf("invalid SSA graph: %w", err)
	}
	serializer := newSSASerializer(graph, signatures)
	body, resultType, err := serializer.buildBody()
	if err != nil {
		return "", err
	}
	if returnType.Kind != "" {
		declaredType, err := wasmPrimitiveType(returnType)
		if err != nil {
			return "", fmt.Errorf("declared return type: %w", err)
		}
		if declaredType != resultType {
			return "", fmt.Errorf("declares result %s, body has type %s", declaredType, resultType)
		}
	}
	var paramClauses strings.Builder
	for _, param := range params {
		paramType, err := wasmPrimitiveType(param.Type)
		if err != nil {
			return "", fmt.Errorf("parameter %q: %w", param.Name, err)
		}
		fmt.Fprintf(&paramClauses, " (param $%s %s)", param.Name, paramType)
	}
	exportClause := ""
	if exportName != "" {
		exportClause = fmt.Sprintf(" (export %q)", exportName)
	}
	return fmt.Sprintf(
		"  (func $%s%s%s (result %s)\n    %s\n  )",
		name, exportClause, paramClauses.String(), resultType, body,
	), nil
}

// newSSASerializer builds an ssaSerializer with its block/instruction
// indexes populated. signatures is nil for a single-function graph (any
// ir.OpCall in that graph then fails as calling an undefined function).
func newSSASerializer(graph *ir.Graph, signatures map[string]signatureInfo) *ssaSerializer {
	serializer := &ssaSerializer{
		graph:        graph,
		blocks:       make(map[ir.BlockLabel]*ir.BasicBlock, len(graph.Blocks)),
		instructions: make(map[ir.ValueID]*ir.Instruction),
		signatures:   signatures,
	}
	for _, block := range graph.Blocks {
		serializer.blocks[block.Label] = block
		for index := range block.Instructions {
			instruction := &block.Instructions[index]
			serializer.instructions[instruction.Result.ID] = instruction
		}
	}
	return serializer
}

// buildBody validates the graph's loop shape (if any) and instruction
// support, then serializes its body content — no (module ...)/(func ...)
// wrapper — returning the body text and its result type. Shared by
// SerializeSSA and SerializeSSAProgram so both stay in lockstep.
func (serializer *ssaSerializer) buildBody() (string, string, error) {
	// Detect loop structure (or reject unsupported cycles).
	loop, err := serializer.detectAndValidateLoop()
	if err != nil {
		return "", "", err
	}

	// Now that the loop shape is validated, verify all instructions are supported.
	// OpUnit is a void marker emitted by lowerWhile in exit/body blocks — always safe to allow.
	// OpSet appears in loop body blocks and is handled specially during loop codegen.
	loopBodyLabels := map[ir.BlockLabel]bool{}
	if loop != nil {
		loopBodyLabels[loop.bodyBlock.Label] = true
		loopBodyLabels[loop.exitBlock.Label] = true
	}
	for _, block := range serializer.graph.Blocks {
		for index := range block.Instructions {
			instruction := &block.Instructions[index]
			if loopBodyLabels[block.Label] {
				// Loop body/exit blocks allow OpSet and OpUnit in addition to normal ops.
				if !supportedSSAOperation(instruction.Op) &&
					instruction.Op != ir.OpSet && instruction.Op != ir.OpUnit {
					return "", "", serializer.unsupported(instruction)
				}
			} else {
				if !supportedSSAOperation(instruction.Op) {
					return "", "", serializer.unsupported(instruction)
				}
			}
		}
	}

	if loop != nil {
		return serializer.loopFunctionBody(loop)
	}
	return serializer.functionBody()
}

// loopInfo describes the structured while loop shape produced by lowerWhile.
type loopInfo struct {
	preheaderBlock *ir.BasicBlock
	headerBlock    *ir.BasicBlock
	bodyBlock      *ir.BasicBlock
	exitBlock      *ir.BasicBlock
	// phis is the list of phi instructions in the header block (the loop-carried variables).
	phis []ir.Instruction
	// localNames maps phi ValueID → WAT local name (e.g. "$total_3").
	localNames map[ir.ValueID]string
}

type ssaSerializer struct {
	graph        *ir.Graph
	blocks       map[ir.BlockLabel]*ir.BasicBlock
	instructions map[ir.ValueID]*ir.Instruction
	resolving    map[ir.ValueID]bool
	// loopLocals maps loop-carried phi ValueIDs to their WAT local names.
	// Non-nil only while generating a loop.
	loopLocals map[ir.ValueID]string
	// signatures holds every function's arity/type signature in the current
	// program, used to validate and generate code for ir.OpCall. Nil when
	// serializing a standalone single-function graph (SerializeSSA).
	signatures map[string]signatureInfo
}

type ssaReturn struct {
	block *ir.BasicBlock
	value ir.ValueID
}

// detectAndValidateLoop performs a DFS over the graph to find cycles.
// If it finds exactly the structured while-loop pattern produced by lowerWhile,
// it returns a *loopInfo describing it.
// Any other cycle (or a structurally incorrect loop) returns an error.
func (serializer *ssaSerializer) detectAndValidateLoop() (*loopInfo, error) {
	state := make(map[ir.BlockLabel]uint8) // 0=unvisited, 1=on-stack, 2=done
	var backedgeFrom, backedgeTo ir.BlockLabel

	var visit func(ir.BlockLabel) error
	visit = func(label ir.BlockLabel) error {
		switch state[label] {
		case 1:
			// Back edge found: label is on the DFS stack.
			if backedgeTo != "" {
				// A second back edge — nested loops are not supported.
				return fmt.Errorf("SSA Wasm backend does not support loops involving block %q (nested loops not supported)", label)
			}
			backedgeTo = label
			return nil
		case 2:
			return nil
		}
		state[label] = 1
		block := serializer.blocks[label]
		var targets []ir.BlockLabel
		switch block.Terminator.Kind {
		case ir.TermJump:
			targets = append(targets, block.Terminator.Target)
		case ir.TermBranch:
			targets = append(targets, block.Terminator.TrueTarget, block.Terminator.FalseTarget)
		}
		for _, target := range targets {
			if state[target] == 1 {
				// Record the block causing the backedge.
				if backedgeTo != "" && backedgeTo != target {
					return fmt.Errorf("SSA Wasm backend does not support loops involving block %q (nested loops not supported)", target)
				}
				backedgeTo = target
				backedgeFrom = label
				continue
			}
			if err := visit(target); err != nil {
				return err
			}
			// After visiting, see if the backedgeFrom was set inside a nested call.
			if backedgeFrom == "" && backedgeTo == target {
				// This shouldn't happen but guard anyway.
			}
		}
		state[label] = 2
		return nil
	}

	if err := visit(serializer.graph.Entry); err != nil {
		return nil, err
	}

	if backedgeTo == "" {
		// No cycle — acyclic graph, existing path handles it.
		return nil, nil
	}

	// A cycle was found. Validate that it matches the lowerWhile pattern.
	return serializer.validateLoopShape(backedgeTo, backedgeFrom)
}

// validateLoopShape checks that the cycle from backedgeFrom → backedgeTo
// matches the exact shape produced by lowerWhile, and returns loopInfo if so.
func (serializer *ssaSerializer) validateLoopShape(headerLabel, bodyLabel ir.BlockLabel) (*loopInfo, error) {
	headerBlock := serializer.blocks[headerLabel]
	if headerBlock == nil {
		return nil, fmt.Errorf("SSA Wasm backend does not support loops involving block %q", headerLabel)
	}
	bodyBlock := serializer.blocks[bodyLabel]
	if bodyBlock == nil {
		return nil, fmt.Errorf("SSA Wasm backend does not support loops involving block %q", bodyLabel)
	}

	// The header must have a TermBranch (the loop condition).
	if headerBlock.Terminator.Kind != ir.TermBranch {
		return nil, fmt.Errorf("SSA Wasm backend does not support loops involving block %q (header must end with a branch)", headerLabel)
	}

	// The header's instructions must start with OpPhi instructions for loop-carried variables.
	// There must be at least one phi (otherwise this is not a structured loop we can lower).
	var phis []ir.Instruction
	for _, instr := range headerBlock.Instructions {
		if instr.Op == ir.OpPhi {
			if len(instr.Operands) != 2 || len(instr.Blocks) != 2 {
				return nil, fmt.Errorf("SSA Wasm backend does not support loops involving block %q (phi %%%d must have exactly 2 operands)", headerLabel, instr.Result.ID)
			}
			phis = append(phis, instr)
		}
		// Non-phi instructions in the header are the condition computation — fine.
	}
	if len(phis) == 0 {
		return nil, fmt.Errorf("SSA Wasm backend does not support loops involving block %q", headerLabel)
	}

	// The body block must end with a TermJump back to the header.
	if bodyBlock.Terminator.Kind != ir.TermJump || bodyBlock.Terminator.Target != headerLabel {
		return nil, fmt.Errorf("SSA Wasm backend does not support loops involving block %q (body must jump back to header)", bodyLabel)
	}

	// Find the preheader: the unique predecessor of the header that is NOT the body.
	// In lowerWhile, the preheader is the block that jumps to the header first.
	var preheaderBlock *ir.BasicBlock
	for _, block := range serializer.graph.Blocks {
		if block.Label == headerLabel || block.Label == bodyLabel {
			continue
		}
		term := block.Terminator
		if term.Kind == ir.TermJump && term.Target == headerLabel {
			if preheaderBlock != nil {
				return nil, fmt.Errorf("SSA Wasm backend does not support loops involving block %q (multiple preheaders)", headerLabel)
			}
			preheaderBlock = block
		}
	}
	if preheaderBlock == nil {
		return nil, fmt.Errorf("SSA Wasm backend does not support loops involving block %q (no preheader found)", headerLabel)
	}

	// Verify each phi's incoming blocks: one from the preheader, one from the body.
	for _, phi := range phis {
		hasPreheader := false
		hasBody := false
		for _, block := range phi.Blocks {
			if block == preheaderBlock.Label {
				hasPreheader = true
			}
			if block == bodyLabel {
				hasBody = true
			}
		}
		if !hasPreheader || !hasBody {
			return nil, fmt.Errorf("SSA Wasm backend does not support loops involving block %q (phi %%%d has unexpected predecessors)", headerLabel, phi.Result.ID)
		}
	}

	// Find the exit block: the false target of the header's branch.
	exitLabel := headerBlock.Terminator.FalseTarget
	exitBlock := serializer.blocks[exitLabel]
	if exitBlock == nil {
		return nil, fmt.Errorf("loop exit block %q not found", exitLabel)
	}

	// Build local names for each phi.
	localNames := make(map[ir.ValueID]string, len(phis))
	for _, phi := range phis {
		// Use symbol (source variable name) + value ID for uniqueness.
		name := fmt.Sprintf("$%s_%d", phi.Symbol, phi.Result.ID)
		if phi.Symbol == "" {
			name = fmt.Sprintf("$v%d", phi.Result.ID)
		}
		localNames[phi.Result.ID] = name
	}

	return &loopInfo{
		preheaderBlock: preheaderBlock,
		headerBlock:    headerBlock,
		bodyBlock:      bodyBlock,
		exitBlock:      exitBlock,
		phis:           phis,
		localNames:     localNames,
	}, nil
}

// loopFunctionBody generates the WAT body for a graph containing a while
// loop. The structure uses a nested block pattern:
//
//	(block (result T)            ;; outer: provides the return value
//	  (local.set ...)            ;; init each phi from preheader value
//	  (block                     ;; inner void block: exit target for br_if
//	    (loop                    ;; loop header
//	      (br_if 1 (i32.eqz condition))  ;; exit inner block if cond false
//	      <body stmts>
//	      (br 0)                 ;; back to loop top
//	    )
//	  )
//	  (local.get $returnVar)     ;; produce function result from local
//	)
//
// It builds the local declarations, init statements, and
// nested block/loop body for a structured while loop, returning the joined
// body content (no module/func wrapper) and its result type.
func (serializer *ssaSerializer) loopFunctionBody(loop *loopInfo) (string, string, error) {
	// Set up loop-local resolution so that expression() on a loop phi → local.get.
	serializer.loopLocals = loop.localNames

	// Determine the result type from the return value.
	var returnValue ir.ValueID
	for _, block := range serializer.graph.Blocks {
		if block.Terminator.Kind == ir.TermReturn && block.Terminator.Value != 0 {
			returnValue = block.Terminator.Value
			break
		}
	}
	if returnValue == 0 {
		return "", "", fmt.Errorf("SSA loop graph has no return value")
	}

	returnExpr, resultType, err := serializer.expression(returnValue)
	if err != nil {
		return "", "", fmt.Errorf("loop return expression: %w", err)
	}

	// Build local declarations.
	var localDecls []string
	for _, phi := range loop.phis {
		localType, err := wasmPrimitiveType(phi.Result.Type)
		if err != nil {
			return "", "", fmt.Errorf("loop phi %%%d: %w", phi.Result.ID, err)
		}
		localDecls = append(localDecls, fmt.Sprintf("(local %s %s)", loop.localNames[phi.Result.ID], localType))
	}

	// Build local.set initializations from phi preheader operands.
	var initStmts []string
	for _, phi := range loop.phis {
		localName := loop.localNames[phi.Result.ID]
		preheaderOperand := ir.ValueID(0)
		for idx, block := range phi.Blocks {
			if block == loop.preheaderBlock.Label {
				preheaderOperand = phi.Operands[idx]
				break
			}
		}
		if preheaderOperand == 0 {
			return "", "", fmt.Errorf("loop phi %%%d has no preheader operand", phi.Result.ID)
		}
		initExpr, _, err := serializer.expression(preheaderOperand)
		if err != nil {
			return "", "", fmt.Errorf("loop phi %%%d init: %w", phi.Result.ID, err)
		}
		initStmts = append(initStmts, fmt.Sprintf("(local.set %s %s)", localName, initExpr))
	}

	// Build the loop condition from the header's branch condition.
	condExpr, condType, err := serializer.expression(loop.headerBlock.Terminator.Cond)
	if err != nil {
		return "", "", fmt.Errorf("loop condition: %w", err)
	}
	if condType != "i32" {
		return "", "", fmt.Errorf("loop condition has type %s, want i32", condType)
	}

	// Build body statements from the body block.
	// OpSet → emit (local.set $name <expr>)
	// OpUnit → skip (void marker)
	// Other ops → resolved lazily via expression() when referenced by OpSet
	var bodyStmts []string
	for _, instr := range loop.bodyBlock.Instructions {
		switch instr.Op {
		case ir.OpSet:
			localName := serializer.findLocalNameBySymbol(instr.Symbol, loop)
			if localName == "" {
				return "", "", fmt.Errorf("loop body OpSet for symbol %q has no matching loop phi", instr.Symbol)
			}
			valExpr, _, err := serializer.expression(instr.Operands[0])
			if err != nil {
				return "", "", fmt.Errorf("loop body set %q: %w", instr.Symbol, err)
			}
			bodyStmts = append(bodyStmts, fmt.Sprintf("(local.set %s %s)", localName, valExpr))
		case ir.OpUnit:
			// void marker — skip
		default:
			// Binary ops and constants are computed lazily by expression()
			// and only appear in output when referenced. No need to emit them standalone.
		}
	}

	// Assemble the WAT using a two-block nesting pattern:
	// outer (block (result T)) → inner (block) void exit target → (loop) body
	//
	// Inside the loop:
	//   (br_if 1 (i32.eqz cond))  ;; exit inner void block when cond is false
	//   body stmts
	//   (br 0)                     ;; continue loop
	//
	// After inner block exits (cond was false): local.get provides the result.
	indent := "        "
	loopBody := []string{
		fmt.Sprintf("(br_if 1 (i32.eqz %s))", condExpr),
	}
	loopBody = append(loopBody, bodyStmts...)
	loopBody = append(loopBody, "(br 0)")

	loopStmt := fmt.Sprintf("(loop\n%s%s\n      )",
		indent,
		strings.Join(loopBody, "\n"+indent))

	innerBlock := fmt.Sprintf("(block\n      %s\n    )", loopStmt)

	// outer block: inits + inner block + result expr
	var outerContents []string
	outerContents = append(outerContents, initStmts...)
	outerContents = append(outerContents, innerBlock)
	outerContents = append(outerContents, returnExpr)

	outerBlock := fmt.Sprintf("(block (result %s)\n    %s\n  )",
		resultType,
		strings.Join(outerContents, "\n    "))

	// Assemble function with local declarations then the outer block.
	var funcParts []string
	for _, decl := range localDecls {
		funcParts = append(funcParts, decl)
	}
	funcParts = append(funcParts, outerBlock)

	return strings.Join(funcParts, "\n    "), resultType, nil
}

// findLocalNameBySymbol finds the WAT local name for a phi with the given symbol name.
func (serializer *ssaSerializer) findLocalNameBySymbol(symbol string, loop *loopInfo) string {
	for _, phi := range loop.phis {
		if phi.Symbol == symbol {
			return loop.localNames[phi.Result.ID]
		}
	}
	return ""
}

func (serializer *ssaSerializer) functionBody() (string, string, error) {
	var returns []ssaReturn
	for _, block := range serializer.graph.Blocks {
		if block.Terminator.Kind == ir.TermReturn {
			if block.Terminator.Value == 0 {
				return "", "", fmt.Errorf("SSA Wasm backend does not support void returns in block %q", block.Label)
			}
			returns = append(returns, ssaReturn{block: block, value: block.Terminator.Value})
		}
	}
	switch len(returns) {
	case 0:
		return "", "", fmt.Errorf("SSA graph has no return terminator")
	case 1:
		body, valueType, err := serializer.expression(returns[0].value)
		return body, valueType, err
	case 2:
		return serializer.returningBranch(returns)
	default:
		return "", "", fmt.Errorf("SSA Wasm backend does not support %d independent return paths", len(returns))
	}
}

func (serializer *ssaSerializer) returningBranch(returns []ssaReturn) (string, string, error) {
	for _, block := range serializer.graph.Blocks {
		terminator := block.Terminator
		if terminator.Kind != ir.TermBranch {
			continue
		}
		trueReturn, trueOK := returnForBlock(returns, terminator.TrueTarget)
		falseReturn, falseOK := returnForBlock(returns, terminator.FalseTarget)
		if !trueOK || !falseOK {
			continue
		}
		condition, conditionType, err := serializer.expression(terminator.Cond)
		if err != nil {
			return "", "", err
		}
		if conditionType != "i32" {
			return "", "", fmt.Errorf("branch condition %%%d has type %s, want bool", terminator.Cond, conditionType)
		}
		trueExpression, trueType, err := serializer.expression(trueReturn.value)
		if err != nil {
			return "", "", err
		}
		falseExpression, falseType, err := serializer.expression(falseReturn.value)
		if err != nil {
			return "", "", err
		}
		if trueType != falseType {
			return "", "", fmt.Errorf("return paths have incompatible primitive types %s and %s", trueType, falseType)
		}
		body := fmt.Sprintf(
			"(if (result %s) %s (then (return %s)) (else (return %s)))",
			trueType,
			condition,
			trueExpression,
			falseExpression,
		)
		return body, trueType, nil
	}
	return "", "", fmt.Errorf("SSA Wasm backend supports multiple returns only as the two arms of one branch")
}

func returnForBlock(returns []ssaReturn, label ir.BlockLabel) (ssaReturn, bool) {
	for _, result := range returns {
		if result.block.Label == label {
			return result, true
		}
	}
	return ssaReturn{}, false
}

func (serializer *ssaSerializer) expression(value ir.ValueID) (string, string, error) {
	instruction, exists := serializer.instructions[value]
	if !exists {
		return "", "", fmt.Errorf("SSA value %%%d has no defining instruction", value)
	}
	if serializer.resolving == nil {
		serializer.resolving = make(map[ir.ValueID]bool)
	}
	if serializer.resolving[value] {
		return "", "", fmt.Errorf("SSA value dependency cycle at %%%d", value)
	}
	serializer.resolving[value] = true
	defer delete(serializer.resolving, value)

	switch instruction.Op {
	case ir.OpConst:
		return serializer.constant(instruction)
	case ir.OpPhi:
		// Loop-carried phi: resolve via local.get instead of recursive phi expansion.
		if serializer.loopLocals != nil {
			if localName, ok := serializer.loopLocals[value]; ok {
				localType, err := wasmPrimitiveType(instruction.Result.Type)
				if err != nil {
					return "", "", fmt.Errorf("loop phi %%%d: %w", value, err)
				}
				return fmt.Sprintf("(local.get %s)", localName), localType, nil
			}
		}
		return serializer.phi(instruction)
	case ir.OpParam:
		return serializer.param(instruction)
	case ir.OpCall:
		return serializer.call(instruction)
	default:
		return serializer.binary(instruction)
	}
}

// param resolves an OpParam reference to the WAT param local it was bound to
// by serializeFunctionDeclaration (one (param $name type) per ir.Param, in
// the same order LowerSSAFunction bound them into the function's env).
func (serializer *ssaSerializer) param(instruction *ir.Instruction) (string, string, error) {
	resultType, err := wasmPrimitiveType(instruction.Result.Type)
	if err != nil {
		return "", "", fmt.Errorf("SSA param %%%d: %w", instruction.Result.ID, err)
	}
	return fmt.Sprintf("(local.get $%s)", instruction.Symbol), resultType, nil
}

// call resolves an OpCall to another function in the same program, declared
// via serializeFunctionDeclaration/SerializeSSAProgram. Arity and every
// argument/result type are checked against the callee's signature; strings,
// lists, dicts, and any other non-scalar type are rejected here (through
// wasmPrimitiveType) rather than silently mis-serialized.
func (serializer *ssaSerializer) call(instruction *ir.Instruction) (string, string, error) {
	signature, ok := serializer.signatures[instruction.Symbol]
	if !ok {
		return "", "", fmt.Errorf(
			"SSA Wasm backend does not support calling undefined function %q at %%%d",
			instruction.Symbol, instruction.Result.ID,
		)
	}
	if len(instruction.Operands) != len(signature.params) {
		return "", "", fmt.Errorf(
			"call to %q at %%%d expects %d arguments, got %d",
			instruction.Symbol, instruction.Result.ID, len(signature.params), len(instruction.Operands),
		)
	}
	var argExprs []string
	for index, operand := range instruction.Operands {
		expr, actualType, err := serializer.expression(operand)
		if err != nil {
			return "", "", err
		}
		expectedType, err := wasmPrimitiveType(signature.params[index])
		if err != nil {
			return "", "", fmt.Errorf("call to %q argument %d: %w", instruction.Symbol, index+1, err)
		}
		if actualType != expectedType {
			return "", "", fmt.Errorf(
				"call to %q argument %d has type %s, want %s",
				instruction.Symbol, index+1, actualType, expectedType,
			)
		}
		argExprs = append(argExprs, expr)
	}
	resultType, err := wasmPrimitiveType(signature.result)
	if err != nil {
		return "", "", fmt.Errorf("call to %q result: %w", instruction.Symbol, err)
	}
	declaredType, err := wasmPrimitiveType(instruction.Result.Type)
	if err != nil {
		return "", "", fmt.Errorf("SSA result %%%d: %w", instruction.Result.ID, err)
	}
	if declaredType != resultType {
		return "", "", fmt.Errorf(
			"call to %q at %%%d declares result %s, want %s",
			instruction.Symbol, instruction.Result.ID, declaredType, resultType,
		)
	}
	argsClause := ""
	if len(argExprs) > 0 {
		argsClause = " " + strings.Join(argExprs, " ")
	}
	return fmt.Sprintf("(call $%s%s)", instruction.Symbol, argsClause), resultType, nil
}

func (serializer *ssaSerializer) constant(instruction *ir.Instruction) (string, string, error) {
	switch instruction.Result.Type.Kind {
	case ast.Int:
		if _, err := strconv.ParseInt(instruction.Literal, 10, 64); err != nil {
			return "", "", fmt.Errorf("SSA int constant %%%d has invalid literal %q", instruction.Result.ID, instruction.Literal)
		}
		return fmt.Sprintf("(i64.const %s)", instruction.Literal), "i64", nil
	case ast.Float:
		if _, err := strconv.ParseFloat(instruction.Literal, 64); err != nil {
			return "", "", fmt.Errorf("SSA float constant %%%d has invalid literal %q", instruction.Result.ID, instruction.Literal)
		}
		return fmt.Sprintf("(f64.const %s)", instruction.Literal), "f64", nil
	case ast.Bool:
		switch instruction.Literal {
		case "true":
			return "(i32.const 1)", "i32", nil
		case "false":
			return "(i32.const 0)", "i32", nil
		default:
			return "", "", fmt.Errorf("SSA bool constant %%%d has invalid literal %q", instruction.Result.ID, instruction.Literal)
		}
	default:
		return "", "", fmt.Errorf(
			"SSA Wasm backend does not support %s constant %%%d",
			primitiveName(instruction.Result.Type),
			instruction.Result.ID,
		)
	}
}

func (serializer *ssaSerializer) binary(instruction *ir.Instruction) (string, string, error) {
	if len(instruction.Operands) != 2 {
		return "", "", fmt.Errorf("SSA operation %q at %%%d expects two operands", instruction.Op, instruction.Result.ID)
	}
	left, leftType, err := serializer.expression(instruction.Operands[0])
	if err != nil {
		return "", "", err
	}
	right, rightType, err := serializer.expression(instruction.Operands[1])
	if err != nil {
		return "", "", err
	}

	operator, operandType, resultType := ssaWasmOperator(instruction.Op, leftType)
	if operator == "" {
		return "", "", serializer.unsupported(instruction)
	}
	if leftType != operandType || rightType != operandType {
		return "", "", fmt.Errorf(
			"SSA operation %q at %%%d has operand types %s and %s, want %s",
			instruction.Op,
			instruction.Result.ID,
			leftType,
			rightType,
			operandType,
		)
	}
	declaredType, err := wasmPrimitiveType(instruction.Result.Type)
	if err != nil {
		return "", "", fmt.Errorf("SSA result %%%d: %w", instruction.Result.ID, err)
	}
	if declaredType != resultType {
		return "", "", fmt.Errorf(
			"SSA operation %q at %%%d declares result %s, want %s",
			instruction.Op,
			instruction.Result.ID,
			declaredType,
			resultType,
		)
	}
	return fmt.Sprintf("(%s %s %s)", operator, left, right), resultType, nil
}

func (serializer *ssaSerializer) phi(instruction *ir.Instruction) (string, string, error) {
	if len(instruction.Operands) != 2 || len(instruction.Blocks) != 2 {
		return "", "", fmt.Errorf("SSA Wasm backend requires phi %%%d to have exactly two incoming values", instruction.Result.ID)
	}
	for _, block := range serializer.graph.Blocks {
		terminator := block.Terminator
		if terminator.Kind != ir.TermBranch {
			continue
		}
		trueIndex, falseIndex := -1, -1
		for index, predecessor := range instruction.Blocks {
			switch predecessor {
			case terminator.TrueTarget:
				trueIndex = index
			case terminator.FalseTarget:
				falseIndex = index
			}
		}
		if trueIndex < 0 || falseIndex < 0 {
			continue
		}

		condition, conditionType, err := serializer.expression(terminator.Cond)
		if err != nil {
			return "", "", err
		}
		if conditionType != "i32" {
			return "", "", fmt.Errorf("phi %%%d branch condition has type %s, want bool", instruction.Result.ID, conditionType)
		}
		trueExpression, trueType, err := serializer.expression(instruction.Operands[trueIndex])
		if err != nil {
			return "", "", err
		}
		falseExpression, falseType, err := serializer.expression(instruction.Operands[falseIndex])
		if err != nil {
			return "", "", err
		}
		resultType, err := wasmPrimitiveType(instruction.Result.Type)
		if err != nil {
			return "", "", fmt.Errorf("phi %%%d: %w", instruction.Result.ID, err)
		}
		if trueType != resultType || falseType != resultType {
			return "", "", fmt.Errorf(
				"phi %%%d has incoming types %s and %s, want %s",
				instruction.Result.ID,
				trueType,
				falseType,
				resultType,
			)
		}
		return fmt.Sprintf(
			"(if (result %s) %s (then %s) (else %s))",
			resultType,
			condition,
			trueExpression,
			falseExpression,
		), resultType, nil
	}
	return "", "", fmt.Errorf("phi %%%d is not controlled by a matching branch", instruction.Result.ID)
}

func (serializer *ssaSerializer) unsupported(instruction *ir.Instruction) error {
	location := instruction.Result.Source
	prefix := ""
	if location.Filename != "" {
		prefix = fmt.Sprintf("%s:%d:%d: ", location.Filename, location.Line, location.Column)
	}
	return fmt.Errorf(
		"%sSSA Wasm backend does not support operation %q at %%%d",
		prefix,
		instruction.Op,
		instruction.Result.ID,
	)
}

func supportedSSAOperation(operation ir.SSAOp) bool {
	switch operation {
	case ir.OpConst, ir.OpPhi,
		ir.OpAdd, ir.OpSubtract, ir.OpMultiply, ir.OpDivide,
		ir.OpLess, ir.OpGreater, ir.OpLessEqual, ir.OpGreaterEqual,
		ir.OpEqual, ir.OpNotEqual, ir.OpAnd, ir.OpOr,
		ir.OpParam, ir.OpCall:
		return true
	default:
		return false
	}
}

func ssaWasmOperator(operation ir.SSAOp, operandType string) (string, string, string) {
	if operation == ir.OpAnd {
		return "i32.and", "i32", "i32"
	}
	if operation == ir.OpOr {
		return "i32.or", "i32", "i32"
	}
	if operation == ir.OpEqual || operation == ir.OpNotEqual {
		suffix := "eq"
		if operation == ir.OpNotEqual {
			suffix = "ne"
		}
		if operandType == "i64" || operandType == "i32" || operandType == "f64" {
			return operandType + "." + suffix, operandType, "i32"
		}
		return "", "", ""
	}
	if operandType == "f64" {
		operators := map[ir.SSAOp]string{
			ir.OpAdd:          "f64.add",
			ir.OpSubtract:     "f64.sub",
			ir.OpMultiply:     "f64.mul",
			ir.OpDivide:       "f64.div",
			ir.OpLess:         "f64.lt",
			ir.OpGreater:      "f64.gt",
			ir.OpLessEqual:    "f64.le",
			ir.OpGreaterEqual: "f64.ge",
		}
		operator := operators[operation]
		if operator == "" {
			return "", "", ""
		}
		resultType := "f64"
		if operation == ir.OpLess || operation == ir.OpGreater ||
			operation == ir.OpLessEqual || operation == ir.OpGreaterEqual {
			resultType = "i32"
		}
		return operator, "f64", resultType
	}
	operators := map[ir.SSAOp]string{
		ir.OpAdd:          "i64.add",
		ir.OpSubtract:     "i64.sub",
		ir.OpMultiply:     "i64.mul",
		ir.OpDivide:       "i64.div_s",
		ir.OpLess:         "i64.lt_s",
		ir.OpGreater:      "i64.gt_s",
		ir.OpLessEqual:    "i64.le_s",
		ir.OpGreaterEqual: "i64.ge_s",
	}
	operator := operators[operation]
	if operator == "" {
		return "", "", ""
	}
	resultType := "i64"
	if operation == ir.OpLess || operation == ir.OpGreater ||
		operation == ir.OpLessEqual || operation == ir.OpGreaterEqual {
		resultType = "i32"
	}
	return operator, "i64", resultType
}

func wasmPrimitiveType(info ast.TypeInfo) (string, error) {
	switch info.Kind {
	case ast.Int:
		return "i64", nil
	case ast.Bool:
		return "i32", nil
	case ast.Float:
		return "f64", nil
	default:
		return "", fmt.Errorf("unsupported primitive type %s", primitiveName(info))
	}
}

func primitiveName(info ast.TypeInfo) string {
	if info.Name != "" {
		return info.Name
	}
	if info.Kind != "" {
		return string(info.Kind)
	}
	return "unknown"
}
