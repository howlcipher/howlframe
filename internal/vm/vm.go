package vm

import (
	"bufio"
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
	"sync"
	"time"
	"zero/internal/ast"
	"zero/internal/bytecode"
	"zero/internal/capability"
	"zero/internal/ir"
	"zero/internal/lexer"
	"zero/internal/parser"
)

// interpreter.go implements Phase 1 of improvement #49 (Direct Neural
// Bytecode Synthesis): a tree-walking interpreter that executes a cli_app
// AST directly, with no Go/JS text ever generated and no go build/go run/
// node subprocess ever invoked. See docs/direct_execution_design.md for the
// full design, covered node subset, and documented deviations from the Go
// backend.

// returnSignal unwinds a (return val) out of arbitrarily nested if/while/do
// blocks via panic/recover, mirroring Go's own return semantics.
type returnSignal struct{ value any }

type InterpEnv struct {
	vars   map[string]any
	parent *InterpEnv
}

func NewInterpEnv(parent *InterpEnv) *InterpEnv {
	return &InterpEnv{vars: make(map[string]any), parent: parent}
}

func (e *InterpEnv) get(name string) (any, bool) {
	for env := e; env != nil; env = env.parent {
		if v, ok := env.vars[name]; ok {
			return v, true
		}
	}
	return nil, false
}

func (e *InterpEnv) set(name string, val any) bool {
	for env := e; env != nil; env = env.parent {
		if _, ok := env.vars[name]; ok {
			env.vars[name] = val
			return true
		}
	}
	return false
}

type InterpFunc struct {
	params         []string
	body           *ast.Node
	lazySynthesize bool
	docstring      string
	name           string
}

// Interpreter holds the global function table for a single -run invocation.
// defun bodies have no closure over caller scope, matching the Go backend's
// model where defun compiles to an independent top-level function.
type Interpreter struct {
	funcs      map[string]*InterpFunc
	args       []string
	In         io.Reader
	Out        io.Writer
	ErrOut     io.Writer
	lineReader *bufio.Reader
}

type VmExit struct {
	code int
}

func InterpErr(reason string, node *ast.Node) {
	line, col := 0, 0
	if node != nil {
		line, col = node.Line, node.Column
	}
	ast.ReportError(reason, line, col)
}

// Interpret executes a cli_app AST directly and returns a process exit code.
// http_server/web_app roots are rejected with a clear error — Phase 1 is
// cli_app only, per docs/direct_execution_design.md.
func Interpret(ast *ast.Node, args []string, in io.Reader, out io.Writer, errOut io.Writer) (exitCode int) {
	if ast == nil || ast.Type != "List" || len(ast.Children) == 0 || ast.Children[0].Type != "SYMBOL" {
		InterpErr("Expected cli_app as root symbol", ast)
	}
	root := ast.Children[0].Value
	if root != "cli_app" {
		InterpErr(fmt.Sprintf("-run only supports cli_app in Phase 1 (see docs/direct_execution_design.md); got %q", root), ast.Children[0])
	}

	interp := &Interpreter{
		funcs:  make(map[string]*InterpFunc),
		args:   args,
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}
	if interp.In == nil {
		interp.In = os.Stdin
	}
	if interp.Out == nil {
		interp.Out = os.Stdout
	}
	if interp.ErrOut == nil {
		interp.ErrOut = os.Stderr
	}
	interp.lineReader = bufio.NewReader(interp.In)
	globalEnv := NewInterpEnv(nil)

	for _, child := range ast.Children[1:] {
		if IsDefun(child) {
			interp.registerDefun(child)
		}
	}

	func() {
		defer func() {
			if r := recover(); r != nil {
				if exit, ok := r.(VmExit); ok {
					exitCode = exit.code
					return
				}
				if returnSig, ok := r.(returnSignal); ok {
					if returnSig.value != nil {
						if num, ok := returnSig.value.(int64); ok {
							exitCode = int(num)
						}
					}
					return
				}
				panic(r)
			}
		}()
		for _, child := range ast.Children[1:] {
			if !IsDefun(child) {
				interp.eval(child, globalEnv)
			}
		}
	}()
	return exitCode
}

func IsDefun(node *ast.Node) bool {
	return node.Type == "List" && len(node.Children) > 0 &&
		node.Children[0].Type == "SYMBOL" && (node.Children[0].Value == "defun" || node.Children[0].Value == "lazy_synthesize")
}

func (interp *Interpreter) registerDefun(node *ast.Node) {
	if len(node.Children) < 4 {
		InterpErr("defun/lazy_synthesize expects (defun name (args) body) or (lazy_synthesize name (args) docstring)", node)
	}
	head := node.Children[0].Value
	name := node.Children[1].Value
	argsNode := node.Children[2]
	var params []string
	for _, arg := range argsNode.Children {
		if arg.Type == "List" && len(arg.Children) >= 1 {
			params = append(params, arg.Children[0].Value)
		} else {
			params = append(params, arg.Value)
		}
	}
	if head == "lazy_synthesize" {
		docstring := node.Children[3].Value
		interp.funcs[name] = &InterpFunc{params: params, lazySynthesize: true, docstring: docstring, name: name}
	} else {
		body := node.Children[len(node.Children)-1]
		interp.funcs[name] = &InterpFunc{params: params, body: body}
	}
}

func (interp *Interpreter) eval(node *ast.Node, env *InterpEnv) any {
	switch node.Type {
	case "STRING":
		return node.Value
	case "INT":
		v, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			InterpErr(fmt.Sprintf("invalid integer literal: %s", node.Value), node)
		}
		return v
	case "FLOAT":
		v, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			InterpErr(fmt.Sprintf("invalid float literal: %s", node.Value), node)
		}
		return v
	case "SYMBOL":
		if v, ok := env.get(node.Value); ok {
			return v
		}
		InterpErr(fmt.Sprintf("undefined variable: %s", node.Value), node)
	case "List":
		return interp.evalList(node, env)
	}
	InterpErr(fmt.Sprintf("cannot evaluate node of type %s", node.Type), node)
	return nil
}

