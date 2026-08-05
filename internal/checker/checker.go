package checker

import (
	"fmt"
	"zero/internal/ast"
	"zero/internal/ir"
)

func Check(node *ast.Node) *Analysis {
	if node == nil {
		return Analyze(nil)
	}
	analysis := Analyze(node)
	if len(analysis.Diagnostics) > 0 {
		diagnostic := analysis.Diagnostics[0]
		ast.ReportError(diagnostic.Reason, diagnostic.Line, diagnostic.Column)
	}
	if node.Type != "List" || len(node.Children) == 0 {
		ast.ReportError("Expected list at root", node.Line, node.Column)
	}

	head := node.Children[0]
	if head.Type != "SYMBOL" {
		ast.ReportError("Expected symbol at root", head.Line, head.Column)
	}

	switch head.Value {
	case "wasm_app":
		checkWasmApp(node)
	case "web_app":
		checkWebApp(node)
	case "http_server", "cli_app":
		checkGoApp(node)
	default:
		ast.ReportError(fmt.Sprintf("Expected http_server, cli_app, web_app, or wasm_app as root symbol, got %s", head.Value), head.Line, head.Column)
	}
	return analysis
}

func checkWasmApp(node *ast.Node) {
	if len(node.Children) != 2 {
		ast.ReportError("wasm_app expects exactly one result expression", node.Line, node.Column)
	}
	checkWasmExpression(node.Children[1])
}

func checkWasmExpression(node *ast.Node) {
	if node == nil {
		return
	}
	if node.Type == "INT" {
		return
	}
	if node.Type == "FLOAT" {
		return
	}
	if node.Type == "STRING" {
		return
	}
	if node.Type == "SYMBOL" {
		if node.Value != "true" && node.Value != "false" {
			ast.ReportError(fmt.Sprintf("Wasm backend does not support symbol %q", node.Value), node.Line, node.Column)
		}
		return
	}
	if node.Type != "List" {
		ast.ReportError(fmt.Sprintf("Wasm backend does not support %s literals", node.Type), node.Line, node.Column)
	}
	irNode, ok := ir.LowerShared(node)
	if !ok {
		if len(node.Children) == 0 {
			ast.ReportError("Wasm backend does not support an empty expression", node.Line, node.Column)
		}
		ast.ReportError(fmt.Sprintf("Wasm backend does not support %q", node.Children[0].Value), node.Line, node.Column)
	}
	switch irNode.Kind {
	case "binop":
		if len(irNode.Kids) != 2 {
			ast.ReportError(fmt.Sprintf("%s expects 2 arguments", irNode.Op), node.Line, node.Column)
		}
		checkWasmExpression(irNode.Kids[0])
		checkWasmExpression(irNode.Kids[1])
	case "if":
		if len(irNode.Kids) != 3 {
			ast.ReportError("Wasm backend requires if to include an else branch", node.Line, node.Column)
		}
		checkWasmExpression(irNode.Kids[0])
		checkWasmExpression(irNode.Kids[1])
		checkWasmExpression(irNode.Kids[2])
	case "do":
		if len(irNode.Kids) == 0 {
			ast.ReportError("Wasm backend requires do to contain a result expression", node.Line, node.Column)
		}
		for _, kid := range irNode.Kids {
			checkWasmExpression(kid)
		}
	case "list":
		if irNode.Type.Element == nil || (irNode.Type.Element.Kind != ast.Int && irNode.Type.Element.Kind != ast.String) {
			ast.ReportError("Wasm list aggregates currently require int or string elements", node.Line, node.Column)
		}
		for _, kid := range irNode.Kids {
			if kid.Type != "STRING" {
				checkWasmExpression(kid)
			}
		}
	case "list_get":
		if len(irNode.Kids) != 2 || irNode.Kids[0].Inferred.Element == nil || (irNode.Kids[0].Inferred.Element.Kind != ast.Int && irNode.Kids[0].Inferred.Element.Kind != ast.String) || irNode.Kids[1].Inferred.Kind != ast.Int {
			ast.ReportError("Wasm list_get requires an int/string list and integer index", node.Line, node.Column)
		}
		checkWasmExpression(irNode.Kids[0])
		checkWasmExpression(irNode.Kids[1])
	case "dict":
		if irNode.Type.Element == nil || (irNode.Type.Element.Kind != ast.Int && irNode.Type.Element.Kind != ast.String) {
			ast.ReportError("Wasm dict aggregates currently require int or string values", node.Line, node.Column)
		}
		for _, pair := range irNode.Kids {
			if pair.Type != "List" || len(pair.Children) != 2 || pair.Children[0].Type != "STRING" ||
				(pair.Children[1].Inferred.Kind != ast.Int && pair.Children[1].Inferred.Kind != ast.String) {
				ast.ReportError("Wasm dict aggregates require string keys and int/string values", node.Line, node.Column)
			}
			valueType := pair.Children[1].Inferred.Kind
			if irNode.Type.Element != nil && known(ast.TypeInfo{Kind: valueType}) && valueType != irNode.Type.Element.Kind {
				ast.ReportError("Wasm dict aggregates require homogeneous value types", node.Line, node.Column)
			}
		}
	case "map_get":
		if len(irNode.Kids) != 2 || irNode.Kids[0].Type != "List" || len(irNode.Kids[0].Children) == 0 || irNode.Kids[0].Children[0].Value != "dict" || irNode.Kids[1].Inferred.Kind != ast.String {
			ast.ReportError("Wasm map_get requires a static dict and string key expression", node.Line, node.Column)
		}
		checkWasmExpression(irNode.Kids[0])
		checkWasmExpression(irNode.Kids[1])
	case "return":
		checkWasmExpression(irNode.Kids[0])
	case "to_float":
		if len(irNode.Kids) != 1 || (irNode.Kids[0].Inferred.Kind != ast.Int && irNode.Kids[0].Inferred.Kind != ast.Float) {
			ast.ReportError("Wasm to_float requires an int or float operand", node.Line, node.Column)
		}
		checkWasmExpression(irNode.Kids[0])
	case "to_int":
		if len(irNode.Kids) != 1 || (irNode.Kids[0].Inferred.Kind != ast.Int && irNode.Kids[0].Inferred.Kind != ast.Float) {
			ast.ReportError("Wasm to_int requires an int or float operand", node.Line, node.Column)
		}
		checkWasmExpression(irNode.Kids[0])
	default:
		ast.ReportError(fmt.Sprintf("Wasm backend does not support %q", irNode.Kind), node.Line, node.Column)
	}
}

