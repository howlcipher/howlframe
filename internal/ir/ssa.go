package ir

import (
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
	"sort"
	"strconv"
)

// ValueID is a graph-wide identifier for one SSA definition. The zero value is
// reserved for terminators and expressions that do not produce a value.
type ValueID uint64

// BlockLabel is a stable, human-readable basic block identifier.
type BlockLabel string

// SourceLocation retains enough of the source AST location for diagnostics.
type SourceLocation struct {
	Filename string
	Line     int
	Column   int
}

// Value is the result defined by an instruction.
type Value struct {
	ID     ValueID
	Type   ast.TypeInfo
	Source SourceLocation
}

// SSAOp identifies a flat instruction operation.
type SSAOp string

const (
	OpConst         SSAOp = "const"
	OpSymbol        SSAOp = "symbol"
	OpPhi           SSAOp = "phi"
	OpUnit          SSAOp = "unit"
	OpAdd           SSAOp = "add"
	OpSubtract      SSAOp = "subtract"
	OpMultiply      SSAOp = "multiply"
	OpDivide        SSAOp = "divide"
	OpLess          SSAOp = "less"
	OpGreater       SSAOp = "greater"
	OpLessEqual     SSAOp = "less_equal"
	OpGreaterEqual  SSAOp = "greater_equal"
	OpEqual         SSAOp = "equal"
	OpNotEqual      SSAOp = "not_equal"
	OpAnd           SSAOp = "and"
	OpOr            SSAOp = "or"
	OpSet           SSAOp = "set"
	OpCall          SSAOp = "call"
	OpParam         SSAOp = "param"
	OpList          SSAOp = "list"
	OpDict          SSAOp = "dict"
	OpMapGet        SSAOp = "map_get"
	OpListGet       SSAOp = "list_get"
	OpPrint         SSAOp = "print"
	OpToInt         SSAOp = "to_int"
	OpToFloat       SSAOp = "to_float"
	OpToString      SSAOp = "to_string"
	OpBytesToString SSAOp = "bytes_to_string"
)

// Instruction defines exactly one SSA value. Blocks is populated for phi
// instructions and is positionally aligned with Operands.
type Instruction struct {
	Result   Value
	Op       SSAOp
	Operands []ValueID
	Blocks   []BlockLabel
	Literal  string
	Symbol   string
}

// TerminatorKind identifies the control transfer ending a basic block.
type TerminatorKind string

const (
	TermJump   TerminatorKind = "jump"
	TermBranch TerminatorKind = "branch"
	TermReturn TerminatorKind = "return"
)

// Terminator is an explicit control-flow edge. Value is optional for a void
// return and Cond is required for a branch.
type Terminator struct {
	Kind        TerminatorKind
	Cond        ValueID
	Value       ValueID
	Target      BlockLabel
	TrueTarget  BlockLabel
	FalseTarget BlockLabel
	Source      SourceLocation
}

// BasicBlock is ordered in Graph.Blocks and contains instructions in source
// evaluation order.
type BasicBlock struct {
	Label        BlockLabel
	Instructions []Instruction
	Terminator   *Terminator
}

// Graph is a flat SSA control-flow graph for one expression or function body.
type Graph struct {
	Entry  BlockLabel
	Blocks []*BasicBlock
}

// LowerSSA lowers a checker-annotated shared AST expression into a flat SSA
// graph. It does not mutate the source tree or affect existing tree-IR users.
func LowerSSA(node *ast.Node) (*Graph, error) {
	if node == nil {
		return nil, fmt.Errorf("cannot lower a nil AST node")
	}
	builder := newSSABuilder()
	value, err := builder.lower(node)
	if err != nil {
		return nil, err
	}
	if builder.current != nil && builder.current.Terminator == nil {
		builder.current.Terminator = &Terminator{
			Kind:   TermReturn,
			Value:  value,
			Source: sourceLocation(node),
		}
	}
	graph := &Graph{Entry: "entry", Blocks: builder.blocks}
	if err := graph.Validate(); err != nil {
		return nil, fmt.Errorf("lowered invalid SSA graph: %w", err)
	}
	return graph, nil
}