func (interp *Interpreter) evalList(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) == 0 {
		InterpErr("empty expression", node)
	}
	head := node.Children[0].Value

	if ir.BinOpKinds[head] {
		return interp.evalBinop(head, node, env)
	}

	switch head {
	case "intent":
		return nil
	case "schema_bridge":
		if len(node.Children) != 3 {
			InterpErr("schema_bridge expects (schema_bridge StructName source)", node)
		}
		return interp.eval(node.Children[2], env)
	case "optimize_signature":
		if len(node.Children) < 6 {
			InterpErr("optimize_signature expects a name, metric, one or more tests, one or more candidates, and a body", node)
		}
		return interp.eval(node.Children[len(node.Children)-1], env)
	case "let":
		return interp.evalLet(node, env)
	case "set":
		return interp.evalSet(node, env)
	case "if":
		return interp.evalIf(node, env)
	case "while":
		return interp.evalWhile(node, env)
	case "for":
		return interp.evalFor(node, env)
	case "optimize_block":
		if len(node.Children) < 4 {
			InterpErr("optimize_block expects (optimize_block \"metric_name\" threshold_ms body...)", node)
		}
		var result any
		for _, kid := range node.Children[3:] {
			result = interp.eval(kid, env)
		}
		return result
	case "do":
		var result any
		for _, kid := range node.Children[1:] {
			result = interp.eval(kid, env)
		}
		return result
	case "print":
		var args []any
		for _, kid := range node.Children[1:] {
			args = append(args, interp.eval(kid, env))
		}
		var out []string
		for _, arg := range args {
			out = append(out, fmt.Sprint(arg))
		}
		fmt.Fprintln(interp.Out, strings.Join(out, " "))
		return nil
	case "stderr":
		val := interp.eval(node.Children[1], env)
		fmt.Fprint(interp.ErrOut, val)
		return nil
	case "exit":
		val := interp.eval(node.Children[1], env)
		code := int(val.(int64))
		panic(VmExit{code: code})
	case "read_line":
		line, err := interp.lineReader.ReadString('\n')
		if err != nil && err != io.EOF {
			InterpErr(fmt.Sprintf("Failed to read line: %v", err), node)
		}
		if err == io.EOF && line == "" {
			return ""
		}
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")
		return line
	case "return":
		if len(node.Children) != 2 {
			InterpErr("return expects (return val)", node)
		}
		panic(returnSignal{value: interp.eval(node.Children[1], env)})
	case "call":
		return interp.evalCall(node, env)
	case "neural_circuit":
		if len(node.Children) < 3 {
			InterpErr("neural_circuit expects (neural_circuit (args...) \"instruction\")", node)
		}
		argsNode := node.Children[1]
		instructionStr := fmt.Sprint(interp.eval(node.Children[2], env))
		var argVals []any
		for _, arg := range argsNode.Children {
			argVals = append(argVals, interp.eval(arg, env))
		}

		prompt := fmt.Sprintf("Instruction: %s", instructionStr)
		if len(argVals) > 0 {
			prompt += fmt.Sprintf("\nInputs: %v", argVals)
		}

		reqBody, _ := json.Marshal(map[string]any{
			"model":  "llama3",
			"prompt": prompt,
			"stream": false,
		})
		resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			panic(err)
		}
		return res.Response
	case "ephemeral_circuit":
		if len(node.Children) < 3 {
			InterpErr("ephemeral_circuit expects (ephemeral_circuit (args...) \"instruction\")", node)
		}
		argsNode := node.Children[1]
		instructionStr := fmt.Sprint(interp.eval(node.Children[2], env))
		var argVals []any
		for _, arg := range argsNode.Children {
			argVals = append(argVals, interp.eval(arg, env))
		}

		prompt := ""
		if len(argVals) > 0 {
			prompt = fmt.Sprintf("Inputs: %v", argVals)
		}

		modelName := fmt.Sprintf("ephemeral-%d", time.Now().UnixNano())
		modelfile := fmt.Sprintf("FROM llama3\nSYSTEM You are a highly specialized reasoning circuit. Your task is: %s", instructionStr)

		createReq, _ := json.Marshal(map[string]any{
			"name":      modelName,
			"modelfile": modelfile,
			"stream":    false,
		})
		createResp, err := http.Post("http://localhost:11434/api/create", "application/json", bytes.NewReader(createReq))
		if err != nil {
			panic(err)
		}
		createResp.Body.Close()

		defer func() {
			delReq, _ := json.Marshal(map[string]any{"name": modelName})
			req, _ := http.NewRequest("DELETE", "http://localhost:11434/api/delete", bytes.NewReader(delReq))
			req.Header.Set("Content-Type", "application/json")
			client := &http.Client{}
			resp, _ := client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		}()

		reqBody, _ := json.Marshal(map[string]any{
			"model":  modelName,
			"prompt": prompt,
			"stream": false,
		})
		resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			panic(err)
		}
		return res.Response
	case "achieve":
		if len(node.Children) != 3 {
			InterpErr("achieve expects (achieve target constraint)", node)
		}
		targetStr := ast.Stringify(node.Children[1])
		constraintStr := ast.Stringify(node.Children[2])
		reqBody, _ := json.Marshal(map[string]any{
			"model":  "llama3",
			"prompt": "Achieve the following target: " + targetStr + " with constraint: " + constraintStr + ". Return ONLY the result, no explanations.",
			"stream": false,
		})
		resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			panic(err)
		}
		return res.Response
	case "confidence":
		if len(node.Children) != 2 {
			InterpErr("confidence expects (confidence prompt)", node)
		}
		promptStr := fmt.Sprintf("%v", interp.eval(node.Children[1], env))
		reqBody, _ := json.Marshal(map[string]any{
			"model":  "llama3",
			"prompt": "Evaluate the probability of this statement being true. Return ONLY a float between 0.0 and 1.0. Statement: " + promptStr,
			"stream": false,
		})
		resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			panic(err)
		}
		val, _ := strconv.ParseFloat(strings.TrimSpace(res.Response), 64)
		return val
	case "list":
		var items []any
		for _, kid := range node.Children[1:] {
			items = append(items, interp.eval(kid, env))
		}
		return items
	case "dict":
		d := make(map[string]any)
		for _, kid := range node.Children[1:] {
			if kid.Type != "List" || len(kid.Children) != 2 {
				InterpErr("dict expects (k v) pairs", kid)
			}
			k := fmt.Sprint(interp.eval(kid.Children[0], env))
			v := interp.eval(kid.Children[1], env)
			d[k] = v
		}
		return d
	case "append":
		return interp.evalAppend(node, env)
	case "map_set":
		return interp.evalMapSet(node, env)
	case "map_delete":
		return interp.evalMapDelete(node, env)
	case "map_get":
		return interp.evalMapGet(node, env)
	case "list_get":
		return interp.evalListGet(node, env)
	case "to_int":
		if len(node.Children) != 2 {
			InterpErr("to_int expects (to_int val)", node)
		}
		v, _ := strconv.ParseInt(strings.TrimSpace(fmt.Sprint(interp.eval(node.Children[1], env))), 10, 64)
		return v
	case "to_float":
		if len(node.Children) != 2 {
			InterpErr("to_float expects (to_float val)", node)
		}
		v, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(interp.eval(node.Children[1], env))), 64)
		return v
	case "to_string":
		if len(node.Children) != 2 {
			InterpErr("to_string expects (to_string val)", node)
		}
		return fmt.Sprint(interp.eval(node.Children[1], env))
	case "bytes_to_string":
		if len(node.Children) != 2 {
			InterpErr("bytes_to_string expects (bytes_to_string val)", node)
		}
		val := interp.eval(node.Children[1], env)
		if b, ok := val.([]byte); ok {
			return string(b)
		}
		return fmt.Sprint(val)
	case "str_split":
		if len(node.Children) != 3 {
			InterpErr("str_split expects (str_split s sep)", node)
		}
		s := fmt.Sprint(interp.eval(node.Children[1], env))
		sep := fmt.Sprint(interp.eval(node.Children[2], env))
		parts := strings.Split(s, sep)
		items := make([]any, len(parts))
		for i, p := range parts {
			items[i] = p
		}
		return items
	case "str_join":
		if len(node.Children) != 3 {
			InterpErr("str_join expects (str_join list sep)", node)
		}
		listVal := interp.eval(node.Children[1], env)
		sep := fmt.Sprint(interp.eval(node.Children[2], env))
		items, ok := listVal.([]any)
		if !ok {
			InterpErr("str_join expects a list", node.Children[1])
		}
		strs := make([]string, len(items))
		for i, it := range items {
			strs[i] = fmt.Sprint(it)
		}
		return strings.Join(strs, sep)
	case "regex_match":
		if len(node.Children) != 3 {
			InterpErr("regex_match expects (regex_match pattern s)", node)
		}
		pat := fmt.Sprint(interp.eval(node.Children[1], env))
		s := fmt.Sprint(interp.eval(node.Children[2], env))
		matched, err := regexp.MatchString(pat, s)
		if err != nil {
			InterpErr(fmt.Sprintf("invalid regex: %v", err), node)
		}
		return matched
	case "cli_args":
		if len(node.Children) == 1 {
			return SliceToAny(interp.args)
		} else if len(node.Children) == 2 {
			idx, err := ToInt(interp.eval(node.Children[1], env))
			if err != nil {
				InterpErr("cli_args index must be a number", node)
			}
			if int(idx) >= 0 && int(idx) < len(interp.args) {
				return interp.args[idx]
			}
			return ""
		}
		InterpErr("cli_args expects (cli_args) or (cli_args index)", node)
	case "sleep":
		if len(node.Children) != 2 {
			InterpErr("sleep expects (sleep ms)", node)
		}
		ms, err := ToInt(interp.eval(node.Children[1], env))
		if err != nil {
			InterpErr("sleep expects a numeric argument", node)
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return nil
	case "env":
		if len(node.Children) != 2 {
			InterpErr("env expects (env \"KEY\")", node)
		}
		key := fmt.Sprint(interp.eval(node.Children[1], env))
		return os.Getenv(key)
	}

	InterpErr(fmt.Sprintf("%q is not supported under -run in Phase 1 (see docs/direct_execution_design.md)", head), node.Children[0])
	return nil
}

