package javascript

import (
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/ir"
	"strings"
)

func BinOpJSToken(head string) string {
	switch head {
	case "and":
		return "&&"
	case "or":
		return "||"
	case "=", "==":
		return "==="
	case "!=":
		return "!=="
	default:
		return head
	}
}

func flattenModulesJS(nodes []*ast.Node) []*ast.Node {
	var result []*ast.Node
	for _, node := range nodes {
		if node.Type == "List" && len(node.Children) > 0 {
			head := node.Children[0].Value
			if head == "module" {
				result = append(result, flattenModulesJS(node.Children[2:])...)
				continue
			}
			if head == "export" && len(node.Children) == 2 {
				result = append(result, flattenModulesJS([]*ast.Node{node.Children[1]})...)
				continue
			}
		}
		result = append(result, node)
	}
	return result
}

func sanitizeJSName(name string) string {
	if !strings.Contains(name, "/") {
		return name
	}
	parts := strings.Split(name, "/")
	res := parts[0]
	for _, p := range parts[1:] {
		if len(p) > 0 {
			res += "_" + p
		}
	}
	return res
}

func EmitJSIR(ir *ir.IRNode, reqVar string, depth int) string {
	switch ir.Kind {
	case "binop":
		arg1 := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		arg2 := generateJSExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s %s %s)", arg1, BinOpJSToken(ir.Op), arg2)
	case "let":
		var letPrefix strings.Builder
		letPrefix.WriteString("{\n")
		declaredVars := make(map[string]bool)

		bindings, curr := ast.LetChain(&ast.Node{Type: "List", Children: []*ast.Node{{Type: "SYMBOL", Value: "let"}, ir.Kids[0], ir.Kids[1]}})
		for _, binds := range bindings {
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
						if valNode.Children[j].Type == "STRING" {
							args = append(args, fmt.Sprintf("%q", valNode.Children[j].Value))
						} else {
							args = append(args, generateJSExpression(valNode.Children[j], reqVar, depth+1))
						}
					}
					valStr = fmt.Sprintf("(await %s(%s))", sanitizeJSName(valNode.Children[1].Value), strings.Join(args, ", "))
				} else if funcName == "list" {
					var items []string
					for j := 1; j < len(valNode.Children); j++ {
						if valNode.Children[j].Type == "STRING" {
							items = append(items, fmt.Sprintf("%q", valNode.Children[j].Value))
						} else {
							items = append(items, generateJSExpression(valNode.Children[j], reqVar, depth+1))
						}
					}
					valStr = fmt.Sprintf("[%s]", strings.Join(items, ", "))
				} else if funcName == "dict" {
					var pairs []string
					for j := 1; j < len(valNode.Children); j++ {
						pair := valNode.Children[j]
						if pair.Type == "List" && len(pair.Children) == 2 {
							k := pair.Children[0].Value
							if pair.Children[0].Type == "STRING" {
								k = fmt.Sprintf("%q", k)
							}
							v := pair.Children[1].Value
							if pair.Children[1].Type == "STRING" {
								v = fmt.Sprintf("%q", v)
							} else {
								v = generateJSExpression(pair.Children[1], reqVar, depth+1)
							}
							pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
						}
					}
					valStr = fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
				} else if funcName == "parse_json" {
					bodyVar := valNode.Children[2].Value
					valStr = fmt.Sprintf("JSON.parse(%s)", bodyVar)
				} else {
					valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
				}
			} else {
				valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
			}

			if declaredVars[varName] {
				letPrefix.WriteString(fmt.Sprintf("%s = %s;\n", varName, valStr))
			} else {
				letPrefix.WriteString(fmt.Sprintf("let %s = %s;\n", varName, valStr))
				declaredVars[varName] = true
			}
		}
		bodyCode := generateJSStatement(curr, reqVar, depth+1)
		return fmt.Sprintf("%s%s\n}", letPrefix.String(), bodyCode)
	case "try_let":
		binds := ir.Kids[0]
		varName := binds.Children[0].Value
		valNode := binds.Children[1]

		var valStr string
		if valNode.Type == "List" && len(valNode.Children) > 0 && valNode.Children[0].Value == "parse_json" {
			bodyVar := generateJSStatementRaw(valNode.Children[2], reqVar, depth+1)
			valStr = fmt.Sprintf("JSON.parse(%s)", bodyVar)
		} else {
			valStr = generateJSStatementRaw(valNode, reqVar, depth+1)
		}

		catchNode := ir.Kids[1]
		errVar := catchNode.Children[1].Value
		catchBodyCode := generateJSStatement(catchNode.Children[2], reqVar, depth+1)
		successBodyCode := generateJSStatement(ir.Kids[2], reqVar, depth+1)

		return fmt.Sprintf("{\n\tlet %s;\n\tlet %s = null;\n\ttry {\n\t\t%s = %s;\n\t} catch (e) {\n\t\t%s = e;\n\t}\n\tif (%s !== null) {\n\t\t%s\n\t} else {\n\t\t%s\n\t}\n}", varName, errVar, varName, valStr, errVar, errVar, catchBodyCode, successBodyCode)
	case "for":
		itemNode := ir.Kids[0].Value
		listNode := ir.Kids[1].Value
		bodyCode := generateJSStatement(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("for (let %s of %s) {\n%s\n}", itemNode, listNode, bodyCode)
	case "call":
		funcName := sanitizeJSName(ir.Kids[0].Value)
		var args []string
		for j := 1; j < len(ir.Kids); j++ {
			args = append(args, generateJSExpression(ir.Kids[j], reqVar, depth+1))
		}
		return fmt.Sprintf("(await %s(%s))", funcName, strings.Join(args, ", "))
	case "spawn":
		lambdaNode := ir.Kids[0]
		bodyCode := generateJSStatement(lambdaNode.Children[2], reqVar, depth+1)
		return fmt.Sprintf(";(async () => {\n%s\n})();", bodyCode)
	case "return":
		return fmt.Sprintf("return %s;", generateJSStatementRaw(ir.Kids[0], reqVar, depth+1))
	case "if":
		condExpr := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		thenCode := generateJSStatement(ir.Kids[1], reqVar, depth+1)
		if len(ir.Kids) == 2 {
			return fmt.Sprintf("if (%s) {\n%s\n}", condExpr, thenCode)
		}
		elseCode := generateJSStatement(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("if (%s) {\n%s\n} else {\n%s\n}", condExpr, thenCode, elseCode)
	case "while":
		condExpr := generateJSExpression(ir.Kids[0], reqVar, depth+1)
		bodyCode := generateJSStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("while (%s) {\n%s\n}", condExpr, bodyCode)
	case "do":
		var stmts string
		for _, kid := range ir.Kids {
			stmts += generateJSStatement(kid, reqVar, depth+1) + ";\n"
		}
		return fmt.Sprintf("{\n%s}", stmts)
	case "set":
		varStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		valStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("%s = %s", varStr, valStr)
	case "match":
		varStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		var casesStr string
		for _, c := range ir.Cases {
			caseValStr := c.Label.Value
			if c.Label.Type == "STRING" {
				caseValStr = fmt.Sprintf("%q", caseValStr)
			}
			caseBodyCode := generateJSStatement(c.Body, reqVar, depth+1)
			if c.IsDefault {
				casesStr += fmt.Sprintf("default:\n%s;\nbreak;\n", caseBodyCode)
			} else {
				casesStr += fmt.Sprintf("case %s:\n%s;\nbreak;\n", caseValStr, caseBodyCode)
			}
		}
		return fmt.Sprintf("switch (%s) {\n%s}", varStr, casesStr)
	case "sleep":
		msStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("(await new Promise(r => setTimeout(r, %s)))", msStr)
	case "to_int":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("parseInt(%s, 10)", valStr)
	case "to_float":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("parseFloat(%s)", valStr)
	case "to_string", "bytes_to_string":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("String(%s)", valStr)
	case "str_split":
		sStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		sepStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s).split(%s)", sStr, sepStr)
	case "str_join":
		listStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		sepStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s).join(%s)", listStr, sepStr)
	case "regex_match":
		patStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		sStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("new RegExp(%s).test(%s)", patStr, sStr)
	case "append":
		listNode := ir.Kids[0]
		itemStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("%s.push(%s)", listNode.Value, itemStr)
	case "map_set":
		dictNode := ir.Kids[0]
		keyStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		valStr := generateJSStatementRaw(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("%s[%s] = %s", dictNode.Value, keyStr, valStr)
	case "map_delete":
		dictNode := ir.Kids[0]
		keyStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("delete %s[%s]", dictNode.Value, keyStr)
	case "map_get":
		dictNode := ir.Kids[0]
		keyStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s[%s] ?? \"\")", dictNode.Value, keyStr)
	case "list_get":
		listNode := ir.Kids[0]
		idxStr := generateJSStatementRaw(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s[%s] ?? \"\")", listNode.Value, idxStr)
	case "list_len":
		listStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("(%s).length", listStr)
	case "is_nil":
		valStr := generateJSStatementRaw(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("(%s === null || %s === undefined)", valStr, valStr)
	case "list":
		var items []string
		for _, kid := range ir.Kids {
			if kid.Type == "STRING" {
				items = append(items, fmt.Sprintf("%q", kid.Value))
			} else {
				items = append(items, generateJSExpression(kid, reqVar, depth+1))
			}
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	case "dict":
		var pairs []string
		for _, kid := range ir.Kids {
			if kid.Type != "List" || len(kid.Children) != 2 {
				// ast.ReportError("dict expects (k v) pairs", kid.Line, kid.Column)
			}
			k := kid.Children[0].Value
			if kid.Children[0].Type == "STRING" {
				k = fmt.Sprintf("%q", k)
			}
			v := kid.Children[1].Value
			if kid.Children[1].Type == "STRING" {
				v = fmt.Sprintf("%q", v)
			} else {
				v = generateJSExpression(kid.Children[1], reqVar, depth+1)
			}
			pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
		}
		return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
	case "print":
		var args []string
		for _, kid := range ir.Kids {
			args = append(args, generateJSStatementRaw(kid, reqVar, depth+1))
		}
		return fmt.Sprintf("console.log(%s)", strings.Join(args, ", "))
	}
	// ast.ReportError(fmt.Sprintf("Unknown IR kind for JS: %s", ir.Kind), 0, 0)
	return ""
}