func checkWebApp(node *ast.Node) {
	for i := 1; i < len(node.Children); i++ {
		checkWebAppHandler(node.Children[i])
	}
}

func checkWebAppHandler(handlerNode *ast.Node) {
	if handlerNode.Type != "List" || len(handlerNode.Children) == 0 {
		checkJSStatement(handlerNode, 0)
		return
	}
	headVal := handlerNode.Children[0].Value
	if headVal == "module" {
		for j := 2; j < len(handlerNode.Children); j++ {
			checkWebAppHandler(handlerNode.Children[j])
		}
		return
	}
	if headVal == "export" {
		checkWebAppHandler(handlerNode.Children[1])
		return
	}
	if headVal == "intent" {
		return
	}
	if headVal == "test" {
		if len(handlerNode.Children) < 3 {
			ast.ReportError(`test expects (test "description" body...)`, handlerNode.Line, handlerNode.Column)
		}
		descNode := handlerNode.Children[1]
		if descNode.Type != "STRING" {
			ast.ReportError("test description must be a string", descNode.Line, descNode.Column)
		}
		for j := 2; j < len(handlerNode.Children); j++ {
			checkJSStatement(handlerNode.Children[j], 0)
		}
		return
	}
	if headVal == "defun" {
		if len(handlerNode.Children) < 4 {
			ast.ReportError("defun expects (defun name (args) body)", handlerNode.Line, handlerNode.Column)
		}
		bodyNode := handlerNode.Children[len(handlerNode.Children)-1]
		checkJSStatement(bodyNode, 0)
		return
	}
	checkJSStatement(handlerNode, 0)
}