func (interp *Interpreter) evalLet(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 3 {
		InterpErr("let expects (let (var val) body) — wrap multiple body statements in (do ...)", node)
	}
	binding := node.Children[1]
	if binding.Type != "List" || len(binding.Children) != 2 {
		InterpErr("let binding expects (var val)", binding)
	}
	varName := binding.Children[0].Value
	val := interp.eval(binding.Children[1], env)
	childEnv := NewInterpEnv(env)
	childEnv.vars[varName] = val
	return interp.eval(node.Children[2], childEnv)
}

func (interp *Interpreter) evalSet(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 3 {
		InterpErr("set expects (set var val)", node)
	}
	varName := node.Children[1].Value
	val := interp.eval(node.Children[2], env)
	if !env.set(varName, val) {
		InterpErr(fmt.Sprintf("undefined variable: %s", varName), node.Children[1])
	}
	return nil
}

func (interp *Interpreter) evalIf(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 3 && len(node.Children) != 4 {
		InterpErr("if expects (if cond then [else])", node)
	}
	if ToBool(interp.eval(node.Children[1], env), node.Children[1]) {
		return interp.eval(node.Children[2], env)
	} else if len(node.Children) == 4 {
		return interp.eval(node.Children[3], env)
	}
	return nil
}

func (interp *Interpreter) evalWhile(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 3 {
		InterpErr("while expects (while cond body)", node)
	}
	for ToBool(interp.eval(node.Children[1], env), node.Children[1]) {
		interp.eval(node.Children[2], env)
	}
	return nil
}

func (interp *Interpreter) evalFor(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 4 {
		InterpErr("for expects (for item list body)", node)
	}
	itemName := node.Children[1].Value
	listVal := interp.eval(node.Children[2], env)
	items, ok := listVal.([]any)
	if !ok {
		InterpErr("for requires a list value to iterate", node.Children[2])
	}
	for _, item := range items {
		childEnv := NewInterpEnv(env)
		childEnv.vars[itemName] = item
		interp.eval(node.Children[3], childEnv)
	}
	return nil
}

func (interp *Interpreter) evalCall(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) < 2 {
		InterpErr("call expects (call func args...)", node)
	}
	funcName := node.Children[1].Value
	fn, ok := interp.funcs[funcName]
	if !ok {
		InterpErr(fmt.Sprintf("%q is not a defined function (only user defun functions are callable under -run in Phase 1)", funcName), node.Children[1])
	}

	if fn.lazySynthesize {
		promptStr := fmt.Sprintf("You are a Zero compiler. Synthesize the Zero Lisp code for the function '%s' with parameters %v. Docstring: \"%s\"\n\nReply ONLY with the Zero Lisp code for the function body expressions. Do not include (defun ...). Do not include markdown formatting.\nFor example, if the docstring says \"Returns the sum of a and b\", you reply:\n(+ a b)", fn.name, fn.params, fn.docstring)
		reqBody, _ := json.Marshal(map[string]any{
			"model":  "llama3",
			"prompt": promptStr,
			"stream": false,
		})
		resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			panic(err)
		}
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			resp.Body.Close()
			panic(err)
		}
		resp.Body.Close()
		code := strings.TrimSpace(res.Response)

		code = strings.TrimPrefix(code, "```lisp")
		code = strings.TrimPrefix(code, "```")
		code = strings.TrimSuffix(code, "```")
		code = strings.TrimSpace(code)
		progCode := fmt.Sprintf("(defun %s (%s) %s)", fn.name, strings.Join(fn.params, " "), code)
		lx := lexer.NewLexer(progCode)
		p := parser.NewParser(lx, "lazy_synthesize")
		astNode := p.ParseExpression()

		fn.body = astNode.Children[len(astNode.Children)-1]
		fn.lazySynthesize = false
	}
	var argVals []any
	for _, argNode := range node.Children[2:] {
		argVals = append(argVals, interp.eval(argNode, env))
	}
	if len(argVals) != len(fn.params) {
		InterpErr(fmt.Sprintf("%s expects %d argument(s), got %d", funcName, len(fn.params), len(argVals)), node)
	}
	callEnv := NewInterpEnv(nil)
	for i, p := range fn.params {
		callEnv.vars[p] = argVals[i]
	}
	var result any
	func() {
		defer func() {
			if r := recover(); r != nil {
				if rs, ok := r.(returnSignal); ok {
					result = rs.value
					return
				}
				panic(r)
			}
		}()
		result = interp.eval(fn.body, callEnv)
	}()
	return result
}

func (interp *Interpreter) evalAppend(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 3 {
		InterpErr("append expects (append list item)", node)
	}
	listNode := node.Children[1]
	if listNode.Type != "SYMBOL" {
		InterpErr("append requires a symbol for list", listNode)
	}
	current, ok := env.get(listNode.Value)
	if !ok {
		InterpErr(fmt.Sprintf("undefined variable: %s", listNode.Value), listNode)
	}
	items, ok := current.([]any)
	if !ok {
		InterpErr(fmt.Sprintf("append target %q is not a list", listNode.Value), listNode)
	}
	item := interp.eval(node.Children[2], env)
	newItems := append(append([]any{}, items...), item)
	if !env.set(listNode.Value, newItems) {
		InterpErr(fmt.Sprintf("undefined variable: %s", listNode.Value), listNode)
	}
	return nil
}

func (interp *Interpreter) evalMapSet(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 4 {
		InterpErr("map_set expects (map_set dict key val)", node)
	}
	dictNode := node.Children[1]
	if dictNode.Type != "SYMBOL" {
		InterpErr("map_set requires a symbol for dict", dictNode)
	}
	current, ok := env.get(dictNode.Value)
	if !ok {
		InterpErr(fmt.Sprintf("undefined variable: %s", dictNode.Value), dictNode)
	}
	d, ok := current.(map[string]any)
	if !ok {
		InterpErr(fmt.Sprintf("map_set target %q is not a dict", dictNode.Value), dictNode)
	}
	key := fmt.Sprint(interp.eval(node.Children[2], env))
	val := interp.eval(node.Children[3], env)
	d[key] = val
	return nil
}

