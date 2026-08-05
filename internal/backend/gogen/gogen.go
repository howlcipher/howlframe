package gogen

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"zero/internal/ast"
	"zero/internal/ir"
)

var CurrentSchemaDDLs []string

func GenerateCode(node *ast.Node) (string, string) {
	CurrentSchemaDDLs = nil
	if node.Type != "List" || len(node.Children) == 0 {
		// ast.ReportError("Expected list at root", node.Line, node.Column)
	}
	head := node.Children[0]
	if head.Type != "SYMBOL" || (head.Value != "http_server" && head.Value != "cli_app") {
		// ast.ReportError("Expected http_server or cli_app as root symbol", head.Line, head.Column)
	}

	isCliApp := head.Value == "cli_app"

	var portNode *ast.Node
	var startIndex int
	if isCliApp {
		startIndex = 1
	} else {
		if len(node.Children) < 3 {
			// ast.ReportError("http_server expects at least a port and 1 route", head.Line, head.Column)
		}
		portNode = node.Children[1]
		if portNode.Type != "INT" {
			// ast.ReportError("Expected integer for port", portNode.Line, portNode.Column)
		}
		startIndex = 2
	}

	var funcsCode string
	var routesCode string
	var cliCode string
	var testCode string
	var extraImports []string
	defaultImports := map[string]bool{
		"bytes":         true,
		"database/sql":  true,
		"encoding/json": true,
		"fmt":           true,
		"io":            true,
		"net/http":      true,
		"os":            true,
		"os/exec":       true,
		"regexp":        true,
		"runtime":       true,
		"runtime/debug": true,
		"strconv":       true,
		"strings":       true,
		"time":          true,
	}
	seenImports := make(map[string]bool)

	for i := startIndex; i < len(node.Children); i++ {
		handlerNode := node.Children[i]
		if handlerNode.Type != "List" || len(handlerNode.Children) == 0 {
			// ast.ReportError("Expected route, defun, struct, import, test, or middleware definition", handlerNode.Line, handlerNode.Column)
		}

		head := handlerNode.Children[0].Value

		if head == "intent" {
			continue
		}

		if head == "test" {
			if len(handlerNode.Children) < 3 {
				// ast.ReportError("test expects (test \"description\" body...)", handlerNode.Line, handlerNode.Column)
			}
			descNode := handlerNode.Children[1]
			if descNode.Type != "STRING" {
				// ast.ReportError("test description must be a string", descNode.Line, descNode.Column)
			}
			desc := descNode.Value
			safeDesc := ""
			lastWasUnderscore := false
			for _, r := range desc {
				if unicode.IsLetter(r) || unicode.IsDigit(r) {
					safeDesc += string(r)
					lastWasUnderscore = false
				} else {
					if !lastWasUnderscore {
						safeDesc += "_"
						lastWasUnderscore = true
					}
				}
			}
			safeDesc = strings.Trim(safeDesc, "_")
			testFuncName := "Test"
			if len(safeDesc) > 0 {
				testFuncName += "_" + safeDesc
			}

			var testBodyCode string
			for j := 2; j < len(handlerNode.Children); j++ {
				testBodyCode += generateStatement(handlerNode.Children[j], "", 0) + "\n"
			}
			testCode += fmt.Sprintf("func %s(t *testing.T) {\n%s\n}\n\n", testFuncName, testBodyCode)
			continue
		}

		if head == "go_import" {
			if len(handlerNode.Children) != 2 {
				// ast.ReportError("go_import expects (go_import \"pkg\")", handlerNode.Line, handlerNode.Column)
			}
			pkgNode := handlerNode.Children[1]
			if pkgNode.Type != "STRING" {
				// ast.ReportError("go_import package must be a string", pkgNode.Line, pkgNode.Column)
			}
			pkg := pkgNode.Value
			if !defaultImports[pkg] && !seenImports[pkg] {
				seenImports[pkg] = true
				extraImports = append(extraImports, pkg)
			}
			continue
		}

		if head == "struct" {
			if len(handlerNode.Children) < 2 {
				// ast.ReportError("struct expects (struct Name (field type)...)", handlerNode.Line, handlerNode.Column)
			}
			name := handlerNode.Children[1].Value
			funcsCode += fmt.Sprintf("type %s struct {\n", name)
			for j := 2; j < len(handlerNode.Children); j++ {
				fieldNode := handlerNode.Children[j]
				if fieldNode.Type != "List" || len(fieldNode.Children) != 2 {
					// ast.ReportError("struct field expects (name type)", fieldNode.Line, fieldNode.Column)
				}
				fieldName := fieldNode.Children[0].Value
				fieldType := fieldNode.Children[1].Value
				if len(fieldName) > 0 {
					fieldName = strings.ToUpper(fieldName[:1]) + fieldName[1:]
				}
				funcsCode += fmt.Sprintf("\t%s %s\n", fieldName, fieldType)
			}
			funcsCode += "}\n\n"
			continue
		}

		if head == "schema" {
			if len(handlerNode.Children) < 2 {
				// ast.ReportError("schema expects (schema \"tableName\" (column \"name\" \"type\")...)", handlerNode.Line, handlerNode.Column)
			}
			tableName := handlerNode.Children[1].Value
			structName := tableName
			if len(structName) > 0 {
				structName = strings.ToUpper(structName[:1]) + structName[1:]
			}
			funcsCode += fmt.Sprintf("type %s struct {\n", structName)

			var columns []string
			for j := 2; j < len(handlerNode.Children); j++ {
				colNode := handlerNode.Children[j]
				if colNode.Type != "List" {
					// ast.ReportError("schema column expects (column name type) or (name type)", colNode.Line, colNode.Column)
				}
				var colName, colType string
				if len(colNode.Children) == 3 && colNode.Children[0].Value == "column" {
					colName = colNode.Children[1].Value
					colType = colNode.Children[2].Value
				} else if len(colNode.Children) == 2 {
					colName = colNode.Children[0].Value
					colType = colNode.Children[1].Value
				} else {
					// ast.ReportError("schema column expects (column name type) or (name type)", colNode.Line, colNode.Column)
				}

				goFieldName := colName
				if len(goFieldName) > 0 {
					goFieldName = strings.ToUpper(goFieldName[:1]) + goFieldName[1:]
				}
				funcsCode += fmt.Sprintf("\t%s %s\n", goFieldName, colType)

				sqlType := colType
				if sqlType == "string" {
					sqlType = "TEXT"
				} else if sqlType == "int" {
					sqlType = "INTEGER"
				} else if sqlType == "float" || sqlType == "float64" {
					sqlType = "REAL"
				} else if sqlType == "bool" {
					sqlType = "BOOLEAN"
				}
				columns = append(columns, fmt.Sprintf("%s %s", colName, sqlType))
			}
			funcsCode += "}\n\n"

			ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s);", tableName, strings.Join(columns, ", "))
			CurrentSchemaDDLs = append(CurrentSchemaDDLs, ddl)
			continue
		}

		if head == "defun" {
			if len(handlerNode.Children) < 4 {
				// ast.ReportError("defun expects (defun name (args) body)", handlerNode.Line, handlerNode.Column)
			}
			name := handlerNode.Children[1].Value
			argsNode := handlerNode.Children[2]

			returnType := "string"
			bodyStartIndex := 3
			if len(handlerNode.Children) > 4 && handlerNode.Children[3].Type == "SYMBOL" {
				returnType = handlerNode.Children[3].Value
				bodyStartIndex = 4
			}

			typeHints := make(map[string]string)
			var typeParams []string
			for j := bodyStartIndex; j < len(handlerNode.Children)-1; j++ {
				cfgNode := handlerNode.Children[j]
				if cfgNode.Type == "List" && len(cfgNode.Children) >= 3 && cfgNode.Children[0].Value == "type_hint" {
					varName := cfgNode.Children[1].Value
					varType := cfgNode.Children[2].Value
					typeHints[varName] = varType
				} else if cfgNode.Type == "List" && len(cfgNode.Children) >= 2 && cfgNode.Children[0].Value == "type_param" {
					typeParams = append(typeParams, cfgNode.Children[1].Value)
				} else if cfgNode.Type == "List" && len(cfgNode.Children) >= 1 && cfgNode.Children[0].Value == "type_hints" {
					for k := 1; k < len(cfgNode.Children); k++ {
						hintPair := cfgNode.Children[k]
						if hintPair.Type == "List" && len(hintPair.Children) >= 2 {
							typeHints[hintPair.Children[0].Value] = hintPair.Children[1].Value
						}
					}
				}
			}

			typeParamsStr := ""
			if len(typeParams) > 0 {
				var typed []string
				for _, tp := range typeParams {
					typed = append(typed, tp+" any")
				}
				typeParamsStr = "[" + strings.Join(typed, ", ") + "]"
			}

			var argsList []string
			for _, arg := range argsNode.Children {
				var argName string
				argType := "string"
				if arg.Type == "List" && len(arg.Children) >= 2 {
					argName = arg.Children[0].Value
					argType = arg.Children[1].Value
				} else {
					argName = arg.Value
				}
				if t, ok := typeHints[argName]; ok {
					argType = t
				}
				argsList = append(argsList, argName+" "+argType)
			}
			argsStr := strings.Join(argsList, ", ")

			if t, ok := typeHints["return"]; ok {
				returnType = t
			}
			returnTypeStr := " " + returnType
			if returnType == "void" {
				returnTypeStr = ""
			}

			bodyNode := handlerNode.Children[len(handlerNode.Children)-1]
			bodyCode := generateStatement(bodyNode, "", 0)
			traceArgs := "map[string]any{"
			for _, arg := range argsList {
				argName := strings.Split(arg, " ")[0]
				traceArgs += fmt.Sprintf("%q: %s, ", argName, argName)
			}
			traceArgs += "}"
			traceInject := fmt.Sprintf("\tdefer observer.Trace(%q, %s)()\n", name, traceArgs)
			funcsCode += fmt.Sprintf("//line %s:%d\nfunc %s%s(%s)%s {\n%s%s\n}\n\n", handlerNode.Filename, handlerNode.Line, name, typeParamsStr, argsStr, returnTypeStr, traceInject, bodyCode)
			continue
		}

		if head == "lazy_synthesize" {
			if len(handlerNode.Children) != 4 {
				// ast.ReportError("lazy_synthesize expects (lazy_synthesize name (args) docstring)", handlerNode.Line, handlerNode.Column)
			}
			name := handlerNode.Children[1].Value
			argsNode := handlerNode.Children[2]
			docstring := handlerNode.Children[3].Value

			var params []string
			var argsList []string
			for _, arg := range argsNode.Children {
				var argName string
				if arg.Type == "List" && len(arg.Children) >= 1 {
					argName = arg.Children[0].Value
				} else {
					argName = arg.Value
				}
				params = append(params, argName)
				argsList = append(argsList, argName+" string")
			}
			argsStr := strings.Join(argsList, ", ")

			var promptExpr string
			if len(params) == 0 {
				prompt := fmt.Sprintf("You are a Zero compiler. Synthesize and directly execute the function %q with parameters []. Docstring: %q. Reply ONLY with the result value, no explanation, no markdown.", name, docstring)
				promptExpr = fmt.Sprintf("%q", prompt)
			} else {
				var inputFmtParts []string
				for _, p := range params {
					inputFmtParts = append(inputFmtParts, p+"=%v")
				}
				promptTemplate := fmt.Sprintf("You are a Zero compiler. Synthesize and directly execute the function %q with parameters %v. Docstring: %q. Given inputs %s, reply ONLY with the result value, no explanation, no markdown.", name, params, docstring, strings.Join(inputFmtParts, ", "))
				promptExpr = fmt.Sprintf("fmt.Sprintf(%q, %s)", promptTemplate, strings.Join(params, ", "))
			}

			bodyCode := fmt.Sprintf(`	reqBody, _ := json.Marshal(map[string]any{
		"model":  "llama3",
		"prompt": %s,
		"stream": false,
	})
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
	if err != nil { panic(err) }
	defer resp.Body.Close()
	var res struct {
		Response string `+"`json:\"response\"`"+`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { panic(err) }
	return strings.TrimSpace(res.Response)
`, promptExpr)

			funcsCode += fmt.Sprintf("//line %s:%d\nfunc %s(%s) string {\n%s}\n\n", handlerNode.Filename, handlerNode.Line, name, argsStr, bodyCode)
			continue
		}

		if head == "route" {
			if len(handlerNode.Children) != 3 {
				// ast.ReportError("route expects (route path handler)", handlerNode.Line, handlerNode.Column)
			}
			pathNode := handlerNode.Children[1]
			if pathNode.Type != "STRING" {
				// ast.ReportError("route path must be a string", pathNode.Line, pathNode.Column)
			}
			reqNodeList := handlerNode.Children[2].Children[1]
			if reqNodeList.Type != "List" || len(reqNodeList.Children) != 1 {
				// ast.ReportError("Expected exactly 1 argument in lambda (req)", reqNodeList.Line, reqNodeList.Column)
			}
			reqVar := reqNodeList.Children[0].Value
			bodyNode := handlerNode.Children[2].Children[2]
			bodyCode := generateStatement(bodyNode, reqVar, 0)
			traceInject := fmt.Sprintf("\t\tdefer observer.Trace(%q, map[string]any{%q: %s.URL.Path})()\n", "route:"+pathNode.Value, reqVar, reqVar)
			routesCode += fmt.Sprintf(`	http.HandleFunc(%q, func(w http.ResponseWriter, %s *http.Request) {
%s%s
	})
`, pathNode.Value, reqVar, traceInject, bodyCode)
			continue
		}

		if head == "middleware" {
			if len(handlerNode.Children) < 3 {
				// ast.ReportError("middleware expects (middleware (lambda (req) body) routes...)", handlerNode.Line, handlerNode.Column)
			}
			lambdaNode := handlerNode.Children[1]
			if lambdaNode.Type != "List" || len(lambdaNode.Children) != 3 || lambdaNode.Children[0].Value != "lambda" {
				// ast.ReportError("middleware expects a lambda", lambdaNode.Line, lambdaNode.Column)
			}
			reqNodeList := lambdaNode.Children[1]
			if reqNodeList.Type != "List" || len(reqNodeList.Children) != 1 {
				// ast.ReportError("middleware lambda expects exactly 1 argument", reqNodeList.Line, reqNodeList.Column)
			}
			mwReqVar := reqNodeList.Children[0].Value
			mwBodyNode := lambdaNode.Children[2]

			for j := 2; j < len(handlerNode.Children); j++ {
				routeNode := handlerNode.Children[j]
				if routeNode.Type != "List" || len(routeNode.Children) == 0 || routeNode.Children[0].Value != "route" {
					// ast.ReportError("middleware block can only contain routes", routeNode.Line, routeNode.Column)
				}
				if len(routeNode.Children) != 3 {
					// ast.ReportError("route expects (route path handler)", routeNode.Line, routeNode.Column)
				}
				pathNode := routeNode.Children[1]
				if pathNode.Type != "STRING" {
					// ast.ReportError("route path must be a string", pathNode.Line, pathNode.Column)
				}

				routeLambdaNode := routeNode.Children[2]
				routeReqList := routeLambdaNode.Children[1]
				routeReqVar := routeReqList.Children[0].Value
				routeBodyNode := routeLambdaNode.Children[2]

				clonedMwBody := ast.CopyNode(mwBodyNode)
				clonedRouteBody := ast.CopyNode(routeBodyNode)
				if routeReqVar != mwReqVar {
					ast.RenameVar(clonedRouteBody, routeReqVar, mwReqVar)
				}

				ast.ReplaceNext(clonedMwBody, clonedRouteBody)
				combinedCode := generateStatement(clonedMwBody, mwReqVar, 0)
				traceInject := fmt.Sprintf("\t\tdefer observer.Trace(%q, map[string]any{%q: %s.URL.Path})()\n", "middleware_route:"+pathNode.Value, mwReqVar, mwReqVar)

				routesCode += fmt.Sprintf(`	http.HandleFunc(%q, func(w http.ResponseWriter, %s *http.Request) {
%s%s
	})
`, pathNode.Value, mwReqVar, traceInject, combinedCode)
			}
			continue
		}

		if isCliApp {
			// For cli_app, unhandled blocks are treated as statements executed in main
			cliCode += generateStatement(handlerNode, "", 0) + "\n"
			continue
		}

		// ast.ReportError("Expected route, defun, struct, import, test, or middleware block", handlerNode.Line, handlerNode.Column)
	}

	code := `package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"zero/observer"
`
	for _, imp := range extraImports {
		code += fmt.Sprintf("\t%q\n", imp)
	}
	code += `)
`
	code += funcsCode
	code += `func main() {
	defer func() {
		if r := recover(); r != nil {
			crashData := struct {
				Error string
				Stack string
			}{
				Error: fmt.Sprintf("%v", r),
				Stack: string(debug.Stack()),
			}
			dump, _ := json.Marshal(crashData)
			_ = os.WriteFile("crash.json", dump, 0644)
			os.Exit(1)
		}
	}()
	var _ = runtime.GOOS
	var _ = debug.Stack
	var _ = sql.Open
	var _ = os.Getenv
	var _ = json.Marshal
	var _ = io.ReadAll
	var _ = bytes.NewBuffer
	var _ = http.DefaultClient
	var _ = exec.Command
	var _ = regexp.MatchString
	var _ = strings.Split
	var _ = time.Sleep
	var _ = strconv.Atoi
	var _ = fmt.Println
	var _ = observer.Trace
`
	if isCliApp {
		code += cliCode
		code += "}\n"
	} else {
		code += routesCode
		code += fmt.Sprintf(`	
	fmt.Println("Starting server on port %s...")
	if err := http.ListenAndServe(":%s", nil); err != nil {
		fmt.Println("Server error:", err)
	}
}
`, portNode.Value, portNode.Value)
	}

	if testCode != "" {
		fullTestCode := `package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"zero/observer"
`
		for _, imp := range extraImports {
			parts := strings.Split(imp, "/")
			pkgName := parts[len(parts)-1]
			if strings.Contains(testCode, pkgName+".") {
				fullTestCode += fmt.Sprintf("\t%q\n", imp)
			}
		}
		fullTestCode += `)

var _ = sql.Open
var _ = os.Getenv
var _ = json.Marshal
var _ = io.ReadAll
var _ = bytes.NewBuffer
var _ = http.DefaultClient
var _ = exec.Command
var _ = regexp.MatchString
var _ = strings.Split
var _ = time.Sleep
var _ = strconv.Atoi
var _ = fmt.Println
var _ = observer.Trace

` + testCode
		testCode = fullTestCode
	}

	return code, testCode
}

