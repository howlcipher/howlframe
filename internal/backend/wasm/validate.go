package wasm

import (
	"fmt"
	"strconv"
	"strings"
)

type watToken struct {
	value    string
	position int
	quoted   bool
}

type watForm struct {
	head     string
	args     []watValue
	position int
}

type watValue struct {
	atom   string
	quoted bool
	form   *watForm
}

const (
	watVoid   = "void"
	watBottom = "bottom"
	wasmPage  = uint64(65536)
)

// ValidateWAT parses and type-checks the folded instruction subset emitted by
// this backend. External tools remain the authority for the complete WAT
// grammar, but malformed instructions cannot pass merely because parentheses
// happen to balance.
func ValidateWAT(source string) error {
	tokens, err := tokenizeWAT(source)
	if err != nil {
		return err
	}
	index := 0
	root, err := parseWATForm(tokens, &index)
	if err != nil {
		return err
	}
	if index != len(tokens) {
		return fmt.Errorf("unexpected token %q at byte %d", tokens[index].value, tokens[index].position)
	}
	if root.head != "module" {
		return fmt.Errorf("WAT must start with a module")
	}
	return validateModule(root)
}

func tokenizeWAT(source string) ([]watToken, error) {
	var tokens []watToken
	for index := 0; index < len(source); {
		switch {
		case source[index] == ' ' || source[index] == '\t' || source[index] == '\r' || source[index] == '\n':
			index++
		case strings.HasPrefix(source[index:], ";;"):
			for index < len(source) && source[index] != '\n' {
				index++
			}
		case strings.HasPrefix(source[index:], "(;"):
			start := index
			depth := 1
			index += 2
			for index < len(source) && depth > 0 {
				switch {
				case strings.HasPrefix(source[index:], "(;"):
					depth++
					index += 2
				case strings.HasPrefix(source[index:], ";)"):
					depth--
					index += 2
				default:
					index++
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("unterminated block comment at byte %d", start)
			}
		case source[index] == '(' || source[index] == ')':
			tokens = append(tokens, watToken{value: source[index : index+1], position: index})
			index++
		case source[index] == '"':
			start := index
			index++
			var value strings.Builder
			for index < len(source) && source[index] != '"' {
				if source[index] == '\\' {
					if index+1 >= len(source) {
						return nil, fmt.Errorf("unterminated string at byte %d", start)
					}
					value.WriteByte(source[index])
					index++
				}
				value.WriteByte(source[index])
				index++
			}
			if index >= len(source) {
				return nil, fmt.Errorf("unterminated string at byte %d", start)
			}
			index++
			tokens = append(tokens, watToken{value: value.String(), position: start, quoted: true})
		default:
			start := index
			for index < len(source) &&
				source[index] != '(' && source[index] != ')' &&
				source[index] != ' ' && source[index] != '\t' &&
				source[index] != '\r' && source[index] != '\n' {
				index++
			}
			if start == index {
				return nil, fmt.Errorf("unexpected byte at %d", index)
			}
			tokens = append(tokens, watToken{value: source[start:index], position: start})
		}
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty WAT source")
	}
	return tokens, nil
}

func parseWATForm(tokens []watToken, index *int) (*watForm, error) {
	if *index >= len(tokens) || tokens[*index].value != "(" {
		if *index >= len(tokens) {
			return nil, fmt.Errorf("expected form at end of WAT")
		}
		return nil, fmt.Errorf("expected form at byte %d", tokens[*index].position)
	}
	position := tokens[*index].position
	*index++
	if *index >= len(tokens) || tokens[*index].value == "(" || tokens[*index].value == ")" || tokens[*index].quoted {
		return nil, fmt.Errorf("expected form name at byte %d", position)
	}
	form := &watForm{head: tokens[*index].value, position: position}
	*index++
	for *index < len(tokens) && tokens[*index].value != ")" {
		if tokens[*index].value == "(" {
			child, err := parseWATForm(tokens, index)
			if err != nil {
				return nil, err
			}
			form.args = append(form.args, watValue{form: child})
			continue
		}
		form.args = append(form.args, watValue{atom: tokens[*index].value, quoted: tokens[*index].quoted})
		*index++
	}
	if *index >= len(tokens) {
		return nil, fmt.Errorf("unclosed %q form at byte %d", form.head, position)
	}
	*index++
	return form, nil
}

func validateModule(module *watForm) error {
	memoryBytes := uint64(0)
	functions := 0
	for _, value := range module.args {
		if value.form == nil {
			return fmt.Errorf("module contains unexpected atom %q", value.atom)
		}
		switch value.form.head {
		case "memory":
			pages, err := validateMemory(value.form)
			if err != nil {
				return err
			}
			memoryBytes += pages * wasmPage
		case "data":
			if err := validateData(value.form, memoryBytes); err != nil {
				return err
			}
		case "func":
			functions++
			if err := validateFunction(value.form, memoryBytes > 0); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported module field %q at byte %d", value.form.head, value.form.position)
		}
	}
	if functions == 0 {
		return fmt.Errorf("WAT module must contain a function")
	}
	return nil
}

func validateMemory(form *watForm) (uint64, error) {
	index := 0
	if len(form.args) > 0 && form.args[0].form != nil && form.args[0].form.head == "export" {
		if err := validateExport(form.args[0].form); err != nil {
			return 0, err
		}
		index++
	}
	if len(form.args) != index+1 || form.args[index].form != nil || form.args[index].quoted {
		return 0, fmt.Errorf("memory requires one page count at byte %d", form.position)
	}
	pages, err := strconv.ParseUint(form.args[index].atom, 10, 32)
	if err != nil || pages == 0 {
		return 0, fmt.Errorf("invalid memory page count %q at byte %d", form.args[index].atom, form.position)
	}
	return pages, nil
}

func validateData(form *watForm, memoryBytes uint64) error {
	if memoryBytes == 0 {
		return fmt.Errorf("data segment requires declared memory at byte %d", form.position)
	}
	if len(form.args) != 2 || form.args[0].form == nil || !form.args[1].quoted {
		return fmt.Errorf("data requires an offset and byte string at byte %d", form.position)
	}
	offset, err := constantI32(form.args[0].form)
	if err != nil {
		return fmt.Errorf("invalid data offset: %w", err)
	}
	length, err := watStringLength(form.args[1].atom)
	if err != nil {
		return fmt.Errorf("invalid data payload at byte %d: %w", form.position, err)
	}
	if uint64(offset)+uint64(length) > memoryBytes {
		return fmt.Errorf("data segment ending at %d exceeds memory size %d", uint64(offset)+uint64(length), memoryBytes)
	}
	return nil
}

func validateFunction(form *watForm, hasMemory bool) error {
	index := 0
	if len(form.args) > 0 && form.args[0].form != nil && form.args[0].form.head == "export" {
		if err := validateExport(form.args[0].form); err != nil {
			return err
		}
		index++
	}
	if index >= len(form.args) || form.args[index].form == nil || form.args[index].form.head != "result" {
		return fmt.Errorf("function requires one result declaration at byte %d", form.position)
	}
	resultType, err := declaredResult(form.args[index].form)
	if err != nil {
		return err
	}
	index++
	// Collect optional (local $name type) declarations.
	locals := make(map[string]string)
	for index < len(form.args) && form.args[index].form != nil && form.args[index].form.head == "local" {
		name, localType, lerr := validateLocalDecl(form.args[index].form)
		if lerr != nil {
			return lerr
		}
		locals[name] = localType
		index++
	}
	if index >= len(form.args) {
		return fmt.Errorf("function requires at least one body expression at byte %d", form.position)
	}
	// Body may be one expression or a sequence (all-but-last void/bottom, last matches result).
	body := form.args[index:]
	for bodyIdx, value := range body {
		if value.form == nil {
			return fmt.Errorf("function body contains unexpected atom %q", value.atom)
		}
		actual, verr := validateExpression(value.form, resultType, hasMemory, locals)
		if verr != nil {
			return verr
		}
		last := bodyIdx == len(body)-1
		if last {
			if !watTypeMatches(resultType, actual) {
				return fmt.Errorf("function result has type %s, want %s", actual, resultType)
			}
		} else {
			if actual != watVoid && actual != watBottom {
				return fmt.Errorf("function body expression %d leaves unused %s value", bodyIdx+1, actual)
			}
		}
	}
	return nil
}

func validateExpression(form *watForm, functionResult string, hasMemory bool, locals map[string]string) (string, error) {
	if strings.HasSuffix(form.head, ".const") {
		valueType := strings.TrimSuffix(form.head, ".const")
		if !isWasmValueType(valueType) || len(form.args) != 1 || form.args[0].form != nil || form.args[0].quoted {
			return "", fmt.Errorf("invalid %q instruction at byte %d", form.head, form.position)
		}
		if err := validateConstant(valueType, form.args[0].atom); err != nil {
			return "", fmt.Errorf("invalid %s constant %q at byte %d: %w", valueType, form.args[0].atom, form.position, err)
		}
		return valueType, nil
	}

	binaryOps := map[string][3]string{
		"i32.add": {"i32", "i32", "i32"}, "i32.mul": {"i32", "i32", "i32"},
		"i32.and": {"i32", "i32", "i32"}, "i32.or": {"i32", "i32", "i32"},
		"i32.eq": {"i32", "i32", "i32"}, "i32.ne": {"i32", "i32", "i32"},
		"i64.add": {"i64", "i64", "i64"}, "i64.sub": {"i64", "i64", "i64"},
		"i64.mul": {"i64", "i64", "i64"}, "i64.div_s": {"i64", "i64", "i64"},
		"i64.lt_s": {"i64", "i64", "i32"}, "i64.gt_s": {"i64", "i64", "i32"},
		"i64.le_s": {"i64", "i64", "i32"}, "i64.ge_s": {"i64", "i64", "i32"},
		"i64.lt_u": {"i64", "i64", "i32"}, "i64.eq": {"i64", "i64", "i32"},
		"i64.ne":  {"i64", "i64", "i32"},
		"f64.add": {"f64", "f64", "f64"}, "f64.sub": {"f64", "f64", "f64"},
		"f64.mul": {"f64", "f64", "f64"}, "f64.div": {"f64", "f64", "f64"},
		"f64.lt": {"f64", "f64", "i32"}, "f64.gt": {"f64", "f64", "i32"},
		"f64.le": {"f64", "f64", "i32"}, "f64.ge": {"f64", "f64", "i32"},
		"f64.eq": {"f64", "f64", "i32"}, "f64.ne": {"f64", "f64", "i32"},
	}
	if signature, ok := binaryOps[form.head]; ok {
		if err := requireExpressionArgs(form, 2); err != nil {
			return "", err
		}
		for index, expected := range signature[:2] {
			actual, err := validateExpression(form.args[index].form, functionResult, hasMemory, locals)
			if err != nil {
				return "", err
			}
			if !watTypeMatches(expected, actual) {
				return "", fmt.Errorf("%s operand %d has type %s, want %s", form.head, index+1, actual, expected)
			}
		}
		return signature[2], nil
	}

	switch form.head {
	case "f64.convert_i64_s":
		return validateUnary(form, "i64", "f64", functionResult, hasMemory, locals)
	case "i64.trunc_f64_s":
		return validateUnary(form, "f64", "i64", functionResult, hasMemory, locals)
	case "i32.wrap_i64":
		return validateUnary(form, "i64", "i32", functionResult, hasMemory, locals)
	case "i32.eqz":
		return validateUnary(form, "i32", "i32", functionResult, hasMemory, locals)
	case "i32.load", "i64.load":
		if !hasMemory {
			return "", fmt.Errorf("%s requires declared memory at byte %d", form.head, form.position)
		}
		resultType := strings.TrimSuffix(form.head, ".load")
		return validateUnary(form, "i32", resultType, functionResult, hasMemory, locals)
	case "i64.store":
		if !hasMemory {
			return "", fmt.Errorf("%s requires declared memory at byte %d", form.head, form.position)
		}
		if err := requireExpressionArgs(form, 2); err != nil {
			return "", err
		}
		for index, expected := range []string{"i32", "i64"} {
			actual, err := validateExpression(form.args[index].form, functionResult, hasMemory, locals)
			if err != nil {
				return "", err
			}
			if !watTypeMatches(expected, actual) {
				return "", fmt.Errorf("%s operand %d has type %s, want %s", form.head, index+1, actual, expected)
			}
		}
		return watVoid, nil
	case "i32.store":
		if !hasMemory {
			return "", fmt.Errorf("%s requires declared memory at byte %d", form.head, form.position)
		}
		if err := requireExpressionArgs(form, 2); err != nil {
			return "", err
		}
		for index, expected := range []string{"i32", "i32"} {
			actual, err := validateExpression(form.args[index].form, functionResult, hasMemory, locals)
			if err != nil {
				return "", err
			}
			if !watTypeMatches(expected, actual) {
				return "", fmt.Errorf("%s operand %d has type %s, want %s", form.head, index+1, actual, expected)
			}
		}
		return watVoid, nil
	case "drop":
		if err := requireExpressionArgs(form, 1); err != nil {
			return "", err
		}
		actual, err := validateExpression(form.args[0].form, functionResult, hasMemory, locals)
		if err != nil {
			return "", err
		}
		if actual == watVoid {
			return "", fmt.Errorf("drop operand has no value at byte %d", form.position)
		}
		return watVoid, nil
	case "return":
		if err := requireExpressionArgs(form, 1); err != nil {
			return "", err
		}
		actual, err := validateExpression(form.args[0].form, functionResult, hasMemory, locals)
		if err != nil {
			return "", err
		}
		if !watTypeMatches(functionResult, actual) {
			return "", fmt.Errorf("return has type %s, want %s", actual, functionResult)
		}
		return watBottom, nil
	case "if":
		return validateIf(form, functionResult, hasMemory, locals)
	case "block":
		return validateBlock(form, functionResult, hasMemory, locals)
	case "loop":
		return validateLoop(form, functionResult, hasMemory, locals)
	case "local.get":
		return validateLocalGet(form, locals)
	case "local.set":
		return validateLocalSet(form, functionResult, hasMemory, locals)
	case "br":
		return validateBr(form)
	case "br_if":
		return validateBrIf(form, functionResult, hasMemory, locals)
	default:
		return "", fmt.Errorf("unsupported instruction %q at byte %d", form.head, form.position)
	}
}

func validateIf(form *watForm, functionResult string, hasMemory bool, locals map[string]string) (string, error) {
	if len(form.args) != 4 || form.args[0].form == nil || form.args[0].form.head != "result" ||
		form.args[1].form == nil || form.args[2].form == nil || form.args[2].form.head != "then" ||
		form.args[3].form == nil || form.args[3].form.head != "else" {
		return "", fmt.Errorf("if requires result, condition, then, and else forms at byte %d", form.position)
	}
	resultType, err := declaredResult(form.args[0].form)
	if err != nil {
		return "", err
	}
	conditionType, err := validateExpression(form.args[1].form, functionResult, hasMemory, locals)
	if err != nil {
		return "", err
	}
	if conditionType != "i32" {
		return "", fmt.Errorf("if condition has type %s, want i32", conditionType)
	}
	for _, branch := range form.args[2:] {
		if len(branch.form.args) != 1 || branch.form.args[0].form == nil {
			return "", fmt.Errorf("%s requires one expression at byte %d", branch.form.head, branch.form.position)
		}
		actual, err := validateExpression(branch.form.args[0].form, functionResult, hasMemory, locals)
		if err != nil {
			return "", err
		}
		if !watTypeMatches(resultType, actual) {
			return "", fmt.Errorf("%s branch has type %s, want %s", branch.form.head, actual, resultType)
		}
	}
	return resultType, nil
}

func validateBlock(form *watForm, functionResult string, hasMemory bool, locals map[string]string) (string, error) {
	// block may optionally have a leading label atom: (block $label (result T) stmts...)
	// or no label: (block (result T) stmts...)
	// or a void block (no result): (block $label stmts...) or (block stmts...)
	start := 0
	if len(form.args) > 0 && form.args[0].form == nil && !form.args[0].quoted &&
		strings.HasPrefix(form.args[0].atom, "$") {
		start = 1
	}
	// If the first non-label arg is a (result T) form, this is a valued block.
	if start < len(form.args) && form.args[start].form != nil && form.args[start].form.head == "result" {
		resultType, err := declaredResult(form.args[start].form)
		if err != nil {
			return "", err
		}
		body := form.args[start+1:]
		if len(body) == 0 {
			return "", fmt.Errorf("block requires at least one expression at byte %d", form.position)
		}
		for index, value := range body {
			if value.form == nil {
				return "", fmt.Errorf("block contains unexpected atom %q", value.atom)
			}
			actual, err := validateExpression(value.form, functionResult, hasMemory, locals)
			if err != nil {
				return "", err
			}
			last := index == len(body)-1
			if !last && actual != watVoid && actual != watBottom {
				return "", fmt.Errorf("block expression %d leaves unused %s value", index+1, actual)
			}
			if last && !watTypeMatches(resultType, actual) {
				return "", fmt.Errorf("block result has type %s, want %s", actual, resultType)
			}
		}
		return resultType, nil
	}
	// Void block: all statements must be void or bottom.
	body := form.args[start:]
	for index, value := range body {
		if value.form == nil {
			return "", fmt.Errorf("block contains unexpected atom %q", value.atom)
		}
		actual, err := validateExpression(value.form, functionResult, hasMemory, locals)
		if err != nil {
			return "", err
		}
		if actual != watVoid && actual != watBottom {
			return "", fmt.Errorf("void block expression %d leaves unused %s value", index+1, actual)
		}
	}
	return watVoid, nil
}

// validateLoop validates a (loop $label stmts...) form.
// Loops in Wasm structured control flow have void result type.
func validateLoop(form *watForm, functionResult string, hasMemory bool, locals map[string]string) (string, error) {
	// loop may optionally have a leading label atom: (loop $label stmts...)
	start := 0
	if len(form.args) > 0 && form.args[0].form == nil && !form.args[0].quoted &&
		strings.HasPrefix(form.args[0].atom, "$") {
		start = 1
	}
	for index, value := range form.args[start:] {
		if value.form == nil {
			return "", fmt.Errorf("loop contains unexpected atom %q at position %d", value.atom, index)
		}
		actual, err := validateExpression(value.form, functionResult, hasMemory, locals)
		if err != nil {
			return "", err
		}
		if actual != watVoid && actual != watBottom {
			return "", fmt.Errorf("loop statement %d leaves unused %s value", index+1, actual)
		}
	}
	return watVoid, nil
}

// validateLocalGet validates (local.get $name) and returns the local's type.
func validateLocalGet(form *watForm, locals map[string]string) (string, error) {
	if len(form.args) != 1 || form.args[0].form != nil || form.args[0].quoted {
		return "", fmt.Errorf("local.get requires one label argument at byte %d", form.position)
	}
	name := form.args[0].atom
	if localType, ok := locals[name]; ok {
		return localType, nil
	}
	return "", fmt.Errorf("local.get references undeclared local %q at byte %d", name, form.position)
}

// validateLocalSet validates (local.set $name expr) and returns void.
func validateLocalSet(form *watForm, functionResult string, hasMemory bool, locals map[string]string) (string, error) {
	if len(form.args) != 2 || form.args[0].form != nil || form.args[0].quoted || form.args[1].form == nil {
		return "", fmt.Errorf("local.set requires a label and expression at byte %d", form.position)
	}
	name := form.args[0].atom
	localType, ok := locals[name]
	if !ok {
		return "", fmt.Errorf("local.set references undeclared local %q at byte %d", name, form.position)
	}
	actual, err := validateExpression(form.args[1].form, functionResult, hasMemory, locals)
	if err != nil {
		return "", err
	}
	if !watTypeMatches(localType, actual) {
		return "", fmt.Errorf("local.set %q has type %s, want %s", name, actual, localType)
	}
	return watVoid, nil
}

// validateBr validates (br $label) — unconditional branch, returns bottom.
func validateBr(form *watForm) (string, error) {
	if len(form.args) != 1 || form.args[0].form != nil || form.args[0].quoted {
		return "", fmt.Errorf("br requires one label argument at byte %d", form.position)
	}
	return watBottom, nil
}

// validateBrIf validates (br_if $label cond) — conditional branch, returns void.
func validateBrIf(form *watForm, functionResult string, hasMemory bool, locals map[string]string) (string, error) {
	if len(form.args) != 2 || form.args[0].form != nil || form.args[0].quoted || form.args[1].form == nil {
		return "", fmt.Errorf("br_if requires a label and condition expression at byte %d", form.position)
	}
	condType, err := validateExpression(form.args[1].form, functionResult, hasMemory, locals)
	if err != nil {
		return "", err
	}
	if condType != "i32" {
		return "", fmt.Errorf("br_if condition has type %s, want i32 at byte %d", condType, form.position)
	}
	return watVoid, nil
}

// validateLocalDecl validates (local $name type) and returns the name and type.
func validateLocalDecl(form *watForm) (string, string, error) {
	if len(form.args) != 2 || form.args[0].form != nil || form.args[0].quoted ||
		form.args[1].form != nil || form.args[1].quoted {
		return "", "", fmt.Errorf("local requires a name and type at byte %d", form.position)
	}
	name := form.args[0].atom
	if !strings.HasPrefix(name, "$") {
		return "", "", fmt.Errorf("local name must start with $ at byte %d", form.position)
	}
	localType := form.args[1].atom
	if !isWasmValueType(localType) {
		return "", "", fmt.Errorf("local type %q is not a valid wasm type at byte %d", localType, form.position)
	}
	return name, localType, nil
}

func validateUnary(form *watForm, operandType, resultType, functionResult string, hasMemory bool, locals map[string]string) (string, error) {
	if err := requireExpressionArgs(form, 1); err != nil {
		return "", err
	}
	actual, err := validateExpression(form.args[0].form, functionResult, hasMemory, locals)
	if err != nil {
		return "", err
	}
	if !watTypeMatches(operandType, actual) {
		return "", fmt.Errorf("%s operand has type %s, want %s", form.head, actual, operandType)
	}
	return resultType, nil
}

func requireExpressionArgs(form *watForm, count int) error {
	if len(form.args) != count {
		return fmt.Errorf("%s expects %d operands at byte %d", form.head, count, form.position)
	}
	for _, value := range form.args {
		if value.form == nil {
			return fmt.Errorf("%s requires folded expression operands at byte %d", form.head, form.position)
		}
	}
	return nil
}

func declaredResult(form *watForm) (string, error) {
	if len(form.args) != 1 || form.args[0].form != nil || form.args[0].quoted || !isWasmValueType(form.args[0].atom) {
		return "", fmt.Errorf("invalid result declaration at byte %d", form.position)
	}
	return form.args[0].atom, nil
}

func validateExport(form *watForm) error {
	if len(form.args) != 1 || !form.args[0].quoted {
		return fmt.Errorf("export requires one quoted name at byte %d", form.position)
	}
	return nil
}

func constantI32(form *watForm) (uint32, error) {
	if form.head != "i32.const" || len(form.args) != 1 || form.args[0].form != nil || form.args[0].quoted {
		return 0, fmt.Errorf("expected i32.const")
	}
	value, err := strconv.ParseUint(form.args[0].atom, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid i32 constant %q", form.args[0].atom)
	}
	return uint32(value), nil
}

func validateConstant(valueType, value string) error {
	switch valueType {
	case "i32":
		if _, err := strconv.ParseInt(value, 10, 32); err == nil {
			return nil
		}
		_, err := strconv.ParseUint(value, 10, 32)
		return err
	case "i64":
		if _, err := strconv.ParseInt(value, 10, 64); err == nil {
			return nil
		}
		_, err := strconv.ParseUint(value, 10, 64)
		return err
	case "f64":
		_, err := strconv.ParseFloat(value, 64)
		return err
	default:
		return fmt.Errorf("unsupported value type %q", valueType)
	}
}

func watStringLength(value string) (int, error) {
	length := 0
	for index := 0; index < len(value); {
		if value[index] != '\\' {
			length++
			index++
			continue
		}
		if index+2 < len(value) {
			if _, err := strconv.ParseUint(value[index+1:index+3], 16, 8); err == nil {
				length++
				index += 3
				continue
			}
		}
		if index+1 >= len(value) {
			return 0, fmt.Errorf("trailing escape")
		}
		length++
		index += 2
	}
	return length, nil
}

func isWasmValueType(value string) bool {
	return value == "i32" || value == "i64" || value == "f64"
}

func watTypeMatches(expected, actual string) bool {
	return actual == watBottom || expected == actual
}