func GenerateJSCode(node *ast.Node) (string, string) {
	if node.Type != "List" || len(node.Children) == 0 {
		// ast.ReportError("Expected list at root", node.Line, node.Column)
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" || head.Value != "web_app" {
		// ast.ReportError("Expected web_app as root symbol", head.Line, head.Column)
	}

	var funcsCode string
	var appCode string
	var testCode string

	handlers := flattenModulesJS(node.Children[1:])
	for i := 0; i < len(handlers); i++ {
		handlerNode := handlers[i]
		if handlerNode.Type != "List" || len(handlerNode.Children) == 0 {
			appCode += generateJSStatement(handlerNode, "", 0) + "\n"
			continue
		}

		headVal := handlerNode.Children[0].Value

		if headVal == "intent" {
			continue
		}

		if headVal == "test" {
			if len(handlerNode.Children) < 3 {
				// ast.ReportError(`test expects (test "description" body...)`, handlerNode.Line, handlerNode.Column)
			}
			descNode := handlerNode.Children[1]
			if descNode.Type != "STRING" {
				// ast.ReportError("test description must be a string", descNode.Line, descNode.Column)
			}
			desc := descNode.Value
			var testBodyCode string
			for j := 2; j < len(handlerNode.Children); j++ {
				testBodyCode += generateJSStatement(handlerNode.Children[j], "", 0) + "\n"
			}
			testCode += fmt.Sprintf("test(%q, async (t) => {\n%s\n});\n\n", desc, testBodyCode)
			continue
		}

		if headVal == "defun" {
			if len(handlerNode.Children) < 4 {
				// ast.ReportError("defun expects (defun name (args) body)", handlerNode.Line, handlerNode.Column)
			}
			name := sanitizeJSName(handlerNode.Children[1].Value)
			argsNode := handlerNode.Children[2]

			var argsList []string
			for _, arg := range argsNode.Children {
				argsList = append(argsList, arg.Value)
			}
			argsStr := strings.Join(argsList, ", ")

			bodyNode := handlerNode.Children[len(handlerNode.Children)-1]
			bodyCode := generateJSStatement(bodyNode, "", 0)
			funcsCode += fmt.Sprintf("async function %s(%s) {\n%s\n}\n\n", name, argsStr, bodyCode)
			continue
		}

		appCode += generateJSStatement(handlerNode, "", 0) + "\n"
	}

	code := funcsCode + appCode

	if testCode != "" {
		testCode = "const test = require('node:test');\n" +
			"const assert = require('node:assert');\n\n" +
			funcsCode + testCode
	}

	return code, testCode
}