// Param describes one function parameter for LowerSSAFunction: the name it
// is referenced by within the function body, and its checker-inferred type.
type Param struct {
	Name string
	Type ast.TypeInfo
}

// LowerSSAFunction lowers one function body into its own flat SSA graph,
// pre-binding each parameter as a resolvable SYMBOL reference via an OpParam
// instruction in the entry block, so the body's ordinary SYMBOL-lowering
// path in lower() resolves parameter references exactly like any other name
// bound by builder.env (e.g. a let binding).
func LowerSSAFunction(params []Param, body *ast.Node) (*Graph, error) {
	if body == nil {
		return nil, fmt.Errorf("cannot lower a nil function body")
	}
	builder := newSSABuilder()
	for index, param := range params {
		value := builder.emit(OpParam, nil, param.Name, strconv.Itoa(index), param.Type, nil)
		builder.env[param.Name] = value
	}
	value, err := builder.lower(body)
	if err != nil {
		return nil, err
	}
	if builder.current != nil && builder.current.Terminator == nil {
		builder.current.Terminator = &Terminator{
			Kind:   TermReturn,
			Value:  value,
			Source: sourceLocation(body),
		}
	}
	graph := &Graph{Entry: "entry", Blocks: builder.blocks}
	if err := graph.Validate(); err != nil {
		return nil, fmt.Errorf("lowered invalid SSA graph for function: %w", err)
	}
	return graph, nil
}

// Validate verifies structural and value-reference invariants for a graph.
func (graph *Graph) Validate() error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	if graph.Entry == "" {
		return fmt.Errorf("graph has no entry block")
	}

	labels := make(map[BlockLabel]struct{}, len(graph.Blocks))
	values := make(map[ValueID]struct{})
	for index, block := range graph.Blocks {
		if block == nil {
			return fmt.Errorf("block %d is nil", index)
		}
		if block.Label == "" {
			return fmt.Errorf("block %d has an empty label", index)
		}
		if _, exists := labels[block.Label]; exists {
			return fmt.Errorf("duplicate block label %q", block.Label)
		}
		labels[block.Label] = struct{}{}
		for instructionIndex, instruction := range block.Instructions {
			if instruction.Result.ID == 0 {
				return fmt.Errorf("instruction %d in block %q has invalid result %%0", instructionIndex, block.Label)
			}
			if _, exists := values[instruction.Result.ID]; exists {
				return fmt.Errorf("duplicate SSA value %%%d", instruction.Result.ID)
			}
			values[instruction.Result.ID] = struct{}{}
		}
	}
	if _, exists := labels[graph.Entry]; !exists {
		return fmt.Errorf("entry block %q does not exist", graph.Entry)
	}

	for _, block := range graph.Blocks {
		if block.Terminator == nil {
			return fmt.Errorf("block %q has no terminator", block.Label)
		}
		for instructionIndex, instruction := range block.Instructions {
			for _, operand := range instruction.Operands {
				if _, exists := values[operand]; !exists {
					return fmt.Errorf("instruction %d in block %q references undefined value %%%d", instructionIndex, block.Label, operand)
				}
			}
			if instruction.Op == OpPhi {
				if len(instruction.Blocks) != len(instruction.Operands) || len(instruction.Blocks) == 0 {
					return fmt.Errorf("phi %%%d in block %q has mismatched incoming blocks and operands", instruction.Result.ID, block.Label)
				}
				for _, predecessor := range instruction.Blocks {
					if _, exists := labels[predecessor]; !exists {
						return fmt.Errorf("phi %%%d predecessor %q does not exist", instruction.Result.ID, predecessor)
					}
				}
			} else if len(instruction.Blocks) != 0 {
				return fmt.Errorf("non-phi instruction %%%d has incoming blocks", instruction.Result.ID)
			}
		}
		if err := validateTerminator(block, labels, values); err != nil {
			return err
		}
	}
	return nil
}