func (interp *Interpreter) evalMapDelete(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 3 {
		InterpErr("map_delete expects (map_delete dict key)", node)
	}
	dictNode := node.Children[1]
	if dictNode.Type != "SYMBOL" {
		InterpErr("map_delete requires a symbol for dict", dictNode)
	}
	current, ok := env.get(dictNode.Value)
	if !ok {
		InterpErr(fmt.Sprintf("undefined variable: %s", dictNode.Value), dictNode)
	}
	d, ok := current.(map[string]any)
	if !ok {
		InterpErr(fmt.Sprintf("map_delete target %q is not a dict", dictNode.Value), dictNode)
	}
	key := fmt.Sprint(interp.eval(node.Children[2], env))
	delete(d, key)
	return nil
}

func (interp *Interpreter) evalMapGet(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 3 {
		InterpErr("map_get expects (map_get dict key)", node)
	}
	dictNode := node.Children[1]
	if dictNode.Type != "SYMBOL" {
		InterpErr("map_get requires a symbol for dict", dictNode)
	}
	current, ok := env.get(dictNode.Value)
	if !ok {
		InterpErr(fmt.Sprintf("undefined variable: %s", dictNode.Value), dictNode)
	}
	d, ok := current.(map[string]any)
	if !ok {
		InterpErr(fmt.Sprintf("map_get target %q is not a dict", dictNode.Value), dictNode)
	}
	key := fmt.Sprint(interp.eval(node.Children[2], env))
	if val, ok := d[key]; ok {
		return val
	}
	return ""
}

func (interp *Interpreter) evalListGet(node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 3 {
		InterpErr("list_get expects (list_get list idx)", node)
	}
	listNode := node.Children[1]
	if listNode.Type != "SYMBOL" {
		InterpErr("list_get requires a symbol for list", listNode)
	}
	current, ok := env.get(listNode.Value)
	if !ok {
		InterpErr(fmt.Sprintf("undefined variable: %s", listNode.Value), listNode)
	}
	items, ok := current.([]any)
	if !ok {
		InterpErr(fmt.Sprintf("list_get target %q is not a list", listNode.Value), listNode)
	}
	idxVal := interp.eval(node.Children[2], env)
	idx, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(idxVal)))
	if err != nil {
		InterpErr("list_get index must be a number", node.Children[2])
	}
	if idx < 0 || idx >= len(items) {
		return ""
	}
	return items[idx]
}

func (interp *Interpreter) evalBinop(op string, node *ast.Node, env *InterpEnv) any {
	if len(node.Children) != 3 {
		InterpErr(fmt.Sprintf("%s expects 2 arguments", op), node)
	}
	a := interp.eval(node.Children[1], env)
	b := interp.eval(node.Children[2], env)
	switch op {
	case "and":
		return ToBool(a, node.Children[1]) && ToBool(b, node.Children[2])
	case "or":
		return ToBool(a, node.Children[1]) || ToBool(b, node.Children[2])
	case "==", "=":
		return ValuesEqual(a, b)
	case "!=":
		return !ValuesEqual(a, b)
	case "+":
		if as, ok := a.(string); ok {
			if bs, ok2 := b.(string); ok2 {
				return as + bs
			}
		}
		return NumericBinop(op, a, b, node)
	default: // - * / < > <= >=
		return NumericBinop(op, a, b, node)
	}
}

func NumericBinop(op string, a, b any, node *ast.Node) any {
	ai, aIsInt := a.(int64)
	bi, bIsInt := b.(int64)
	if aIsInt && bIsInt {
		switch op {
		case "+":
			return ai + bi
		case "-":
			return ai - bi
		case "*":
			return ai * bi
		case "/":
			if bi == 0 {
				InterpErr("division by zero", node)
			}
			return ai / bi
		case "<":
			return ai < bi
		case ">":
			return ai > bi
		case "<=":
			return ai <= bi
		case ">=":
			return ai >= bi
		}
	}

	af, aOk := ToFloat(a)
	bf, bOk := ToFloat(b)
	if !aOk || !bOk {
		InterpErr(fmt.Sprintf("%s requires numeric operands, got %T and %T", op, a, b), node)
	}
	switch op {
	case "+":
		return af + bf
	case "-":
		return af - bf
	case "*":
		return af * bf
	case "/":
		if bf == 0 {
			InterpErr("division by zero", node)
		}
		return af / bf
	case "<":
		return af < bf
	case ">":
		return af > bf
	case "<=":
		return af <= bf
	case ">=":
		return af >= bf
	}
	return nil
}

func ToFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int64:
		return float64(t), true
	case float64:
		return t, true
	}
	return 0, false
}

func ToInt(v any) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case float64:
		return int64(t), nil
	case string:
		return strconv.ParseInt(t, 10, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to int", v)
}

func ToBool(v any, node *ast.Node) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	InterpErr(fmt.Sprintf("expected boolean, got %T", v), node)
	return false
}

func ValuesEqual(a, b any) bool {
	switch a.(type) {
	case []any, map[string]any:
		return false
	}
	switch b.(type) {
	case []any, map[string]any:
		return false
	}
	return a == b
}

func SliceToAny(strs []string) []any {
	out := make([]any, len(strs))
	for i, s := range strs {
		out[i] = s
	}
	return out
}

type BCVM struct {
	prog        *bytecode.BCProgram
	stack       []any
	env         *BcEnv
	stores      *bcStoreRegistry
	ip          int
	insts       []bytecode.BCInstruction
	args        []string
	executed    int
	Limits      VMLimits
	AllowedCaps []capability.Capability
	In          io.Reader
	Out         io.Writer
	ErrOut      io.Writer
	lineReader  *bufio.Reader
}

type bcStoreRegistry struct {
	mu     sync.Mutex
	stores map[string]*bcMemoryStore
}

type bcMemoryStore struct {
	mu      sync.RWMutex
	records map[string]map[string]any
}

func newBCStoreRegistry() *bcStoreRegistry {
	return &bcStoreRegistry{stores: make(map[string]*bcMemoryStore)}
}

func (registry *bcStoreRegistry) open(uri string) *bcMemoryStore {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	store, ok := registry.stores[uri]
	if !ok {
		store = &bcMemoryStore{records: make(map[string]map[string]any)}
		registry.stores[uri] = store
	}
	return store
}

type BcEnv struct {
	vars   map[string]any
	parent *BcEnv
}

func NewBcEnv(parent *BcEnv) *BcEnv {
	return &BcEnv{vars: make(map[string]any), parent: parent}
}

func (e *BcEnv) get(name string) (any, bool) {
	if val, ok := e.vars[name]; ok {
		return val, true
	}
	if e.parent != nil {
		return e.parent.get(name)
	}
	return nil, false
}

func (e *BcEnv) set(name string, val any) bool {
	if _, ok := e.vars[name]; ok {
		e.vars[name] = val
		return true
	}
	if e.parent != nil {
		return e.parent.set(name, val)
	}
	return false // undefined
}

type VmReturn struct {
	val any
}

