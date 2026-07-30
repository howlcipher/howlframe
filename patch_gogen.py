import re

with open("internal/backend/gogen/gogen.go", "r") as f:
    text = f.read()

# Replace in EmitGoIR
emit_go_ir_cases = """
	case "let":
		var letPrefix strings.Builder
		letPrefix.WriteString("		{\n")
		declaredVars := make(map[string]bool)

		curr := &ast.Node{Type: "List", Children: []*ast.Node{{Type: "SYMBOL", Value: "let"}, ir.Kids[0], ir.Kids[1]}}
		for curr.Type == "List" && len(curr.Children) == 3 && curr.Children[0].Value == "let" {
			binds := curr.Children[1]
			varName := binds.Children[0].Value
			valNode := binds.Children[1]
			var valStr string
			if valNode.Type == "STRING" {
				valStr = fmt.Sprintf("%q", valNode.Value)
			} else if valNode.Type == "List" && len(valNode.Children) > 0 {
				funcName := valNode.Children[0].Value
				if funcName == "call" {
					var args []string
					for j := 2; j < len(valNode.Children); j++ {
						args = append(args, generateExpression(valNode.Children[j], reqVar, depth+1))
					}
					valStr = fmt.Sprintf("%s(%s)", valNode.Children[1].Value, strings.Join(args, ", "))
				} else if funcName == "list" {
					var items []string
					for j := 1; j < len(valNode.Children); j++ {
						if valNode.Children[j].Type == "STRING" {
							items = append(items, fmt.Sprintf("%q", valNode.Children[j].Value))
						} else {
							items = append(items, generateExpression(valNode.Children[j], reqVar, depth+1))
						}
					}
					valStr = fmt.Sprintf("[]string{%s}", strings.Join(items, ", "))
				} else if funcName == "dict" {
					var pairs []string
					for j := 1; j < len(valNode.Children); j++ {
						pair := valNode.Children[j]
						if pair.Type == "List" && len(pair.Children) == 2 {
							k := pair.Children[0].Value
							if pair.Children[0].Type == "STRING" {
								k = fmt.Sprintf("%q", k)
							} else {
								k = generateExpression(pair.Children[0], reqVar, depth+1)
							}
							v := pair.Children[1].Value
							if pair.Children[1].Type == "STRING" {
								v = fmt.Sprintf("%q", v)
							} else {
								v = generateExpression(pair.Children[1], reqVar, depth+1)
							}
							pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
						}
					}
					valStr = fmt.Sprintf("map[string]string{%s}", strings.Join(pairs, ", "))
				} else if funcName == "env" {
					keyNode := valNode.Children[1]
					if keyNode.Type == "STRING" {
						valStr = fmt.Sprintf("os.Getenv(%q)", keyNode.Value)
					}
				} else if funcName == "parse_json" {
					// Handled downstream
				} else {
					valStr = generateStatement(valNode, reqVar, depth+1)
				}
			} else {
				valStr = generateStatement(valNode, reqVar, depth+1)
			}

			if valNode.Type == "List" && len(valNode.Children) > 0 && valNode.Children[0].Value == "parse_json" {
				targetType := valNode.Children[1].Value
				bodyVar := valNode.Children[2].Value
				if bodyVar == "req.body" {
					bodyVar = reqVar + ".Body"
				}
				if bodyVar == reqVar+".Body" {
					letPrefix.WriteString(fmt.Sprintf("			var %s %s\\n			_ = json.NewDecoder(%s).Decode(&%s)\\n			_ = %s\\n", varName, targetType, bodyVar, varName, varName))
				} else {
					letPrefix.WriteString(fmt.Sprintf("			var %s %s\\n			_ = json.Unmarshal([]byte(%s), &%s)\\n			_ = %s\\n", varName, targetType, bodyVar, varName, varName))
				}
				declaredVars[varName] = true
			} else {
				if declaredVars[varName] {
					letPrefix.WriteString(fmt.Sprintf("			%s = %s\\n			_ = %s\\n", varName, valStr, varName))
				} else {
					letPrefix.WriteString(fmt.Sprintf("			%s := %s\\n			_ = %s\\n", varName, valStr, varName))
					declaredVars[varName] = true
				}
			}

			curr = curr.Children[2]
		}

		bodyCode := generateStatement(curr, reqVar, depth+1)
		return fmt.Sprintf("%s%s\\n		}", letPrefix.String(), bodyCode)
	case "try_let":
		binds := ir.Kids[0]
		varName := binds.Children[0].Value
		valNode := binds.Children[1]

		catchNode := ir.Kids[1]
		errVar := catchNode.Children[1].Value
		catchBodyCode := generateStatement(catchNode.Children[2], reqVar, depth+1)
		successBodyCode := generateStatement(ir.Kids[2], reqVar, depth+1)

		if valNode.Type == "List" && len(valNode.Children) > 0 && valNode.Children[0].Value == "parse_json" {
			targetType := valNode.Children[1].Value
			bodyVar := valNode.Children[2].Value
			if bodyVar == "req.body" {
				bodyVar = reqVar + ".Body"
				return fmt.Sprintf(` + "`" + `		{
			var %s %s
			if %s := json.NewDecoder(%s).Decode(&%s); %s != nil {
%s
			} else {
				_ = %s
%s
			}
		}` + "`" + `, varName, targetType, errVar, bodyVar, varName, errVar, catchBodyCode, varName, successBodyCode)
			} else {
				return fmt.Sprintf(` + "`" + `		{
			var %s %s
			if %s := json.Unmarshal([]byte(%s), &%s); %s != nil {
%s
			} else {
				_ = %s
%s
			}
		}` + "`" + `, varName, targetType, errVar, bodyVar, varName, errVar, catchBodyCode, varName, successBodyCode)
			}
		}

		valStr := generateStatement(valNode, reqVar, depth+1)
		return fmt.Sprintf(` + "`" + `		{
			%s, %s := %s
			if %s != nil {
%s
			} else {
				_ = %s
%s
			}
		}` + "`" + `, varName, errVar, valStr, errVar, catchBodyCode, varName, successBodyCode)
	case "spawn":
		lambdaNode := ir.Kids[0]
		bodyCode := generateStatement(lambdaNode.Children[2], reqVar, depth+1)
		traceInject := fmt.Sprintf("\\t\\tdefer observer.Trace(%q, map[string]any{})()\\n", "spawn_lambda")
		return fmt.Sprintf("		go func() {\\n%s%s\\n		}()", traceInject, bodyCode)
	case "call":
		funcName := ir.Kids[0].Value
		var args []string
		for j := 1; j < len(ir.Kids); j++ {
			argNode := ir.Kids[j]
			if argNode.Type == "STRING" {
				args = append(args, fmt.Sprintf("%q", argNode.Value))
			} else if argNode.Type == "NUMBER" || argNode.Type == "SYMBOL" {
				args = append(args, argNode.Value)
			} else {
				args = append(args, generateExpression(argNode, reqVar, depth+1))
			}
		}
		return fmt.Sprintf("		%s(%s)", funcName, strings.Join(args, ", "))
	case "for":
		itemNode := ir.Kids[0].Value
		listNode := ir.Kids[1].Value
		bodyCode := generateStatement(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf(` + "`" + `		for _, %s := range %s {
			_ = %s
%s
		}` + "`" + `, itemNode, listNode, itemNode, bodyCode)
"""