func generateStatement(node *ast.Node, reqVar string, depth int) string {
	code := generateStatementRaw(node, reqVar, depth)
	if node.Type != "List" || len(node.Children) == 0 {
		return code
	}
	head := node.Children[0].Value
	switch head {
	case "return", "res_json", "res", "let", "do", "try_let", "spawn", "spawn_agent", "task", "if", "print", "db_connect", "sql_query", "append", "map_set", "map_delete", "for", "sleep", "write_file", "mkdir", "exec", "while", "match", "set", "call", "cli_args":
		if node.Filename != "" {
			return fmt.Sprintf("//line %s:%d\n%s", node.Filename, node.Line, code)
		}
	}
	return code
}

func generateExpression(node *ast.Node, reqVar string, depth int) string {
	return generateStatementRaw(node, reqVar, depth)
}

func BinOpGoToken(head string) string {
	switch head {
	case "and":
		return "&&"
	case "or":
		return "||"
	case "=":
		return "=="
	default:
		return head
	}
}

func EmitGoIR(ir *ir.IRNode, reqVar string, depth int) string {
	switch ir.Kind {
	case "binop":
		if len(ir.Kids) != 2 {
			// ast.ReportError(fmt.Sprintf("%s expects 2 arguments", ir.Op), 0, 0)
		}
		arg1 := generateExpression(ir.Kids[0], reqVar, depth+1)
		arg2 := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("(%s %s %s)", arg1, BinOpGoToken(ir.Op), arg2)
	case "let":
		var letPrefix strings.Builder
		letPrefix.WriteString("		{\n")
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
					letPrefix.WriteString(fmt.Sprintf("			var %s %s\n			_ = json.NewDecoder(%s).Decode(&%s)\n			_ = %s\n", varName, targetType, bodyVar, varName, varName))
				} else {
					letPrefix.WriteString(fmt.Sprintf("			var %s %s\n			_ = json.Unmarshal([]byte(%s), &%s)\n			_ = %s\n", varName, targetType, bodyVar, varName, varName))
				}
				declaredVars[varName] = true
			} else {
				if declaredVars[varName] {
					letPrefix.WriteString(fmt.Sprintf("			%s = %s\n			_ = %s\n", varName, valStr, varName))
				} else {
					letPrefix.WriteString(fmt.Sprintf("			%s := %s\n			_ = %s\n", varName, valStr, varName))
					declaredVars[varName] = true
				}
			}

		}

		bodyCode := generateStatement(curr, reqVar, depth+1)
		return fmt.Sprintf("%s%s\n		}", letPrefix.String(), bodyCode)
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
				return fmt.Sprintf(`		{
			var %s %s
			if %s := json.NewDecoder(%s).Decode(&%s); %s != nil {
%s
			} else {
				_ = %s
%s
			}
		}`, varName, targetType, errVar, bodyVar, varName, errVar, catchBodyCode, varName, successBodyCode)
			} else {
				return fmt.Sprintf(`		{
			var %s %s
			if %s := json.Unmarshal([]byte(%s), &%s); %s != nil {
%s
			} else {
				_ = %s
%s
			}
		}`, varName, targetType, errVar, bodyVar, varName, errVar, catchBodyCode, varName, successBodyCode)
			}
		}

		valStr := generateStatement(valNode, reqVar, depth+1)
		return fmt.Sprintf(`		{
			%s, %s := %s
			if %s != nil {
%s
			} else {
				_ = %s
%s
			}
		}`, varName, errVar, valStr, errVar, catchBodyCode, varName, successBodyCode)
	case "spawn":
		lambdaNode := ir.Kids[0]
		bodyCode := generateStatement(lambdaNode.Children[2], reqVar, depth+1)
		traceInject := fmt.Sprintf("\t\tdefer observer.Trace(%q, map[string]any{})()\n", "spawn_lambda")
		return fmt.Sprintf("		go func() {\n%s%s\n		}()", traceInject, bodyCode)
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
		return fmt.Sprintf(`		for _, %s := range %s {
			_ = %s
%s
		}`, itemNode, listNode, itemNode, bodyCode)
	case "return":
		return fmt.Sprintf("		return %s", generateStatementRaw(ir.Kids[0], reqVar, depth+1))
	case "if":
		condExpr := generateExpression(ir.Kids[0], reqVar, depth+1)
		thenCode := generateStatement(ir.Kids[1], reqVar, depth+1)
		if len(ir.Kids) == 2 {
			return fmt.Sprintf(`		if %s {
%s
		}`, condExpr, thenCode)
		}
		elseCode := generateStatement(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf(`		if %s {
%s
		} else {
%s
		}`, condExpr, thenCode, elseCode)
	case "while":
		condExpr := generateExpression(ir.Kids[0], reqVar, depth+1)
		bodyCode := generateStatement(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf(`		for %s {
%s
		}`, condExpr, bodyCode)
	case "do":
		var stmts string
		for _, kid := range ir.Kids {
			stmts += generateStatement(kid, reqVar, depth+1) + "\n"
		}
		return fmt.Sprintf("		{\n%s\n		}", stmts)
	case "set":
		varStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		valStr := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("		%s = %s", varStr, valStr)
	case "match":
		varStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		var casesStr string
		for _, c := range ir.Cases {
			caseValStr := c.Label.Value
			if c.IsDefault {
				caseValStr = "default"
			} else if c.Label.Type == "STRING" {
				caseValStr = fmt.Sprintf("%q", caseValStr)
			}
			caseBodyCode := generateStatement(c.Body, reqVar, depth+1)
			if caseValStr == "default" {
				casesStr += fmt.Sprintf("		default:\n%s\n", caseBodyCode)
			} else {
				casesStr += fmt.Sprintf("		case %s:\n%s\n", caseValStr, caseBodyCode)
			}
		}
		return fmt.Sprintf("		switch %s {\n%s		}", varStr, casesStr)
	case "sleep":
		msStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("		time.Sleep(time.Duration(%s) * time.Millisecond)", msStr)
	case "to_int":
		valStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("func() int { v, _ := strconv.Atoi(%s); return v }()", valStr)
	case "to_float":
		valStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("func() float64 { v, _ := strconv.ParseFloat(%s, 64); return v }()", valStr)
	case "to_string":
		valStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("fmt.Sprint(%s)", valStr)
	case "bytes_to_string":
		valStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		return fmt.Sprintf("string(%s)", valStr)
	case "str_split":
		sStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		sepStr := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("strings.Split(%s, %s)", sStr, sepStr)
	case "str_join":
		listStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		sepStr := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("strings.Join(%s, %s)", listStr, sepStr)
	case "regex_match":
		patStr := generateExpression(ir.Kids[0], reqVar, depth+1)
		sStr := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("regexp.MatchString(%s, %s)", patStr, sStr)
	case "append":
		listNode := ir.Kids[0]
		if listNode.Type != "SYMBOL" {
			// ast.ReportError("append requires a symbol for list", listNode.Line, listNode.Column)
		}
		itemStr := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("		%s = append(%s, %s)", listNode.Value, listNode.Value, itemStr)
	case "map_set":
		dictNode := ir.Kids[0]
		if dictNode.Type != "SYMBOL" {
			// ast.ReportError("map_set requires a symbol for dict", dictNode.Line, dictNode.Column)
		}
		keyStr := generateExpression(ir.Kids[1], reqVar, depth+1)
		valStr := generateExpression(ir.Kids[2], reqVar, depth+1)
		return fmt.Sprintf("		%s[%s] = %s", dictNode.Value, keyStr, valStr)
	case "map_delete":
		dictNode := ir.Kids[0]
		if dictNode.Type != "SYMBOL" {
			// ast.ReportError("map_delete requires a symbol for dict", dictNode.Line, dictNode.Column)
		}
		keyStr := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("		delete(%s, %s)", dictNode.Value, keyStr)
	case "map_get":
		dictNode := ir.Kids[0]
		if dictNode.Type != "SYMBOL" {
			// ast.ReportError("map_get requires a symbol for dict", dictNode.Line, dictNode.Column)
		}
		keyStr := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("%s[%s]", dictNode.Value, keyStr)
	case "list_get":
		listNode := ir.Kids[0]
		if listNode.Type != "SYMBOL" {
			// ast.ReportError("list_get requires a symbol for list", listNode.Line, listNode.Column)
		}
		idxStr := generateExpression(ir.Kids[1], reqVar, depth+1)
		return fmt.Sprintf("func() string { _i, _ := strconv.Atoi(fmt.Sprint(%s)); if _i >= 0 && _i < len(%s) { return %s[_i] }; return \"\" }()", idxStr, listNode.Value, listNode.Value)
	case "list":
		var items []string
		for _, kid := range ir.Kids {
			if kid.Type == "STRING" {
				items = append(items, fmt.Sprintf("%q", kid.Value))
			} else {
				items = append(items, generateExpression(kid, reqVar, depth+1))
			}
		}
		return fmt.Sprintf("[]string{%s}", strings.Join(items, ", "))
	case "dict":
		var pairs []string
		for _, kid := range ir.Kids {
			if kid.Type != "List" || len(kid.Children) != 2 {
				// ast.ReportError("dict expects (k v) pairs", kid.Line, kid.Column)
			}
			k := kid.Children[0].Value
			if kid.Children[0].Type == "STRING" {
				k = fmt.Sprintf("%q", k)
			} else {
				k = generateExpression(kid.Children[0], reqVar, depth+1)
			}
			v := kid.Children[1].Value
			if kid.Children[1].Type == "STRING" {
				v = fmt.Sprintf("%q", v)
			} else {
				v = generateExpression(kid.Children[1], reqVar, depth+1)
			}
			pairs = append(pairs, fmt.Sprintf("%s: %s", k, v))
		}
		return fmt.Sprintf("map[string]string{%s}", strings.Join(pairs, ", "))
	case "print":
		var args []string
		for _, kid := range ir.Kids {
			args = append(args, generateExpression(kid, reqVar, depth+1))
		}
		return fmt.Sprintf("		fmt.Println(%s)", strings.Join(args, ", "))
	}
	// ast.ReportError(fmt.Sprintf("Unknown IR kind: %s", ir.Kind), 0, 0)
	return ""
}