func RunBytecode(prog *bytecode.BCProgram, cliArgs []string, allowedCaps []capability.Capability, in io.Reader, out io.Writer, errOut io.Writer) (exitCode int) {
	return RunBytecodeWithPolicy(prog, cliArgs, DefaultExecutionPolicy(), allowedCaps, in, out, errOut)
}

// RunBytecodeWithPolicy executes bytecode under authority selected by the
// trusted runner. Capabilities remain an independent grant: increasing a
// resource limit does not authorize any external effect.
func RunBytecodeWithPolicy(prog *bytecode.BCProgram, cliArgs []string, policy ExecutionPolicy, allowedCaps []capability.Capability, in io.Reader, out io.Writer, errOut io.Writer) (exitCode int) {
	vm := &BCVM{
		prog:        prog,
		env:         NewBcEnv(nil),
		insts:       prog.Main,
		args:        cliArgs,
		stores:      newBCStoreRegistry(),
		Limits:      policy.Limits,
		AllowedCaps: allowedCaps,
		In:          in,
		Out:         out,
		ErrOut:      errOut,
	}
	if vm.In == nil {
		vm.In = os.Stdin
	}
	if vm.Out == nil {
		vm.Out = os.Stdout
	}
	if vm.ErrOut == nil {
		vm.ErrOut = os.Stderr
	}
	vm.lineReader = bufio.NewReader(vm.In)

	defer func() {
		if r := recover(); r != nil {
			if exit, ok := r.(VmExit); ok {
				exitCode = exit.code
				return
			}
			if vmerr, ok := r.(*VMError); ok {
				fmt.Fprintln(vm.ErrOut, vmerr.Error())
				os.Exit(1)
			}
			fmt.Fprintf(vm.ErrOut, "VM internal panic: %v\n", r)
			os.Exit(1)
		}
	}()

	vm.run(vm.insts, vm.env)
	return 0
}

func (vm *BCVM) push(v any) {
	vm.stack = append(vm.stack, v)
}

func (vm *BCVM) pop(inst bytecode.Opcode) any {
	if len(vm.stack) == 0 {
		panic(NewRuntimeError("STACK_UNDERFLOW", "main", vm.ip, inst, "stack underflow at %s", bytecode.Registry[inst].Name))
	}
	v := vm.stack[len(vm.stack)-1]
	vm.stack = vm.stack[:len(vm.stack)-1]
	return v
}

func (vm *BCVM) storeHandle(env *BcEnv, name string, op bytecode.Opcode) *bcMemoryStore {
	value, ok := env.get(name)
	if !ok {
		panic(NewRuntimeError(
			"UNDEFINED_STORE",
			"main",
			vm.ip,
			op,
			"undefined store handle: %s",
			name,
		))
	}
	store, ok := value.(*bcMemoryStore)
	if !ok {
		panic(NewRuntimeError(
			"INVALID_STORE_HANDLE",
			"main",
			vm.ip,
			op,
			"%s is not a store handle",
			name,
		))
	}
	return store
}

func cloneStoreRecord(record map[string]any) map[string]any {
	clone := make(map[string]any, len(record))
	for key, value := range record {
		clone[key] = cloneStoreValue(value)
	}
	return clone
}

func cloneStoreValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneStoreRecord(typed)
	case []any:
		clone := make([]any, len(typed))
		for index, item := range typed {
			clone[index] = cloneStoreValue(item)
		}
		return clone
	default:
		return value
	}
}