text = text.replace('\tcase "return":\n', emit_go_ir_cases + '\tcase "return":\n')

# Now remove from generateStatementRaw
let_idx = text.find('} else if head == "let" {')
tl_idx = text.find('} else if head == "try_let" {')
sp_idx = text.find('} else if head == "spawn" {')
sa_idx = text.find('} else if head == "spawn_agent" {')
for_idx = text.find('} else if head == "for" {')
rf_idx = text.find('} else if head == "read_file" {')
call_idx = text.find('} else if head == "call" {')
ca_idx = text.find('} else if head == "cli_args" {')

# Cut out let and try_let
if let_idx != -1 and sp_idx != -1:
    text = text[:let_idx] + text[sp_idx:]

# the sp_idx has changed after the first cut, so let's recalculate
sp_idx = text.find('} else if head == "spawn" {')
sa_idx = text.find('} else if head == "spawn_agent" {')
if sp_idx != -1 and sa_idx != -1:
    text = text[:sp_idx] + text[sa_idx:]

for_idx = text.find('} else if head == "for" {')
rf_idx = text.find('} else if head == "read_file" {')
if for_idx != -1 and rf_idx != -1:
    text = text[:for_idx] + text[rf_idx:]

call_idx = text.find('} else if head == "call" {')
ca_idx = text.find('} else if head == "cli_args" {')
if call_idx != -1 and ca_idx != -1:
    text = text[:call_idx] + text[ca_idx:]

with open("internal/backend/gogen/gogen.go", "w") as f:
    f.write(text)