func checkJSStatement(node *ast.Node, depth int) {
	if depth > 1000 {
		ast.ReportError("AST too deep", node.Line, node.Column)
	}
	if node.Type == "STRING" || node.Type == "SYMBOL" || node.Type == "INT" || node.Type == "FLOAT" {
		return
	}
	if node.Type != "List" || len(node.Children) == 0 {
		ast.ReportError("Expected list for statement", node.Line, node.Column)
	}
	head := node.Children[0].Value
	if head == "intent" {
		return
	}
	if irNode, ok := ir.LowerShared(node); ok {
		switch irNode.Kind {
		case "let":
			bindings, body := ast.LetChain(node)
			for _, binding := range bindings {
				checkJSStatement(binding.Children[1], depth+1)
			}
			checkJSStatement(body, depth+1)
		case "binop":
			checkJSStatement(irNode.Kids[0], depth+1)
			checkJSStatement(irNode.Kids[1], depth+1)
		case "dict":
			for _, kid := range irNode.Kids {
				if kid.Type != "List" || len(kid.Children) != 2 {
					ast.ReportError("dict expects (k v) pairs", kid.Line, kid.Column)
				}
				if kid.Children[1].Type != "STRING" {
					checkJSStatement(kid.Children[1], depth+1)
				}
			}
		default:
			for _, kid := range irNode.Kids {
				checkJSStatement(kid, depth+1)
			}
			for _, c := range irNode.Cases {
				checkJSStatement(c.Body, depth+1)
			}
		}
		return
	}

	if head == "dom_query" {
		if len(node.Children) != 2 {
			ast.ReportError("dom_query expects (dom_query selector)", node.Line, node.Column)
		}
		checkJSStatement(node.Children[1], depth+1)
	} else if head == "on_event" {
		if len(node.Children) != 4 {
			ast.ReportError("on_event expects (on_event el event lambda)", node.Line, node.Column)
		}
		checkJSStatement(node.Children[1], depth+1)
		checkJSStatement(node.Children[2], depth+1)
		lambda := node.Children[3]
		if lambda.Type != "List" || len(lambda.Children) != 3 || lambda.Children[0].Value != "lambda" {
			ast.ReportError("on_event expects a lambda", lambda.Line, lambda.Column)
		}
		checkJSStatement(lambda.Children[2], depth+1)
	} else if head == "set_text" {
		if len(node.Children) != 3 {
			ast.ReportError("set_text expects (set_text el val)", node.Line, node.Column)
		}
		checkJSStatement(node.Children[1], depth+1)
		checkJSStatement(node.Children[2], depth+1)
	} else if head == "set_attr" {
		if len(node.Children) != 4 {
			ast.ReportError("set_attr expects (set_attr el name val)", node.Line, node.Column)
		}
		checkJSStatement(node.Children[1], depth+1)
		checkJSStatement(node.Children[2], depth+1)
		checkJSStatement(node.Children[3], depth+1)
	} else if head == "fetch" {
		if len(node.Children) != 3 {
			ast.ReportError("fetch expects (fetch url method)", node.Line, node.Column)
		}
		checkJSStatement(node.Children[1], depth+1)
		checkJSStatement(node.Children[2], depth+1)
	} else if head == "spawn_agent" {
		if len(node.Children) != 3 {
			ast.ReportError("spawn_agent expects (spawn_agent name task)", node.Line, node.Column)
		}
		checkJSStatement(node.Children[1], depth+1)
		checkJSStatement(node.Children[2], depth+1)
	} else if head == "task" {
		if len(node.Children) != 2 {
			ast.ReportError("task expects (task desc)", node.Line, node.Column)
		}
		checkJSStatement(node.Children[1], depth+1)
	} else {
		ast.ReportError(fmt.Sprintf("Unknown statement for JS: %s", head), node.Line, node.Column)
	}
}

func checkGoApp(node *ast.Node) {
	head := node.Children[0]
	isCliApp := head.Value == "cli_app"
	startIndex := 1
	if !isCliApp {
		if len(node.Children) < 3 {
			ast.ReportError("http_server expects at least a port and 1 route", head.Line, head.Column)
		}
		portNode := node.Children[1]
		if portNode.Type != "INT" {
			ast.ReportError("Expected integer for port", portNode.Line, portNode.Column)
		}
		startIndex = 2
	}
	for i := startIndex; i < len(node.Children); i++ {
		checkGoAppHandler(node.Children[i], isCliApp)
	}
}