func (vm *BCVM) run(insts []bytecode.BCInstruction, env *BcEnv) any {
	if vm.stores == nil {
		vm.stores = newBCStoreRegistry()
	}

	ip := 0
	for ip < len(insts) {
		vm.ip = ip
		inst := insts[ip]
		// MaxInstructions is the maximum number of instructions allowed. Check
		// before incrementing so an exact budget succeeds and even MaxInt cannot
		// overflow into effectively unlimited execution.
		if vm.Limits.MaxInstructions <= 0 || vm.executed >= vm.Limits.MaxInstructions {
			panic(NewRuntimeError("LIMIT_EXCEEDED", "main", vm.ip, inst.Op, "instruction limit exceeded"))
		}
		vm.executed++

		spec := bytecode.Registry[inst.Op]
		if spec.Capability != capability.None {
			allowed := false
			for _, cap := range vm.AllowedCaps {
				if cap == spec.Capability {
					allowed = true
					break
				}
			}
			if !allowed {
				panic(NewRuntimeError("CAPABILITY_DENIED", "main", vm.ip, inst.Op, "capability denied: %s", spec.Capability))
			}
		}

		switch inst.Op {

		case bytecode.OpTryLet:
			varName := inst.StringOperand
			errVar := inst.StringOperand2
			valLen := int(inst.IntOperand)
			catchLen := int(inst.IntOperand2)
			successLen := int(inst.IntOperand3)

			valInsts := insts[ip+1 : ip+1+valLen]
			catchInsts := insts[ip+1+valLen : ip+1+valLen+catchLen]
			successInsts := insts[ip+1+valLen+catchLen : ip+1+valLen+catchLen+successLen]

			var tryErr error
			func() {
				defer func() {
					if r := recover(); r != nil {
						if _, ok := r.(VmReturn); ok {
							panic(r)
						}
						tryErr = fmt.Errorf("%v", r)
					}
				}()
				vm.run(valInsts, env)
			}()

			if tryErr != nil {
				env.vars[errVar] = tryErr.Error()
				vm.run(catchInsts, env)
			} else {
				env.vars[varName] = vm.pop(inst.Op)
				vm.run(successInsts, env)
			}
			ip += 1 + valLen + catchLen + successLen
			continue
		case bytecode.OpDbConnect:
			varName := inst.StringOperand
			driver := inst.StringOperand2
			dsn := inst.StringOperand3
			db, err := sql.Open(driver, dsn)
			if err != nil {
				panic(err)
			}
			env.vars[varName] = db
		case bytecode.OpSqlQuery:
			dbVar := inst.StringOperand
			queryStr := inst.StringOperand2
			dbAny, ok := env.get(dbVar)
			if !ok {
				panic("undefined db: " + dbVar)
			}
			db := dbAny.(*sql.DB)
			rows, err := db.Query(queryStr)
			if err != nil {
				panic(err)
			}
			var results []any
			cols, _ := rows.Columns()
			for rows.Next() {
				vals := make([]any, len(cols))
				valPtrs := make([]any, len(cols))
				for i := range vals {
					valPtrs[i] = &vals[i]
				}
				rows.Scan(valPtrs...)
				rowMap := make(map[string]any)
				for i, col := range cols {
					if b, ok := vals[i].([]byte); ok {
						rowMap[col] = string(b)
					} else {
						rowMap[col] = vals[i]
					}
				}
				results = append(results, rowMap)
			}
			vm.push(results)
		case bytecode.OpStoreOpen:
			uri := inst.StringOperand2
			if !strings.HasPrefix(uri, "memory://") || len(uri) == len("memory://") {
				panic(NewRuntimeError(
					"INVALID_STORE_URI",
					"main",
					vm.ip,
					inst.Op,
					"store URI must use memory:// with a non-empty name",
				))
			}
			env.vars[inst.StringOperand] = vm.stores.open(uri)
		case bytecode.OpStorePut:
			recordAny := vm.pop(inst.Op)
			key := fmt.Sprint(vm.pop(inst.Op))
			record, ok := recordAny.(map[string]any)
			if !ok {
				panic(NewRuntimeError(
					"INVALID_STORE_RECORD",
					"main",
					vm.ip,
					inst.Op,
					"store_put record must be a dict, got %T",
					recordAny,
				))
			}
			store := vm.storeHandle(env, inst.StringOperand, inst.Op)
			store.mu.Lock()
			store.records[key] = cloneStoreRecord(record)
			store.mu.Unlock()
		case bytecode.OpStoreGet:
			key := fmt.Sprint(vm.pop(inst.Op))
			store := vm.storeHandle(env, inst.StringOperand, inst.Op)
			store.mu.RLock()
			record, ok := store.records[key]
			if ok {
				record = cloneStoreRecord(record)
			}
			store.mu.RUnlock()
			if !ok {
				// Missing records have one stable sentinel instead of an
				// allocated empty dict that could be mistaken for stored data.
				vm.push(nil)
			} else {
				vm.push(record)
			}
		case bytecode.OpStoreDelete:
			key := fmt.Sprint(vm.pop(inst.Op))
			store := vm.storeHandle(env, inst.StringOperand, inst.Op)
			store.mu.Lock()
			delete(store.records, key)
			store.mu.Unlock()
		case bytecode.OpFetch:
			method := vm.pop(inst.Op).(string)
			urlStr := vm.pop(inst.Op).(string)
			req, err := http.NewRequest(method, urlStr, nil)
			if err != nil {
				panic(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			var bytesAny []any
			for _, bb := range b {
				bytesAny = append(bytesAny, float64(bb))
			}
			vm.push(bytesAny)
		case bytecode.OpReadFile:
			path := vm.pop(inst.Op).(string)
			b, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			var bytesAny []any
			for _, bb := range b {
				bytesAny = append(bytesAny, float64(bb))
			}
			vm.push(bytesAny)
		case bytecode.OpWriteFile:
			dataAny := vm.pop(inst.Op)
			path := vm.pop(inst.Op).(string)
			var data []byte
			if s, ok := dataAny.(string); ok {
				data = []byte(s)
			} else if b, ok := dataAny.([]any); ok {
				for _, bb := range b {
					data = append(data, byte(bb.(float64)))
				}
			}
			err := os.WriteFile(path, data, 0644)
			if err != nil {
				panic(err)
			}
		case bytecode.OpMkdir:
			path := vm.pop(inst.Op).(string)
			err := os.MkdirAll(path, 0755)
			if err != nil {
				panic(err)
			}
		case bytecode.OpExec:
			numArgs := int(inst.IntOperand)
			var args []string
			for i := 0; i < numArgs; i++ {
				args = append([]string{fmt.Sprint(vm.pop(inst.Op))}, args...)
			}
			cmdStr := fmt.Sprint(vm.pop(inst.Op))
			out, err := exec.Command(cmdStr, args...).CombinedOutput()
			if err != nil {
				panic(err)
			}
			var bytesAny []any
			for _, bb := range out {
				bytesAny = append(bytesAny, float64(bb))
			}
			vm.push(bytesAny)
		case bytecode.OpParseJson:
			bodyVar := inst.StringOperand
			var data []byte
			if bodyVar == "req.body" {
				reqAny, ok := env.get("req")
				if !ok {
					panic("undefined req")
				}
				req := reqAny.(*http.Request)
				data, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewBuffer(data))
			} else {
				bodyAny, ok := env.get(bodyVar)
				if !ok {
					panic("undefined var: " + bodyVar)
				}
				if s, ok := bodyAny.(string); ok {
					data = []byte(s)
				}
				if b, ok := bodyAny.([]any); ok {
					for _, bb := range b {
						data = append(data, byte(bb.(float64)))
					}
				}
			}
			var result any
			err := json.Unmarshal(data, &result)
			if err != nil {
				panic(err)
			}
			vm.push(result)
		case bytecode.OpSpawn:
			bodyLen := int(inst.IntOperand)
			bodyInsts := insts[ip+1 : ip+1+bodyLen]
			capturedEnv := NewBcEnv(nil)
			for e := env; e != nil; e = e.parent {
				for k, v := range e.vars {
					if _, exists := capturedEnv.vars[k]; !exists {
						capturedEnv.vars[k] = v
					}
				}
			}
			go func(cEnv *BcEnv) {
				defer func() { recover() }()
				childVM := &BCVM{prog: vm.prog, env: cEnv, stores: vm.stores, Limits: vm.Limits}
				childVM.run(bodyInsts, cEnv)
			}(capturedEnv)
			ip += bodyLen
			continue
		case bytecode.OpRes:
			bodyAny := vm.pop(inst.Op)
			contentType := vm.pop(inst.Op).(string)
			status := int(ToBCFloat(vm.pop(inst.Op)))
			wAny, ok := env.get("w")
			if !ok {
				panic("no response writer")
			}
			w := wAny.(http.ResponseWriter)
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(status)
			if s, ok := bodyAny.(string); ok {
				w.Write([]byte(s))
			} else if b, ok := bodyAny.([]any); ok {
				var data []byte
				for _, bb := range b {
					data = append(data, byte(bb.(float64)))
				}
				w.Write(data)
			}
		case bytecode.OpResJson:
			data := vm.pop(inst.Op)
			status := int(ToBCFloat(vm.pop(inst.Op)))
			wAny, ok := env.get("w")
			if !ok {
				panic("no response writer")
			}
			w := wAny.(http.ResponseWriter)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(data)
		case bytecode.OpHttpServerStart:
			port := inst.StringOperand
			env.vars["__http_mux"] = http.NewServeMux()
			env.vars["__http_port"] = port
		case bytecode.OpHttpRoute:
			path := inst.StringOperand
			reqVar := inst.StringOperand2
			bodyLen := int(inst.IntOperand)
			bodyInsts := insts[ip+1 : ip+1+bodyLen]

			muxAny, _ := env.get("__http_mux")
			mux := muxAny.(*http.ServeMux)

			capturedEnv := NewBcEnv(nil)
			for e := env; e != nil; e = e.parent {
				for k, v := range e.vars {
					if _, exists := capturedEnv.vars[k]; !exists {
						capturedEnv.vars[k] = v
					}
				}
			}

			prog := vm.prog
			mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				reqEnv := NewBcEnv(capturedEnv)
				reqEnv.vars["w"] = w
				reqEnv.vars[reqVar] = r
				reqEnv.vars["req"] = r
				childVM := &BCVM{prog: prog, env: reqEnv, stores: vm.stores, Limits: vm.Limits}
				func() {
					defer func() { recover() }()
					childVM.run(bodyInsts, reqEnv)
				}()
			})
			ip += bodyLen
			continue
		case bytecode.OpHttpServerServe:
			muxAny, _ := env.get("__http_mux")
			mux := muxAny.(*http.ServeMux)
			portAny, _ := env.get("__http_port")
			port := portAny.(string)
			fmt.Println("Listening on " + port)
			err := http.ListenAndServe(port, mux)
			if err != nil {
				panic(err)
			}
		case bytecode.OpNeuralCircuit:
			numInputs := int(inst.IntOperand)
			inputs := make([]any, numInputs)
			for i := numInputs - 1; i >= 0; i-- {
				inputs[i] = vm.pop(inst.Op)
			}
			instructionAny := vm.pop(inst.Op)
			instructionStr := fmt.Sprintf("%v", instructionAny)

			prompt := fmt.Sprintf("Instruction: %s", instructionStr)
			if numInputs > 0 {
				prompt += fmt.Sprintf("\nInputs: %v", inputs)
			}

			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": prompt,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				panic(err)
			}
			vm.push(res.Response)

		case bytecode.OpEphemeralCircuit:
			numInputs := int(inst.IntOperand)
			inputs := make([]any, numInputs)
			for i := numInputs - 1; i >= 0; i-- {
				inputs[i] = vm.pop(inst.Op)
			}
			instructionAny := vm.pop(inst.Op)
			instructionStr := fmt.Sprintf("%v", instructionAny)

			prompt := ""
			if numInputs > 0 {
				prompt = fmt.Sprintf("Inputs: %v", inputs)
			}

			modelName := fmt.Sprintf("ephemeral-%d", time.Now().UnixNano())
			modelfile := fmt.Sprintf("FROM llama3\nSYSTEM You are a highly specialized reasoning circuit. Your task is: %s", instructionStr)

			createReq, _ := json.Marshal(map[string]any{
				"name":      modelName,
				"modelfile": modelfile,
				"stream":    false,
			})
			createResp, err := http.Post("http://localhost:11434/api/create", "application/json", bytes.NewReader(createReq))
			if err != nil {
				panic(err)
			}
			createResp.Body.Close()

			func() {
				delReq, _ := json.Marshal(map[string]any{"name": modelName})
				req, _ := http.NewRequest("DELETE", "http://localhost:11434/api/delete", bytes.NewReader(delReq))
				req.Header.Set("Content-Type", "application/json")
				client := &http.Client{}
				resp, _ := client.Do(req)
				if resp != nil {
					resp.Body.Close()
				}
			}()

			reqBody, _ := json.Marshal(map[string]any{
				"model":  modelName,
				"prompt": prompt,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				panic(err)
			}
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				panic(err)
			}
			resp.Body.Close()
			vm.push(res.Response)

		case bytecode.OpAchieve:
			constraintAny := vm.pop(inst.Op)
			targetAny := vm.pop(inst.Op)
			constraintStr := fmt.Sprintf("%v", constraintAny)
			targetStr := fmt.Sprintf("%v", targetAny)
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": "Achieve the following target: " + targetStr + " with constraint: " + constraintStr + ". Return ONLY the result, no explanations.",
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				panic(err)
			}
			vm.push(res.Response)

		case bytecode.OpConfidence:
			promptStrAny := vm.pop(inst.Op)
			promptStr := fmt.Sprintf("%v", promptStrAny)
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": "Evaluate the probability of this statement being true. Return ONLY a float between 0.0 and 1.0. Statement: " + promptStr,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				panic(err)
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(res.Response), 64)
			vm.push(val)

		case bytecode.OpLlmGenerate:
			modelStr := inst.StringOperand
			promptStr := vm.pop(inst.Op).(string)
			reqBody, _ := json.Marshal(map[string]any{
				"model":  modelStr,
				"prompt": promptStr,
				"stream": false,
			})
			resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
			if err != nil {
				panic(err)
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				panic(err)
			}
			vm.push(res.Response)

		case bytecode.OpLoadConst:
			vm.push(inst.ValueOperand)
		case bytecode.OpLoadVar:
			name := inst.StringOperand
			if val, ok := env.get(name); ok {
				vm.push(val)
			} else {
				panic("undefined variable: " + name)
			}
		case bytecode.OpStoreVar:
			name := inst.StringOperand
			env.vars[name] = vm.pop(inst.Op)
		case bytecode.OpSetVar:
			name := inst.StringOperand
			if !env.set(name, vm.pop(inst.Op)) {
				panic("undefined variable: " + name)
			}
		case bytecode.OpJumpIfFalse:
			cond := vm.pop(inst.Op)
			if !BcToBool(cond) {
				ip += int(inst.IntOperand)
				continue
			}
		case bytecode.OpJump:
			ip += int(inst.IntOperand)
			continue
		case bytecode.OpForInit:
			list := vm.pop(inst.Op)
			items, ok := list.([]any)
			if !ok {
				panic("for requires a list")
			}
			vm.push(items)
			vm.push(0.0) // index
		case bytecode.OpForNext:
			varName := inst.StringOperand
			offset := int(inst.IntOperand)

			idxAny := vm.pop(inst.Op)
			itemsAny := vm.pop(inst.Op)

			idx := int(idxAny.(float64))
			items := itemsAny.([]any)

			if idx < len(items) {
				env.vars[varName] = items[idx]
				vm.push(items)
				vm.push(float64(idx + 1))
			} else {
				ip += offset
				continue
			}
		case bytecode.OpCall:
			funcName := inst.StringOperand
			numArgs := int(inst.IntOperand)

			fn, ok := vm.prog.Functions[funcName]
			if !ok {
				panic("undefined function: " + funcName)
			}

			if fn.LazySynthesize {
				promptStr := fmt.Sprintf("You are a Zero compiler. Synthesize the Zero Lisp code for the function '%s' with parameters %v. Docstring: \"%s\"\n\nReply ONLY with the Zero Lisp code for the function body expressions. Do not include (defun ...). Do not include markdown formatting.\nFor example, if the docstring says \"Returns the sum of a and b\", you reply:\n(+ a b)", fn.Name, fn.Params, fn.Docstring)
				reqBody, _ := json.Marshal(map[string]any{
					"model":  "llama3",
					"prompt": promptStr,
					"stream": false,
				})
				resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
				if err != nil {
					panic(err)
				}
				var res struct {
					Response string `json:"response"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
					resp.Body.Close()
					panic(err)
				}
				resp.Body.Close()
				code := strings.TrimSpace(res.Response)

				// Strip potential markdown blocks
				code = strings.TrimPrefix(code, "```lisp")
				code = strings.TrimPrefix(code, "```")
				code = strings.TrimSuffix(code, "```")
				code = strings.TrimSpace(code)

				progCode := fmt.Sprintf("(defun %s (%s) %s)", fn.Name, strings.Join(fn.Params, " "), code)
				lx := lexer.NewLexer(progCode)
				p := parser.NewParser(lx, "lazy_synthesize")
				astNode := p.ParseExpression()

				newProg := bytecode.CompileToBytecode(astNode)
				fn.Instructions = newProg.Functions[fn.Name].Instructions
				fn.LazySynthesize = false
			}

			var argVals []any
			for i := 0; i < numArgs; i++ {
				argVals = append([]any{vm.pop(inst.Op)}, argVals...)
			}

			callEnv := NewBcEnv(nil)
			for i, p := range fn.Params {
				callEnv.vars[p] = argVals[i]
			}

			var result any
			func() {
				defer func() {
					if r := recover(); r != nil {
						if ret, ok := r.(VmReturn); ok {
							result = ret.val
							return
						}
						panic(r)
					}
				}()
				vm.run(fn.Instructions, callEnv)
			}()
			vm.push(result)
		case bytecode.OpReturn:
			panic(VmReturn{val: vm.pop(inst.Op)})
		case bytecode.OpPrint:
			numArgs := int(inst.IntOperand)
			var out []string
			var vals []any
			for i := 0; i < numArgs; i++ {
				vals = append([]any{vm.pop(inst.Op)}, vals...)
			}
			for _, v := range vals {
				out = append(out, fmt.Sprint(v))
			}
			fmt.Fprintln(vm.Out, strings.Join(out, " "))
		case bytecode.OpStderr:
			val := vm.pop(inst.Op)
			fmt.Fprint(vm.ErrOut, val)
		case bytecode.OpExit:
			val := vm.pop(inst.Op)
			var code int
			switch v := val.(type) {
			case int64:
				code = int(v)
			case float64:
				code = int(v)
			case int:
				code = v
			}
			panic(VmExit{code: code})
		case bytecode.OpReadLine:
			line, err := vm.lineReader.ReadString('\n')
			if err != nil && err != io.EOF {
				panic(NewRuntimeError("IO_ERROR", "main", vm.ip, inst.Op, "Failed to read line: %v", err))
			}
			if err == io.EOF && line == "" {
				vm.push("")
			} else {
				line = strings.TrimSuffix(line, "\n")
				line = strings.TrimSuffix(line, "\r")
				vm.push(line)
			}
		case bytecode.OpBinop:
			b := vm.pop(inst.Op)
			a := vm.pop(inst.Op)
			op := inst.StringOperand
			vm.push(BcBinop(op, a, b))
		case bytecode.OpConvert:
			a := vm.pop(inst.Op)
			target := inst.StringOperand
			vm.push(BcConvert(target, a))
		case bytecode.OpStrSplit:
			sep := vm.pop(inst.Op).(string)
			s := vm.pop(inst.Op).(string)
			vm.push(BcSliceToAny(strings.Split(s, sep)))
		case bytecode.OpStrJoin:
			sep := vm.pop(inst.Op).(string)
			list := vm.pop(inst.Op).([]any)
			var strs []string
			for _, item := range list {
				strs = append(strs, fmt.Sprint(item))
			}
			vm.push(strings.Join(strs, sep))
		case bytecode.OpRegexMatch:
			s := vm.pop(inst.Op).(string)
			pattern := vm.pop(inst.Op).(string)
			matched, err := regexp.MatchString(pattern, s)
			if err != nil {
				panic(err)
			}
			vm.push(matched)
		case bytecode.OpMakeList:
			num := int(inst.IntOperand)
			var items []any
			for i := 0; i < num; i++ {
				items = append([]any{vm.pop(inst.Op)}, items...)
			}
			vm.push(items)
		case bytecode.OpMakeDict:
			num := int(inst.IntOperand)
			dict := make(map[string]any)
			// pop pairs: value then key
			var pairs []any
			for i := 0; i < num*2; i++ {
				pairs = append([]any{vm.pop(inst.Op)}, pairs...)
			}
			for i := 0; i < num*2; i += 2 {
				key := fmt.Sprint(pairs[i])
				dict[key] = pairs[i+1]
			}
			vm.push(dict)
		case bytecode.OpAppend:
			varName := inst.StringOperand
			item := vm.pop(inst.Op)

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			items := current.([]any)
			newItems := append(append([]any{}, items...), item)
			env.set(varName, newItems)
		case bytecode.OpMapSet:
			varName := inst.StringOperand
			val := vm.pop(inst.Op)
			key := fmt.Sprint(vm.pop(inst.Op))

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			dict := current.(map[string]any)
			dict[key] = val
		case bytecode.OpMapDelete:
			varName := inst.StringOperand
			key := fmt.Sprint(vm.pop(inst.Op))

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			dict := current.(map[string]any)
			delete(dict, key)
		case bytecode.OpMapGet:
			varName := inst.StringOperand
			key := fmt.Sprint(vm.pop(inst.Op))

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			dict := current.(map[string]any)
			if val, ok := dict[key]; ok {
				vm.push(val)
			} else {
				vm.push("")
			}
		case bytecode.OpListGet:
			varName := inst.StringOperand
			idxAny := vm.pop(inst.Op)

			idxStr := strings.TrimSpace(fmt.Sprint(idxAny))
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				panic("list_get index must be a number")
			}

			current, ok := env.get(varName)
			if !ok {
				panic("undefined variable: " + varName)
			}
			items := current.([]any)
			if idx < 0 || idx >= len(items) {
				vm.push("")
			} else {
				vm.push(items[idx])
			}
		case bytecode.OpCliArgs:
			var argsAny []any
			for _, arg := range vm.args {
				argsAny = append(argsAny, arg)
			}
			vm.push(argsAny)
		case bytecode.OpCliArgsGet:
			idxAny := vm.pop(inst.Op)
			idx, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(idxAny)))
			if err != nil {
				panic("cli_args index must be a number")
			}
			if idx >= 0 && idx < len(vm.args) {
				vm.push(vm.args[idx])
			} else {
				vm.push("")
			}
		case bytecode.OpSleep:
			msAny := vm.pop(inst.Op)
			ms, ok := msAny.(float64)
			if !ok {
				panic("sleep requires number")
			}
			time.Sleep(time.Duration(ms) * time.Millisecond)
		case bytecode.OpEnv:
			name := vm.pop(inst.Op).(string)
			vm.push(os.Getenv(name))
		default:
			panic("unknown opcode: " + bytecode.Registry[inst.Op].Name)
		}
		ip++
	}
	return nil
}

func BcToBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	panic("expected boolean")
}

func BcBinop(op string, a, b any) any {
	switch op {
	case "and":
		return BcToBool(a) && BcToBool(b)
	case "or":
		return BcToBool(a) || BcToBool(b)
	case "==":
		return BcValuesEqual(a, b)
	case "!=":
		return !BcValuesEqual(a, b)
	case "+":
		if as, ok := a.(string); ok {
			if bs, ok2 := b.(string); ok2 {
				return as + bs
			}
		}
		return bcNumericBinop(op, a, b)
	default:
		return bcNumericBinop(op, a, b)
	}
}

func bcNumericBinop(op string, a, b any) any {
	// try to coerce float64 since JSON uses float64
	af := ToBCFloat(a)
	bf := ToBCFloat(b)

	switch op {
	case "+":
		return af + bf
	case "-":
		return af - bf
	case "*":
		return af * bf
	case "/":
		return af / bf
	case "<":
		return af < bf
	case ">":
		return af > bf
	case "<=":
		return af <= bf
	case ">=":
		return af >= bf
	}
	panic("unknown binop")
}

func ToBCFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	}
	panic(fmt.Sprintf("expected number, got %T", v))
}

func BcValuesEqual(a, b any) bool {
	switch a.(type) {
	case []any, map[string]any:
		return false
	}
	switch b.(type) {
	case []any, map[string]any:
		return false
	}
	return a == b
}

func BcConvert(target string, a any) any {
	switch target {
	case "to_int":
		switch t := a.(type) {
		case float64:
			return float64(int64(t))
		case string:
			v, _ := strconv.ParseInt(t, 10, 64)
			return float64(v)
		}
	case "to_float":
		switch t := a.(type) {
		case string:
			v, _ := strconv.ParseFloat(t, 64)
			return v
		case float64:
			return t
		}
	case "to_string":
		return fmt.Sprint(a)
	case "bytes_to_string":
		if b, ok := a.([]any); ok {
			var bytes []byte
			for _, bb := range b {
				bytes = append(bytes, byte(bb.(float64)))
			}
			return string(bytes)
		}
	}
	return a
}

func BcSliceToAny(strs []string) []any {
	out := make([]any, len(strs))
	for i, s := range strs {
		out[i] = s
	}
	return out
}