// ValidateGraph is the function form of Graph.Validate.
func ValidateGraph(graph *Graph) error {
	return graph.Validate()
}

func validateTerminator(block *BasicBlock, labels map[BlockLabel]struct{}, values map[ValueID]struct{}) error {
	terminator := block.Terminator
	switch terminator.Kind {
	case TermJump:
		if _, exists := labels[terminator.Target]; !exists {
			return fmt.Errorf("jump target %q does not exist", terminator.Target)
		}
	case TermBranch:
		if _, exists := values[terminator.Cond]; !exists {
			return fmt.Errorf("branch in block %q references undefined value %%%d", block.Label, terminator.Cond)
		}
		if _, exists := labels[terminator.TrueTarget]; !exists {
			return fmt.Errorf("branch target %q does not exist", terminator.TrueTarget)
		}
		if _, exists := labels[terminator.FalseTarget]; !exists {
			return fmt.Errorf("branch target %q does not exist", terminator.FalseTarget)
		}
	case TermReturn:
		if terminator.Value != 0 {
			if _, exists := values[terminator.Value]; !exists {
				return fmt.Errorf("return in block %q references undefined value %%%d", block.Label, terminator.Value)
			}
		}
	default:
		return fmt.Errorf("block %q has unknown terminator %q", block.Label, terminator.Kind)
	}
	return nil
}

type ssaBuilder struct {
	blocks      []*BasicBlock
	current     *BasicBlock
	env         map[string]ValueID
	nextValue   ValueID
	nextBlockID uint64
}

func newSSABuilder() *ssaBuilder {
	entry := &BasicBlock{Label: "entry"}
	return &ssaBuilder{
		blocks:    []*BasicBlock{entry},
		current:   entry,
		env:       make(map[string]ValueID),
		nextValue: 1,
	}
}

func (builder *ssaBuilder) lower(node *ast.Node) (ValueID, error) {
	if node == nil {
		return 0, fmt.Errorf("cannot lower a nil AST node")
	}
	if builder.current == nil {
		return 0, nil
	}
	switch node.Type {
	case "INT", "STRING", "FLOAT":
		return builder.emit(OpConst, nil, "", node.Value, node.Inferred, node), nil
	case "SYMBOL":
		if node.Value == "true" || node.Value == "false" {
			return builder.emit(OpConst, nil, "", node.Value, node.Inferred, node), nil
		}
		if value, exists := builder.env[node.Value]; exists {
			return value, nil
		}
		return builder.emit(OpSymbol, nil, node.Value, "", node.Inferred, node), nil
	case "List":
		return builder.lowerList(node)
	default:
		return 0, builder.errorAt(node, "unsupported literal type %q", node.Type)
	}
}