func checkGoAppHandler(handlerNode *ast.Node, isCliApp bool) {
	if handlerNode.Type != "List" || len(handlerNode.Children) == 0 {
		ast.ReportError("Expected route, defun, struct, import, test, export, module, or middleware definition", handlerNode.Line, handlerNode.Column)
	}
	hHead := handlerNode.Children[0].Value
	switch hHead {
	case "module":
		for j := 2; j < len(handlerNode.Children); j++ {
			checkGoAppHandler(handlerNode.Children[j], isCliApp)
		}
	case "export":
		checkGoAppHandler(handlerNode.Children[1], isCliApp)
	case "intent":
		// skip
	case "test":
		if len(handlerNode.Children) < 3 {
			ast.ReportError(`test expects (test "description" body...)`, handlerNode.Line, handlerNode.Column)
		}
		descNode := handlerNode.Children[1]
		if descNode.Type != "STRING" {
			ast.ReportError("test description must be a string", descNode.Line, descNode.Column)
		}
		for j := 2; j < len(handlerNode.Children); j++ {
			checkGoStatement(handlerNode.Children[j], 0)
		}
	case "go_import":
		if len(handlerNode.Children) != 2 {
			ast.ReportError(`go_import expects (go_import "pkg")`, handlerNode.Line, handlerNode.Column)
		}
		pkgNode := handlerNode.Children[1]
		if pkgNode.Type != "STRING" {
			ast.ReportError("go_import package must be a string", pkgNode.Line, pkgNode.Column)
		}
	case "struct":
		if len(handlerNode.Children) < 2 {
			ast.ReportError("struct expects (struct Name (field type)...)", handlerNode.Line, handlerNode.Column)
		}
		for j := 2; j < len(handlerNode.Children); j++ {
			fieldNode := handlerNode.Children[j]
			if fieldNode.Type != "List" || len(fieldNode.Children) != 2 {
				ast.ReportError("struct field expects (name type)", fieldNode.Line, fieldNode.Column)
			}
		}
	case "schema":
		if len(handlerNode.Children) < 2 {
			ast.ReportError(`schema expects (schema "tableName" (column "name" "type")...)`, handlerNode.Line, handlerNode.Column)
		}
		for j := 2; j < len(handlerNode.Children); j++ {
			colNode := handlerNode.Children[j]
			if colNode.Type != "List" {
				ast.ReportError("schema column expects (column name type) or (name type)", colNode.Line, colNode.Column)
			}
			if len(colNode.Children) == 3 && colNode.Children[0].Value == "column" {
			} else if len(colNode.Children) == 2 {
			} else {
				ast.ReportError("schema column expects (column name type) or (name type)", colNode.Line, colNode.Column)
			}
		}
	case "defun":
		if len(handlerNode.Children) < 4 {
			ast.ReportError("defun expects (defun name (args) body)", handlerNode.Line, handlerNode.Column)
		}
		bodyNode := handlerNode.Children[len(handlerNode.Children)-1]
		checkGoStatement(bodyNode, 0)
	case "route":
		if len(handlerNode.Children) != 3 {
			ast.ReportError("route expects (route path handler)", handlerNode.Line, handlerNode.Column)
		}
		pathNode := handlerNode.Children[1]
		if pathNode.Type != "STRING" {
			ast.ReportError("route path must be a string", pathNode.Line, pathNode.Column)
		}
		reqNodeList := handlerNode.Children[2].Children[1]
		if reqNodeList.Type != "List" || len(reqNodeList.Children) != 1 {
			ast.ReportError("Expected exactly 1 argument in lambda (req)", reqNodeList.Line, reqNodeList.Column)
		}
		bodyNode := handlerNode.Children[2].Children[2]
		checkGoStatement(bodyNode, 0)
	case "middleware":
		if len(handlerNode.Children) < 3 {
			ast.ReportError("middleware expects (middleware (lambda (req) body) routes...)", handlerNode.Line, handlerNode.Column)
		}
		lambdaNode := handlerNode.Children[1]
		if lambdaNode.Type != "List" || len(lambdaNode.Children) != 3 || lambdaNode.Children[0].Value != "lambda" {
			ast.ReportError("middleware expects a lambda", lambdaNode.Line, lambdaNode.Column)
		}
		reqNodeList := lambdaNode.Children[1]
		if reqNodeList.Type != "List" || len(reqNodeList.Children) != 1 {
			ast.ReportError("middleware lambda expects exactly 1 argument", reqNodeList.Line, reqNodeList.Column)
		}
		mwBodyNode := lambdaNode.Children[2]
		checkGoStatement(mwBodyNode, 0)
		for j := 2; j < len(handlerNode.Children); j++ {
			routeNode := handlerNode.Children[j]
			if routeNode.Type != "List" || len(routeNode.Children) == 0 || routeNode.Children[0].Value != "route" {
				ast.ReportError("middleware block can only contain routes", routeNode.Line, routeNode.Column)
			}
			if len(routeNode.Children) != 3 {
				ast.ReportError("route expects (route path handler)", routeNode.Line, routeNode.Column)
			}
			pathNode := routeNode.Children[1]
			if pathNode.Type != "STRING" {
				ast.ReportError("route path must be a string", pathNode.Line, pathNode.Column)
			}
			routeLambdaNode := routeNode.Children[2]
			routeBodyNode := routeLambdaNode.Children[2]
			checkGoStatement(routeBodyNode, 0)
		}
	default:
		if isCliApp {
			checkGoStatement(handlerNode, 0)
		} else {
			ast.ReportError("Expected route, defun, struct, import, test, module, export, or middleware block", handlerNode.Line, handlerNode.Column)
		}
	}
}

