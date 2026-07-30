import re

with open("internal/ir/ir.go", "r") as f:
    content = f.read()

new_cases = """
	case "let":
		if len(node.Children) != 3 {
			ast.ReportError("let expects (let (var val) body) — wrap multiple body statements in (do ...)", node.Line, node.Column)
		}
		binds := node.Children[1]
		if binds.Type != "List" || len(binds.Children) != 2 {
			ast.ReportError("let binding expects (var val)", binds.Line, binds.Column)
		}
		return &IRNode{Kind: "let", Kids: node.Children[1:]}, true
	case "try_let":
		if len(node.Children) != 4 {
			ast.ReportError("try_let expects (try_let (var val) (catch err catchBody) successBody)", node.Line, node.Column)
		}
		binds := node.Children[1]
		if binds.Type != "List" || len(binds.Children) != 2 {
			ast.ReportError("try_let binding expects (var val)", binds.Line, binds.Column)
		}
		catchNode := node.Children[2]
		if catchNode.Type != "List" || len(catchNode.Children) != 3 || catchNode.Children[0].Value != "catch" {
			ast.ReportError("try_let catch expects (catch errVar catchBody)", catchNode.Line, catchNode.Column)
		}
		return &IRNode{Kind: "try_let", Kids: node.Children[1:]}, true
	case "call":
		if len(node.Children) < 2 {
			ast.ReportError("call expects (call func args...)", node.Line, node.Column)
		}
		return &IRNode{Kind: "call", Kids: node.Children[1:]}, true
	case "for":
		if len(node.Children) != 4 {
			ast.ReportError("for expects (for item list body)", node.Line, node.Column)
		}
		return &IRNode{Kind: "for", Kids: node.Children[1:]}, true
	case "spawn":
		if len(node.Children) != 2 {
			ast.ReportError("spawn expects (spawn (lambda () body))", node.Line, node.Column)
		}
		lambdaNode := node.Children[1]
		if lambdaNode.Type != "List" || len(lambdaNode.Children) != 3 || lambdaNode.Children[0].Value != "lambda" {
			ast.ReportError("spawn expects a lambda", lambdaNode.Line, lambdaNode.Column)
		}
		argsNode := lambdaNode.Children[1]
		if argsNode.Type != "List" || len(argsNode.Children) != 0 {
			ast.ReportError("spawn lambda expects no arguments ()", argsNode.Line, argsNode.Column)
		}
		return &IRNode{Kind: "spawn", Kids: node.Children[1:]}, true
	case "return":
"""

content = content.replace('\tcase "return":\n', new_cases)

with open("internal/ir/ir.go", "w") as f:
    f.write(content)