func (builder *ssaBuilder) lowerList(node *ast.Node) (ValueID, error) {
	shared, ok := LowerShared(node)
	if !ok {
		if node == nil || len(node.Children) == 0 {
			return 0, builder.errorAt(node, "cannot lower an empty expression")
		}
		return 0, builder.errorAt(node, "SSA lowering does not support %q", node.Children[0].Value)
	}
	switch shared.Kind {
	case "binop":
		left, err := builder.lower(shared.Kids[0])
		if err != nil {
			return 0, err
		}
		right, err := builder.lower(shared.Kids[1])
		if err != nil {
			return 0, err
		}
		return builder.emit(binarySSAOp(shared.Op), []ValueID{left, right}, "", "", node.Inferred, node), nil
	case "do":
		return builder.lowerDo(shared.Kids, node)
	case "let":
		return builder.lowerLet(node)
	case "set":
		return builder.lowerSet(shared, node)
	case "return":
		return builder.lowerReturn(shared, node)
	case "if":
		return builder.lowerIf(shared, node)
	case "while":
		return builder.lowerWhile(shared, node)
	case "call":
		return builder.lowerCall(shared, node)
	case "list":
		operands, err := builder.lowerOperands(shared.Kids)
		if err != nil {
			return 0, err
		}
		return builder.emit(OpList, operands, "", "", node.Inferred, node), nil
	case "dict":
		var operands []ValueID
		for _, pair := range shared.Kids {
			key, err := builder.lower(pair.Children[0])
			if err != nil {
				return 0, err
			}
			value, err := builder.lower(pair.Children[1])
			if err != nil {
				return 0, err
			}
			operands = append(operands, key, value)
		}
		return builder.emit(OpDict, operands, "", "", node.Inferred, node), nil
	case "map_get":
		return builder.lowerSimple(OpMapGet, shared.Kids, node)
	case "list_get":
		return builder.lowerSimple(OpListGet, shared.Kids, node)
	case "print":
		return builder.lowerSimple(OpPrint, shared.Kids, node)
	case "to_int":
		return builder.lowerSimple(OpToInt, shared.Kids, node)
	case "to_float":
		return builder.lowerSimple(OpToFloat, shared.Kids, node)
	case "to_string":
		return builder.lowerSimple(OpToString, shared.Kids, node)
	case "bytes_to_string":
		return builder.lowerSimple(OpBytesToString, shared.Kids, node)
	default:
		return 0, builder.errorAt(node, "SSA lowering does not support %q", shared.Kind)
	}
}

func (builder *ssaBuilder) lowerDo(nodes []*ast.Node, source *ast.Node) (ValueID, error) {
	var result ValueID
	for _, node := range nodes {
		if builder.current == nil || builder.current.Terminator != nil {
			break
		}
		value, err := builder.lower(node)
		if err != nil {
			return 0, err
		}
		result = value
	}
	if result == 0 && builder.current != nil && builder.current.Terminator == nil {
		result = builder.emit(OpUnit, nil, "", "", ast.Layout(ast.Void), source)
	}
	return result, nil
}

func (builder *ssaBuilder) lowerLet(node *ast.Node) (ValueID, error) {
	bindings, body := ast.LetChain(node)
	if len(bindings) == 0 || body == nil {
		return 0, builder.errorAt(node, "malformed let chain")
	}
	outer := cloneSSAEnv(builder.env)
	boundNames := make(map[string]struct{}, len(bindings))
	for _, binding := range bindings {
		nameNode := binding.Children[0]
		if nameNode.Type != "SYMBOL" {
			return 0, builder.errorAt(nameNode, "let binding name must be a symbol")
		}
		value, err := builder.lower(binding.Children[1])
		if err != nil {
			return 0, err
		}
		builder.env[nameNode.Value] = value
		boundNames[nameNode.Value] = struct{}{}
	}
	result, err := builder.lower(body)
	if err != nil {
		return 0, err
	}
	for name := range boundNames {
		if value, exists := outer[name]; exists {
			builder.env[name] = value
		} else {
			delete(builder.env, name)
		}
	}
	return result, nil
}

func (builder *ssaBuilder) lowerSet(shared *IRNode, node *ast.Node) (ValueID, error) {
	target := shared.Kids[0]
	if target.Type != "SYMBOL" {
		return 0, builder.errorAt(target, "set target must be a symbol")
	}
	value, err := builder.lower(shared.Kids[1])
	if err != nil {
		return 0, err
	}
	assignedType := shared.Kids[1].Inferred
	assigned := builder.emit(OpSet, []ValueID{value}, target.Value, "", assignedType, node)
	builder.env[target.Value] = assigned
	return builder.emit(OpUnit, nil, "", "", node.Inferred, node), nil
}

func (builder *ssaBuilder) lowerReturn(shared *IRNode, node *ast.Node) (ValueID, error) {
	value, err := builder.lower(shared.Kids[0])
	if err != nil {
		return 0, err
	}
	builder.current.Terminator = &Terminator{
		Kind:   TermReturn,
		Value:  value,
		Source: sourceLocation(node),
	}
	return 0, nil
}

