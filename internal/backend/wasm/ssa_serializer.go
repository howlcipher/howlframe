package wasm

import (
	"fmt"
	"strconv"
	"zero/internal/ast"
	"zero/internal/ir"
)

// SerializeSSA emits a standalone WAT module from a typed, acyclic SSA graph.
// The first slice deliberately supports primitive numeric and boolean values
// plus canonical if/phi control flow. Unsupported graph operations return an
// error instead of falling back to AST code generation.
func SerializeSSA(graph *ir.Graph) (string, error) {
	if err := graph.Validate(); err != nil {
		return "", fmt.Errorf("invalid SSA graph: %w", err)
	}

	serializer := &ssaSerializer{
		graph:        graph,
		blocks:       make(map[ir.BlockLabel]*ir.BasicBlock, len(graph.Blocks)),
		instructions: make(map[ir.ValueID]*ir.Instruction),
	}
	for _, block := range graph.Blocks {
		serializer.blocks[block.Label] = block
		for index := range block.Instructions {
			instruction := &block.Instructions[index]
			serializer.instructions[instruction.Result.ID] = instruction
			if !supportedSSAOperation(instruction.Op) {
				return "", serializer.unsupported(instruction)
			}
		}
	}
	if err := serializer.rejectCycles(); err != nil {
		return "", err
	}

	body, resultType, err := serializer.functionBody()
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

type ssaSerializer struct {
	graph        *ir.Graph
	blocks       map[ir.BlockLabel]*ir.BasicBlock
	instructions map[ir.ValueID]*ir.Instruction
	resolving    map[ir.ValueID]bool
}

type ssaReturn struct {
	block *ir.BasicBlock
	value ir.ValueID
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
		return serializer.phi(instruction)
	default:
		return serializer.binary(instruction)
	}
}

func (serializer *ssaSerializer) constant(instruction *ir.Instruction) (string, string, error) {
	switch instruction.Result.Type.Kind {
	case ast.Int:
		if _, err := strconv.ParseInt(instruction.Literal, 10, 64); err != nil {
			return "", "", fmt.Errorf("SSA int constant %%%d has invalid literal %q", instruction.Result.ID, instruction.Literal)
		}
		return fmt.Sprintf("(i64.const %s)", instruction.Literal), "i64", nil
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

func (serializer *ssaSerializer) rejectCycles() error {
	state := make(map[ir.BlockLabel]uint8)
	var visit func(ir.BlockLabel) error
	visit = func(label ir.BlockLabel) error {
		switch state[label] {
		case 1:
			return fmt.Errorf("SSA Wasm backend does not support loops involving block %q", label)
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
			if err := visit(target); err != nil {
				return err
			}
		}
		state[label] = 2
		return nil
	}
	return visit(serializer.graph.Entry)
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
		ir.OpEqual, ir.OpNotEqual, ir.OpAnd, ir.OpOr:
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
		if operandType == "i64" || operandType == "i32" {
			return operandType + "." + suffix, operandType, "i32"
		}
		return "", "", ""
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