func generateJSStatement(node *ast.Node, reqVar string, depth int) string {
	code := generateJSStatementRaw(node, reqVar, depth)
	if node.Type != "List" || len(node.Children) == 0 {
		return code
	}
	head := node.Children[0].Value
	switch head {
	case "return", "let", "do", "try_let", "spawn", "spawn_agent", "task", "if", "print", "for", "sleep", "while", "match", "set", "call":
		if node.Filename != "" {
			return fmt.Sprintf("//line %s:%d\n%s", node.Filename, node.Line, code)
		}
	}
	return code
}

func generateJSExpression(node *ast.Node, reqVar string, depth int) string {
	return generateJSStatementRaw(node, reqVar, depth)
}

func generateJSStatementRaw(node *ast.Node, reqVar string, depth int) string {
	if depth > 1000 {
		// ast.ReportError("AST too deep", node.Line, node.Column)
	}
	if node.Type == "STRING" {
		return fmt.Sprintf("%q", node.Value)
	}
	if node.Type == "SYMBOL" || node.Type == "INT" || node.Type == "FLOAT" {
		return node.Value
	}
	if node.Type != "List" || len(node.Children) == 0 {
		// ast.ReportError("Expected list for statement", node.Line, node.Column)
	}
	head := node.Children[0].Value
	if head == "intent" {
		return ""
	}
	if ir, ok := ir.LowerShared(node); ok {
		return EmitJSIR(ir, reqVar, depth)
	}
	if head == "dom_query" {
		if len(node.Children) != 2 {
			// ast.ReportError("dom_query expects (dom_query selector)", node.Line, node.Column)
		}
		selector := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("document.querySelector(%s)", selector)
	} else if head == "on_event" {
		if len(node.Children) != 4 {
			// ast.ReportError("on_event expects (on_event el event lambda)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		event := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		lambda := node.Children[3]
		if lambda.Type != "List" || len(lambda.Children) != 3 || lambda.Children[0].Value != "lambda" {
			// ast.ReportError("on_event expects a lambda", lambda.Line, lambda.Column)
		}
		args := lambda.Children[1].Children
		argName := "e"
		if len(args) > 0 {
			argName = args[0].Value
		}
		body := generateJSStatement(lambda.Children[2], reqVar, depth+1)
		return fmt.Sprintf("%s.addEventListener(%s, async (%s) => {\n%s\n})", el, event, argName, body)
	} else if head == "set_html" {
		if len(node.Children) != 3 {
			// ast.ReportError("set_html expects (set_html el val)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		val := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("%s.innerHTML = %s", el, val)
	} else if head == "toggle_class" {
		if len(node.Children) != 3 {
			// ast.ReportError("toggle_class expects (toggle_class el class)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		cls := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("%s.classList.toggle(%s)", el, cls)
	} else if head == "set_text" {
		if len(node.Children) != 3 {
			// ast.ReportError("set_text expects (set_text el val)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		val := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("%s.textContent = %s", el, val)
	} else if head == "set_attr" {
		if len(node.Children) != 4 {
			// ast.ReportError("set_attr expects (set_attr el name val)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		attr := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		val := generateJSStatementRaw(node.Children[3], reqVar, depth+1)
		return fmt.Sprintf("%s.setAttribute(%s, %s)", el, attr, val)
	} else if head == "dom_value" {
		if len(node.Children) != 2 {
			// ast.ReportError("dom_value expects (dom_value el)", node.Line, node.Column)
		}
		el := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("%s.value", el)
	} else if head == "fetch" {
		if len(node.Children) != 3 && len(node.Children) != 4 {
			// ast.ReportError("fetch expects (fetch url method [body])", node.Line, node.Column)
		}
		urlStr := generateJSStatementRaw(node.Children[1], reqVar, depth+1)
		methodStr := generateJSStatementRaw(node.Children[2], reqVar, depth+1)
		if len(node.Children) == 4 {
			bodyStr := generateJSStatementRaw(node.Children[3], reqVar, depth+1)
			return fmt.Sprintf("(await fetch(%s, { method: %s, body: %s }).then(r => r.text()))", urlStr, methodStr, bodyStr)
		}
		return fmt.Sprintf("(await fetch(%s, { method: %s }).then(r => r.text()))", urlStr, methodStr)
	} else if head == "spawn_agent" {
		if len(node.Children) != 3 {
			// ast.ReportError("spawn_agent expects (spawn_agent name task)", node.Line, node.Column)
		}
		agentNameStr := generateJSExpression(node.Children[1], reqVar, depth+1)
		taskDescStr := generateJSExpression(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf(";(async () => {\n  console.log(`[Swarm JS] Spawning agent ${%s} for task: ${%s}`);\n  await new Promise(r => setTimeout(r, 100));\n  console.log(`[Swarm JS] Agent ${%s} completed task: ${%s}`);\n})();", agentNameStr, taskDescStr, agentNameStr, taskDescStr)
	} else if head == "task" {
		if len(node.Children) != 2 {
			// ast.ReportError("task expects (task desc)", node.Line, node.Column)
		}
		return generateJSExpression(node.Children[1], reqVar, depth+1)
	} else if head == "schema_bridge" {
		return generateJSStatementRaw(node.Children[2], reqVar, depth+1)
	} else if head == "optimize_signature" {
		return generateJSStatementRaw(node.Children[len(node.Children)-1], reqVar, depth+1)
	}

	// ast.ReportError(fmt.Sprintf("Unknown statement for JS: %s", head), node.Line, node.Column)
	return ""
}