func generateStatementRaw(node *ast.Node, reqVar string, depth int) string {
	if depth > 1000 {
		// ast.ReportError("AST too deep: exceeded maximum nesting limit of 1000", node.Line, node.Column)
	}
	if node.Type == "STRING" {
		return fmt.Sprintf("%q", node.Value)
	}
	if node.Type == "SYMBOL" || node.Type == "INT" || node.Type == "FLOAT" {
		if node.Value == "req.method" {
			return reqVar + ".Method"
		}
		return node.Value
	}
	if node.Type != "List" || len(node.Children) == 0 {
		// ast.ReportError("Expected list for statement", node.Line, node.Column)
	}
	head := node.Children[0].Value
	if head == "intent" {
		return ""
	}
	if head == "schema_bridge" {
		return generateStatementRaw(node.Children[2], reqVar, depth+1)
	}
	if head == "optimize_signature" {
		return generateStatementRaw(node.Children[len(node.Children)-1], reqVar, depth+1)
	}
	if ir, ok := ir.LowerShared(node); ok {
		return EmitGoIR(ir, reqVar, depth)
	}
	if head == "res_json" {
		if len(node.Children) != 3 {
			// ast.ReportError("res_json expects (res_json status data)", node.Line, node.Column)
		}
		status := node.Children[1].Value
		dataNode := node.Children[2]
		dataVar := dataNode.Value
		if dataNode.Type == "STRING" {
			dataVar = fmt.Sprintf("%q", dataVar)
		}
		return fmt.Sprintf(`		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(%s)
		_ = json.NewEncoder(w).Encode(%s)`, status, dataVar)
	} else if head == "res" {
		if len(node.Children) != 4 {
			// ast.ReportError("res expects status, contentType, and body", node.Line, node.Column)
		}
		status := node.Children[1].Value
		contentType := node.Children[2].Value
		resBody := node.Children[3].Value
		if node.Children[3].Type == "SYMBOL" || (node.Children[3].Type == "List" && node.Children[3].Children[0].Value == "call") {
			var bodyStr string
			if node.Children[3].Type == "List" {
				funcName := node.Children[3].Children[1].Value
				var args []string
				for j := 2; j < len(node.Children[3].Children); j++ {
					if node.Children[3].Children[j].Type == "STRING" {
						args = append(args, fmt.Sprintf("%q", node.Children[3].Children[j].Value))
					} else {
						args = append(args, node.Children[3].Children[j].Value)
					}
				}
				bodyStr = fmt.Sprintf("%s(%s)", funcName, strings.Join(args, ", "))
			} else {
				bodyStr = resBody
			}
			return fmt.Sprintf(`		w.Header().Set("Content-Type", %q)
		w.WriteHeader(%s)
		fmt.Fprint(w, %s)`, contentType, status, bodyStr)
		} else {
			return fmt.Sprintf(`		w.Header().Set("Content-Type", %q)
		w.WriteHeader(%s)
		fmt.Fprint(w, %q)`, contentType, status, resBody)
		}
	} else if head == "spawn_agent" {
		if len(node.Children) != 3 {
			// ast.ReportError("spawn_agent expects (spawn_agent name task)", node.Line, node.Column)
		}
		agentNameStr := generateExpression(node.Children[1], reqVar, depth+1)
		taskDescStr := generateExpression(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("		fmt.Printf(\"[Swarm Go] Spawning agent %%q for task: %%q\\n\", %s, %s)\n		go func(aName string, tDesc string) {\n			time.Sleep(100 * time.Millisecond)\n			fmt.Printf(\"[Swarm Go] Agent %%q completed task: %%q\\n\", aName, tDesc)\n		}(%s, %s)", agentNameStr, taskDescStr, agentNameStr, taskDescStr)
	} else if head == "task" {
		if len(node.Children) != 2 {
			// ast.ReportError("task expects (task desc)", node.Line, node.Column)
		}
		return generateExpression(node.Children[1], reqVar, depth+1)
	} else if head == "trace" {
		if len(node.Children) != 2 {
			// ast.ReportError("trace expects (trace var)", node.Line, node.Column)
		}
		varStr := generateStatement(node.Children[1], reqVar, depth+1)
		fileLine := fmt.Sprintf("[%s:%d]", node.Filename, node.Line)
		varName := node.Children[1].Value
		if node.Children[1].Type == "List" {
			varName = "expression"
		}
		return fmt.Sprintf("		fmt.Println(%q, %q, %s)", fileLine, varName+" =", varStr)
	} else if head == "db_connect" {
		if len(node.Children) != 4 {
			// ast.ReportError("db_connect expects (db_connect var driver dsn)", node.Line, node.Column)
		}
		varName := node.Children[1].Value
		driverNode := node.Children[2]
		dsnNode := node.Children[3]
		driverStr := generateExpression(driverNode, reqVar, depth+1)
		dsnStr := generateExpression(dsnNode, reqVar, depth+1)
		code := fmt.Sprintf("		%s, _ := sql.Open(%s, %s)\n		_ = %s", varName, driverStr, dsnStr, varName)
		for _, ddl := range CurrentSchemaDDLs {
			code += fmt.Sprintf("\n		%s.Exec(%q)", varName, ddl)
		}
		return code
	} else if head == "sql_query" {
		if len(node.Children) != 3 {
			// ast.ReportError("sql_query expects (sql_query db query)", node.Line, node.Column)
		}
		dbVar := node.Children[1].Value
		queryNode := node.Children[2]
		queryStr := generateExpression(queryNode, reqVar, depth+1)
		return fmt.Sprintf("		%s.Query(%s)", dbVar, queryStr)
	} else if head == "read_file" {
		if len(node.Children) != 2 {
			// ast.ReportError("read_file expects (read_file path)", node.Line, node.Column)
		}
		pathStr := generateStatement(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("os.ReadFile(%s)", pathStr)
	} else if head == "write_file" {
		if len(node.Children) != 3 {
			// ast.ReportError("write_file expects (write_file path data)", node.Line, node.Column)
		}
		pathStr := generateStatement(node.Children[1], reqVar, depth+1)
		dataStr := generateStatement(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf("		os.WriteFile(%s, []byte(%s), 0644)", pathStr, dataStr)
	} else if head == "mkdir" {
		if len(node.Children) != 2 {
			// ast.ReportError("mkdir expects (mkdir path)", node.Line, node.Column)
		}
		pathStr := generateStatement(node.Children[1], reqVar, depth+1)
		return fmt.Sprintf("		os.MkdirAll(%s, 0755)", pathStr)
	} else if head == "exec" {
		if len(node.Children) < 2 {
			// ast.ReportError("exec expects (exec cmd args...)", node.Line, node.Column)
		}
		cmdStr := generateStatement(node.Children[1], reqVar, depth+1)
		var args []string
		for j := 2; j < len(node.Children); j++ {
			args = append(args, generateStatement(node.Children[j], reqVar, depth+1))
		}
		return fmt.Sprintf("func() ([]byte, error) { return exec.Command(%s, %s).CombinedOutput() }()", cmdStr, strings.Join(args, ", "))
	} else if head == "rate_limit" {
		if len(node.Children) != 3 {
			// ast.ReportError("rate_limit expects (rate_limit \"10/s\" body)", node.Line, node.Column)
		}
		rateStr := node.Children[1].Value
		bodyCode := generateStatement(node.Children[2], reqVar, depth+1)
		// simple implementation: "10/s" -> sleep 100ms
		ms := 1000
		if strings.HasSuffix(rateStr, "/s") {
			n, _ := strconv.Atoi(strings.TrimSuffix(rateStr, "/s"))
			if n > 0 {
				ms = 1000 / n
			}
		}
		return fmt.Sprintf(`		{
			time.Sleep(%d * time.Millisecond)
			%s
		}`, ms, bodyCode)
	} else if head == "retry" {
		if len(node.Children) != 3 {
			// ast.ReportError("retry expects (retry times body)", node.Line, node.Column)
		}
		timesStr := generateStatement(node.Children[1], reqVar, depth+1)
		bodyCode := generateStatement(node.Children[2], reqVar, depth+1)
		return fmt.Sprintf(`		for i := 0; i < %s; i++ {
			%s
		}`, timesStr, bodyCode)
	} else if head == "fetch" {
		if len(node.Children) != 3 {
			// ast.ReportError("fetch expects (fetch url method)", node.Line, node.Column)
		}
		urlStr := generateStatement(node.Children[1], reqVar, depth+1)
		methodStr := generateStatement(node.Children[2], reqVar, depth+1)

		return fmt.Sprintf(`func() ([]byte, error) {
			req, err := http.NewRequest(%s, %s, nil)
			if err != nil { return nil, err }
			resp, err := http.DefaultClient.Do(req)
			if err != nil { return nil, err }
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		}()`, methodStr, urlStr)
	} else if head == "confidence" {
		if len(node.Children) != 2 {
			// ast.ReportError("confidence expects (confidence prompt)", node.Line, node.Column)
		}
		promptStr := generateStatement(node.Children[1], reqVar, depth+1)

		return fmt.Sprintf(`func() float64 {
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": fmt.Sprintf("Evaluate the probability of this statement being true. Return ONLY a float between 0.0 and 1.0. Statement: %%v", %s),
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { panic(err) }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { panic(err) }
			val, _ := strconv.ParseFloat(strings.TrimSpace(res.Response), 64)
			return val
		}()`, promptStr)
	} else if head == "achieve" {
		if len(node.Children) != 3 {
			// ast.ReportError("achieve expects (achieve target constraint)", node.Line, node.Column)
		}
		targetStr := ast.Stringify(node.Children[1])
		constraintStr := ast.Stringify(node.Children[2])

		prompt := fmt.Sprintf("Achieve the following target: %s with constraint: %s. Return ONLY the result, no explanations.", targetStr, constraintStr)

		return fmt.Sprintf(`func() string {
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": %q,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { panic(err) }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { panic(err) }
			return res.Response
		}()`, prompt)
	} else if head == "llm_generate" {
		if len(node.Children) < 2 {
			// ast.ReportError("llm_generate expects (llm_generate prompt [model])", node.Line, node.Column)
		}
		promptStr := generateStatement(node.Children[1], reqVar, depth+1)
		modelStr := `"llama3"`
		if len(node.Children) >= 3 {
			modelStr = generateStatement(node.Children[2], reqVar, depth+1)
		}

		return fmt.Sprintf(`func() (string, error) {
			reqBody, _ := json.Marshal(map[string]any{
				"model":  %s,
				"prompt": %s,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { return "", err }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return "", err }
			return res.Response, nil
		}()`, modelStr, promptStr)
	} else if head == "neural_circuit" {
		if len(node.Children) < 3 {
			// ast.ReportError("neural_circuit expects (neural_circuit (args) \"instruction\")", node.Line, node.Column)
		}
		argsNode := node.Children[1]
		instructionStr := generateStatement(node.Children[2], reqVar, depth+1)

		var argVals []string
		for _, arg := range argsNode.Children {
			argVals = append(argVals, generateExpression(arg, reqVar, depth+1))
		}

		promptVar := ""
		if len(argVals) > 0 {
			promptVar = fmt.Sprintf("fmt.Sprintf(\"Instruction: %%s\\nInputs: %%v\", %s, []any{%s})", instructionStr, strings.Join(argVals, ", "))
		} else {
			promptVar = fmt.Sprintf("fmt.Sprintf(\"Instruction: %%s\", %s)", instructionStr)
		}

		return fmt.Sprintf(`func() string {
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": %s,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { panic(err) }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { panic(err) }
			return res.Response
		}()`, promptVar)
	} else if head == "optimize_block" {
		if len(node.Children) < 4 {
			// ast.ReportError("optimize_block expects (optimize_block \"metric_name\" threshold_ms body...)", node.Line, node.Column)
		}
		metricName := generateExpression(node.Children[1], reqVar, depth)
		threshold := generateExpression(node.Children[2], reqVar, depth)

		bodyNode := &ast.Node{Type: "LIST", Children: append([]*ast.Node{{Type: "SYMBOL", Value: "do"}}, node.Children[3:]...)}
		bodyGo := generateStatement(bodyNode, reqVar, depth+1)

		return fmt.Sprintf(`func() {
			if optFn, ok := observer.GetOptimizedPlugin(%s); ok {
				optFn()
				return
			}
			start := time.Now()
			func() {
				%s
			}()
			if time.Since(start).Milliseconds() > int64(%s) {
				go observer.OptimizeGoImplementation(%s, %q)
			}
		}()`, metricName, bodyGo, threshold, metricName, bodyGo)
	} else if head == "ephemeral_circuit" {
		if len(node.Children) < 3 {
			// ast.ReportError("ephemeral_circuit expects (ephemeral_circuit (args) \"instruction\")", node.Line, node.Column)
		}
		argsNode := node.Children[1]
		instructionStr := generateStatement(node.Children[2], reqVar, depth+1)

		var argVals []string
		for _, arg := range argsNode.Children {
			argVals = append(argVals, generateExpression(arg, reqVar, depth+1))
		}

		promptVar := ""
		if len(argVals) > 0 {
			promptVar = fmt.Sprintf("fmt.Sprintf(\"Inputs: %%v\", []any{%s})", strings.Join(argVals, ", "))
		} else {
			promptVar = `""`
		}

		return fmt.Sprintf(`func() string {
			modelName := fmt.Sprintf("ephemeral-%%d", time.Now().UnixNano())
			modelfile := fmt.Sprintf("FROM llama3\nSYSTEM You are a highly specialized reasoning circuit. Your task is: %%s", %s)
			
			createReq, _ := json.Marshal(map[string]any{
				"name":      modelName,
				"modelfile": modelfile,
				"stream":    false,
			})
			createResp, err := http.Post("http://localhost:11434/api/create", "application/json", bytes.NewReader(createReq))
			if err != nil { panic(err) }
			createResp.Body.Close()

			defer func() {
				delReq, _ := json.Marshal(map[string]any{"name": modelName})
				req, _ := http.NewRequest("DELETE", "http://localhost:11434/api/delete", bytes.NewReader(delReq))
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{}
				resp, _ := client.Do(req)
				if resp != nil { resp.Body.Close() }
			}()

			reqBody, _ := json.Marshal(map[string]any{
				"model":  modelName,
				"prompt": %s,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { panic(err) }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { panic(err) }
			return res.Response
		}()`, instructionStr, promptVar)
	} else if head == "fuzzy_cast" {
		if len(node.Children) < 3 {
			// ast.ReportError("fuzzy_cast expects (fuzzy_cast Type var [model])", node.Line, node.Column)
		}
		typeStr := node.Children[1].Value
		varStr := generateStatement(node.Children[2], reqVar, depth+1)
		modelStr := `"llama3"`
		if len(node.Children) >= 4 {
			modelStr = generateStatement(node.Children[3], reqVar, depth+1)
		}

		return fmt.Sprintf(`func() (%s, error) {
			var out %s
			reqBody, _ := json.Marshal(map[string]any{
				"model":  %s,
				"prompt": fmt.Sprintf("Coerce this input into a valid JSON object matching the requested schema. Reply strictly with the JSON object and nothing else.\nInput: %%s", %s),
				"stream": false,
				"format": "json",
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { return out, err }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return out, err }
			err = json.Unmarshal([]byte(res.Response), &out)
			return out, err
		}()`, typeStr, typeStr, modelStr, varStr)
	} else if head == "semantic_match" {
		if len(node.Children) < 3 {
			// ast.ReportError("semantic_match expects (semantic_match input (intent consequence)...)", node.Line, node.Column)
		}
		varStr := generateStatement(node.Children[1], reqVar, depth+1)
		var casesStr string
		var intentExprs []string

		for i := 2; i < len(node.Children); i++ {
			caseNode := node.Children[i]
			if caseNode.Type != "LIST" || len(caseNode.Children) < 2 {
				// ast.ReportError("semantic_match case expects (intent consequence...)", caseNode.Line, caseNode.Column)
			}
			label := caseNode.Children[0]
			isDefault := label.Value == "default" || label.Value == "_"

			bodyNode := &ast.Node{Type: "LIST", Children: append([]*ast.Node{{Type: "SYMBOL", Value: "do"}}, caseNode.Children[1:]...)}
			bodyCode := generateStatement(bodyNode, reqVar, depth+1)

			if isDefault {
				casesStr += fmt.Sprintf("\t\t\tdefault:\n%s\n", bodyCode)
			} else {
				intentIdx := len(intentExprs)
				intentExpr := generateExpression(label, reqVar, depth+1)
				intentExprs = append(intentExprs, intentExpr)
				casesStr += fmt.Sprintf("\t\t\tcase %d:\n%s\n", intentIdx, bodyCode)
			}
		}

		intentsArrayStr := "[]string{" + strings.Join(intentExprs, ", ") + "}"

		return fmt.Sprintf(`func() {
			intents := %s
			intentsStr := ""
			for i, intent := range intents {
				intentsStr += fmt.Sprintf("%%d: %%s\n", i, intent)
			}
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": fmt.Sprintf("Determine which of the following intents best matches the given input string. Return ONLY the integer ID of the matching intent. If none match, return 'default'.\n\nIntents:\n%%s\nInput: %%s", intentsStr, %s),
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { panic(err) }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { panic(err) }

			ans := strings.TrimSpace(strings.ToLower(res.Response))
			idx, err := strconv.Atoi(ans)
			if err != nil {
				idx = -1
			}
			switch idx {
%s			}
		}()`, intentsArrayStr, varStr, casesStr)
	} else if head == "assert_semantic" {
		if len(node.Children) != 3 {
			// ast.ReportError("assert_semantic expects (assert_semantic var \"condition\")", node.Line, node.Column)
		}
		varStr := generateStatement(node.Children[1], reqVar, depth+1)
		condStr := generateStatement(node.Children[2], reqVar, depth+1)

		return fmt.Sprintf(`func() bool {
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": fmt.Sprintf("Does this input satisfy the condition: '%%s'? Reply strictly with 'true' or 'false' and nothing else.\nInput: %%s", %s, %s),
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil { return false }
			defer resp.Body.Close()
			var res struct {
				Response string `+"`json:\"response\"`"+`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil { return false }
			return strings.TrimSpace(strings.ToLower(res.Response)) == "true"
		}()`, condStr, varStr)
	} else if head == "cli_args" {
		if len(node.Children) == 1 {
			return "os.Args[1:]"
		} else if len(node.Children) == 2 {
			idxStr := generateStatement(node.Children[1], reqVar, depth+1)
			return fmt.Sprintf("func() string { _idx, _ := strconv.Atoi(fmt.Sprint(%s)); if len(os.Args) > _idx+1 { return os.Args[_idx+1] }; return \"\" }()", idxStr)
		} else {
			// ast.ReportError("cli_args expects (cli_args) or (cli_args index)", node.Line, node.Column)
		}
	}
	// ast.ReportError(fmt.Sprintf("Unknown statement: %s", head), node.Line, node.Column)
	return ""
}