func (builder *ssaBuilder) lowerIf(shared *IRNode, node *ast.Node) (ValueID, error) {
	condition, err := builder.lower(shared.Kids[0])
	if err != nil {
		return 0, err
	}
	branchBlock := builder.current
	baseEnv := cloneSSAEnv(builder.env)
	thenBlock := builder.newBlock("if.then")
	elseBlock := builder.newBlock("if.else")
	mergeBlock := builder.newBlock("if.merge")
	branchBlock.Terminator = &Terminator{
		Kind:        TermBranch,
		Cond:        condition,
		TrueTarget:  thenBlock.Label,
		FalseTarget: elseBlock.Label,
		Source:      sourceLocation(node),
	}

	builder.current = thenBlock
	builder.env = cloneSSAEnv(baseEnv)
	thenValue, err := builder.lower(shared.Kids[1])
	if err != nil {
		return 0, err
	}
	thenEnv := cloneSSAEnv(builder.env)
	thenPredecessor := builder.finishBranch(mergeBlock.Label, node)

	builder.current = elseBlock
	builder.env = cloneSSAEnv(baseEnv)
	var elseValue ValueID
	if len(shared.Kids) == 3 {
		elseValue, err = builder.lower(shared.Kids[2])
		if err != nil {
			return 0, err
		}
	} else {
		elseValue = builder.emit(OpUnit, nil, "", "", ast.Layout(ast.Void), node)
	}
	elseEnv := cloneSSAEnv(builder.env)
	elsePredecessor := builder.finishBranch(mergeBlock.Label, node)

	var predecessors []BlockLabel
	var values []ValueID
	var environments []map[string]ValueID
	if thenPredecessor != "" {
		predecessors = append(predecessors, thenPredecessor)
		values = append(values, thenValue)
		environments = append(environments, thenEnv)
	}
	if elsePredecessor != "" {
		predecessors = append(predecessors, elsePredecessor)
		values = append(values, elseValue)
		environments = append(environments, elseEnv)
	}
	if len(predecessors) == 0 {
		builder.current = nil
		builder.env = baseEnv
		builder.removeBlock(mergeBlock)
		return 0, nil
	}

	builder.current = mergeBlock
	builder.env = builder.mergeEnvironments(baseEnv, predecessors, environments, node)
	if len(values) == 1 {
		return values[0], nil
	}
	if values[0] == values[1] {
		return values[0], nil
	}
	return builder.emitPhi(mergeBlock, values, predecessors, node.Inferred, "", node), nil
}