func checkGoStatement(node *ast.Node, depth int) {
	if node == nil {
		return
	}
	irNode, ok := ir.LowerShared(node)
	if ok {
		switch irNode.Kind {
		case "let":
			bindings, body := ast.LetChain(node)
			for _, binding := range bindings {
				checkGoStatement(binding.Children[1], depth+1)
			}
			checkGoStatement(body, depth+1)
		case "binop":
			if len(irNode.Kids) != 2 {
				ast.ReportError(fmt.Sprintf("%s expects 2 arguments", irNode.Op), node.Line, node.Column)
			}
			checkGoStatement(irNode.Kids[0], depth+1)
			checkGoStatement(irNode.Kids[1], depth+1)
		case "append":
			listNode := irNode.Kids[0]
			if listNode.Type != "SYMBOL" {
				ast.ReportError("append requires a symbol for list", listNode.Line, listNode.Column)
			}
			checkGoStatement(irNode.Kids[1], depth+1)
		case "map_set":
			dictNode := irNode.Kids[0]
			if dictNode.Type != "SYMBOL" {
				ast.ReportError("map_set requires a symbol for dict", dictNode.Line, dictNode.Column)
			}
			checkGoStatement(irNode.Kids[1], depth+1)
			checkGoStatement(irNode.Kids[2], depth+1)
		case "map_delete":
			dictNode := irNode.Kids[0]
			if dictNode.Type != "SYMBOL" {
				ast.ReportError("map_delete requires a symbol for dict", dictNode.Line, dictNode.Column)
			}
			checkGoStatement(irNode.Kids[1], depth+1)
		case "map_get":
			dictNode := irNode.Kids[0]
			if dictNode.Type != "SYMBOL" {
				ast.ReportError("map_get requires a symbol for dict", dictNode.Line, dictNode.Column)
			}
			checkGoStatement(irNode.Kids[1], depth+1)
		case "list_get":
			listNode := irNode.Kids[0]
			if listNode.Type != "SYMBOL" {
				ast.ReportError("list_get requires a symbol for list", listNode.Line, listNode.Column)
			}
			checkGoStatement(irNode.Kids[1], depth+1)
		default:
			for _, kid := range irNode.Kids {
				checkGoStatement(kid, depth+1)
			}
			for _, c := range irNode.Cases {
				checkGoStatement(c.Body, depth+1)
			}
		}
		return
	}

	if node.Type == "List" && len(node.Children) > 0 {
		head := node.Children[0].Value
		switch head {
		case "let":
			curr := &ast.Node{Type: "List", Children: []*ast.Node{{Type: "SYMBOL", Value: "let"}, node.Children[1], node.Children[2]}}
			for curr.Type == "List" && len(curr.Children) == 3 && curr.Children[0].Value == "let" {
				binds := curr.Children[1]
				valNode := binds.Children[1]
				checkGoStatement(valNode, depth+1)
				curr = curr.Children[2]
			}
			checkGoStatement(curr, depth+1)
		case "try_let":
			binds := node.Children[1]
			valNode := binds.Children[1]
			checkGoStatement(valNode, depth+1)
			catchNode := node.Children[2]
			checkGoStatement(catchNode.Children[2], depth+1)
			successBodyCode := node.Children[3]
			checkGoStatement(successBodyCode, depth+1)
		case "spawn":
			lambdaNode := node.Children[1]
			checkGoStatement(lambdaNode.Children[2], depth+1)
		case "call":
			for j := 2; j < len(node.Children); j++ {
				checkGoStatement(node.Children[j], depth+1)
			}
		case "for":
			checkGoStatement(node.Children[3], depth+1)
		case "do":
			for j := 1; j < len(node.Children); j++ {
				checkGoStatement(node.Children[j], depth+1)
			}
		default:
			for j := 1; j < len(node.Children); j++ {
				checkGoStatement(node.Children[j], depth+1)
			}
		}
	}
}