func (builder *ssaBuilder) lowerWhile(shared *IRNode, node *ast.Node) (ValueID, error) {
	preheader := builder.current
	preheaderEnv := cloneSSAEnv(builder.env)
	header := builder.newBlock("while.header")
	body := builder.newBlock("while.body")
	exit := builder.newBlock("while.exit")
	preheader.Terminator = &Terminator{Kind: TermJump, Target: header.Label, Source: sourceLocation(node)}

	assignedNames := assignedSSAValues(shared.Kids[1])
	loopEnv := cloneSSAEnv(preheaderEnv)
	phiIndexes := make(map[string]int)
	names := make([]string, 0, len(assignedNames))
	for name := range assignedNames {
		if _, exists := preheaderEnv[name]; exists {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		incoming := preheaderEnv[name]
		valueType := builder.valueType(incoming)
		result := builder.newValue(valueType, node)
		header.Instructions = append(header.Instructions, Instruction{
			Result:   result,
			Op:       OpPhi,
			Operands: []ValueID{incoming},
			Blocks:   []BlockLabel{preheader.Label},
			Symbol:   name,
		})
		phiIndexes[name] = len(header.Instructions) - 1
		loopEnv[name] = result.ID
	}

	builder.current = header
	builder.env = loopEnv
	condition, err := builder.lower(shared.Kids[0])
	if err != nil {
		return 0, err
	}
	header.Terminator = &Terminator{
		Kind:        TermBranch,
		Cond:        condition,
		TrueTarget:  body.Label,
		FalseTarget: exit.Label,
		Source:      sourceLocation(node),
	}

	builder.current = body
	builder.env = cloneSSAEnv(loopEnv)
	if _, err := builder.lower(shared.Kids[1]); err != nil {
		return 0, err
	}
	bodyEnv := cloneSSAEnv(builder.env)
	backedge := builder.finishBranch(header.Label, node)
	if backedge != "" {
		for name, index := range phiIndexes {
			instruction := &header.Instructions[index]
			instruction.Operands = append(instruction.Operands, bodyEnv[name])
			instruction.Blocks = append(instruction.Blocks, backedge)
		}
	}

	builder.current = exit
	builder.env = loopEnv
	return builder.emit(OpUnit, nil, "", "", node.Inferred, node), nil
}

func (builder *ssaBuilder) lowerCall(shared *IRNode, node *ast.Node) (ValueID, error) {
	function := shared.Kids[0]
	if function.Type != "SYMBOL" {
		return 0, builder.errorAt(function, "call function must be a symbol")
	}
	operands, err := builder.lowerOperands(shared.Kids[1:])
	if err != nil {
		return 0, err
	}
	return builder.emit(OpCall, operands, function.Value, "", node.Inferred, node), nil
}

func (builder *ssaBuilder) lowerSimple(op SSAOp, nodes []*ast.Node, source *ast.Node) (ValueID, error) {
	operands, err := builder.lowerOperands(nodes)
	if err != nil {
		return 0, err
	}
	return builder.emit(op, operands, "", "", source.Inferred, source), nil
}

func (builder *ssaBuilder) lowerOperands(nodes []*ast.Node) ([]ValueID, error) {
	operands := make([]ValueID, 0, len(nodes))
	for _, node := range nodes {
		value, err := builder.lower(node)
		if err != nil {
			return nil, err
		}
		if value == 0 {
			return nil, builder.errorAt(node, "expression does not produce an SSA value")
		}
		operands = append(operands, value)
	}
	return operands, nil
}

func (builder *ssaBuilder) emit(op SSAOp, operands []ValueID, symbol, literal string, valueType ast.TypeInfo, node *ast.Node) ValueID {
	result := builder.newValue(valueType, node)
	builder.current.Instructions = append(builder.current.Instructions, Instruction{
		Result:   result,
		Op:       op,
		Operands: operands,
		Literal:  literal,
		Symbol:   symbol,
	})
	return result.ID
}

func (builder *ssaBuilder) emitPhi(block *BasicBlock, operands []ValueID, predecessors []BlockLabel, valueType ast.TypeInfo, symbol string, node *ast.Node) ValueID {
	result := builder.newValue(valueType, node)
	block.Instructions = append(block.Instructions, Instruction{
		Result:   result,
		Op:       OpPhi,
		Operands: append([]ValueID(nil), operands...),
		Blocks:   append([]BlockLabel(nil), predecessors...),
		Symbol:   symbol,
	})
	return result.ID
}

func (builder *ssaBuilder) newValue(valueType ast.TypeInfo, node *ast.Node) Value {
	result := Value{ID: builder.nextValue, Type: valueType, Source: sourceLocation(node)}
	builder.nextValue++
	return result
}

func (builder *ssaBuilder) newBlock(prefix string) *BasicBlock {
	builder.nextBlockID++
	block := &BasicBlock{Label: BlockLabel(fmt.Sprintf("%s.%d", prefix, builder.nextBlockID))}
	builder.blocks = append(builder.blocks, block)
	return block
}

func (builder *ssaBuilder) removeBlock(target *BasicBlock) {
	for index, block := range builder.blocks {
		if block == target {
			builder.blocks = append(builder.blocks[:index], builder.blocks[index+1:]...)
			return
		}
	}
}

func (builder *ssaBuilder) finishBranch(target BlockLabel, node *ast.Node) BlockLabel {
	if builder.current == nil || builder.current.Terminator != nil {
		return ""
	}
	predecessor := builder.current.Label
	builder.current.Terminator = &Terminator{Kind: TermJump, Target: target, Source: sourceLocation(node)}
	return predecessor
}

func (builder *ssaBuilder) mergeEnvironments(base map[string]ValueID, predecessors []BlockLabel, environments []map[string]ValueID, node *ast.Node) map[string]ValueID {
	result := cloneSSAEnv(base)
	names := make(map[string]struct{})
	for _, environment := range environments {
		for name := range environment {
			names[name] = struct{}{}
		}
	}
	orderedNames := make([]string, 0, len(names))
	for name := range names {
		orderedNames = append(orderedNames, name)
	}
	sort.Strings(orderedNames)
	for _, name := range orderedNames {
		operands := make([]ValueID, len(environments))
		complete := true
		for index, environment := range environments {
			value, exists := environment[name]
			if !exists {
				complete = false
				break
			}
			operands[index] = value
		}
		if !complete {
			continue
		}
		if len(operands) == 1 || allSameSSAValue(operands) {
			result[name] = operands[0]
			continue
		}
		valueType := builder.valueType(operands[0])
		result[name] = builder.emitPhi(builder.current, operands, predecessors, valueType, name, node)
	}
	return result
}

func (builder *ssaBuilder) valueType(id ValueID) ast.TypeInfo {
	for _, block := range builder.blocks {
		for _, instruction := range block.Instructions {
			if instruction.Result.ID == id {
				return instruction.Result.Type
			}
		}
	}
	return ast.Layout(ast.Unknown)
}

func (builder *ssaBuilder) errorAt(node *ast.Node, format string, arguments ...any) error {
	location := sourceLocation(node)
	message := fmt.Sprintf(format, arguments...)
	if location.Filename != "" {
		return fmt.Errorf("%s:%d:%d: %s", location.Filename, location.Line, location.Column, message)
	}
	return fmt.Errorf("%d:%d: %s", location.Line, location.Column, message)
}

func binarySSAOp(op string) SSAOp {
	switch op {
	case "+":
		return OpAdd
	case "-":
		return OpSubtract
	case "*":
		return OpMultiply
	case "/":
		return OpDivide
	case "<":
		return OpLess
	case ">":
		return OpGreater
	case "<=":
		return OpLessEqual
	case ">=":
		return OpGreaterEqual
	case "==", "=":
		return OpEqual
	case "!=":
		return OpNotEqual
	case "and":
		return OpAnd
	case "or":
		return OpOr
	default:
		return SSAOp(op)
	}
}

func sourceLocation(node *ast.Node) SourceLocation {
	if node == nil {
		return SourceLocation{}
	}
	return SourceLocation{Filename: node.Filename, Line: node.Line, Column: node.Column}
}

func cloneSSAEnv(environment map[string]ValueID) map[string]ValueID {
	result := make(map[string]ValueID, len(environment))
	for name, value := range environment {
		result[name] = value
	}
	return result
}

func assignedSSAValues(node *ast.Node) map[string]struct{} {
	result := make(map[string]struct{})
	var visit func(*ast.Node)
	visit = func(current *ast.Node) {
		if current == nil || current.Type != "List" {
			return
		}
		if len(current.Children) >= 3 && current.Children[0].Type == "SYMBOL" &&
			current.Children[0].Value == "set" && current.Children[1].Type == "SYMBOL" {
			result[current.Children[1].Value] = struct{}{}
		}
		for _, child := range current.Children[1:] {
			visit(child)
		}
	}
	visit(node)
	return result
}

func allSameSSAValue(values []ValueID) bool {
	for index := 1; index < len(values); index++ {
		if values[index] != values[0] {
			return false
		}
	}
	return true
}
