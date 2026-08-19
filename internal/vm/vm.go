package vm

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/ir"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"io"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
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

func (e VmExit) Code() int {
	return e.code
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
		if node.Value == "true" {
			return true
		}
		if node.Value == "false" {
			return false
		}
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
		items := make([]any, 0, len(node.Children)-1)
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
	case "time_now":
		return int64(time.Now().Unix())
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
	case "list_len":
		if len(node.Children) != 2 {
			InterpErr("list_len expects (list_len list)", node)
		}
		val := interp.eval(node.Children[1], env)
		if lst, ok := val.([]any); ok {
			return int64(len(lst))
		}
		InterpErr(fmt.Sprintf("TYPE_ERROR: list_len expected list, got %T", val), node.Children[1])
		return int64(0)
	case "is_nil":
		if len(node.Children) != 2 {
			InterpErr("is_nil expects (is_nil val)", node)
		}
		val := interp.eval(node.Children[1], env)
		return val == nil
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
		promptStr := fmt.Sprintf("You are a HowlFrame compiler. Synthesize the HowlFrame Lisp code for the function '%s' with parameters %v. Docstring: \"%s\"\n\nReply ONLY with the HowlFrame Lisp code for the function body expressions. Do not include (defun ...). Do not include markdown formatting.\nFor example, if the docstring says \"Returns the sum of a and b\", you reply:\n(+ a b)", fn.name, fn.params, fn.docstring)
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
	trace       *boundedTrace
	mapLedger   *mapStateLedger
}

const (
	maxMapLedgerEvents    = 256
	maxMapLedgerResources = 64
)

// mapStateLedger is VM-private execution history. It tracks backing-map
// identity rather than binding names so aliases join and rebindings separate.
type mapStateLedger struct {
	events    []bytecode.MapStateEvent
	complete  bool
	nextSeq   uint64
	nextID    uint64
	resources map[uintptr]uint64
	retained  map[uintptr]map[string]any
	versions  map[uint64]uint64
}

func newMapStateLedger() *mapStateLedger {
	return &mapStateLedger{complete: true, resources: make(map[uintptr]uint64), retained: make(map[uintptr]map[string]any), versions: make(map[uint64]uint64)}
}

func (ledger *mapStateLedger) resourceID(dict map[string]any) (uint64, bool) {
	if ledger == nil || !ledger.complete {
		return 0, false
	}
	pointer := reflect.ValueOf(dict).Pointer()
	if id, ok := ledger.resources[pointer]; ok {
		return id, true
	}
	if len(ledger.resources) >= maxMapLedgerResources {
		ledger.complete = false
		return 0, false
	}
	ledger.nextID++
	id := ledger.nextID
	ledger.resources[pointer] = id
	// Retain every registered map until the run ends, preventing pointer reuse.
	ledger.retained[pointer] = dict
	ledger.versions[id] = 0
	return id, true
}

func (ledger *mapStateLedger) record(event bytecode.MapStateEvent) {
	if ledger == nil || !ledger.complete {
		return
	}
	if len(ledger.events) >= maxMapLedgerEvents {
		ledger.complete = false
		ledger.events = nil
		return
	}
	ledger.nextSeq++
	event.Sequence = ledger.nextSeq
	ledger.events = append(ledger.events, event)
}

func (ledger *mapStateLedger) init(dict map[string]any, instruction int, nodeID string) {
	id, ok := ledger.resourceID(dict)
	if !ok {
		return
	}
	ledger.record(bytecode.MapStateEvent{ResourceID: id, Instruction: instruction, NodeID: nodeID, Operation: "INIT"})
	for key := range dict {
		ledger.record(bytecode.MapStateEvent{ResourceID: id, Instruction: instruction, NodeID: nodeID, Operation: "INIT", KeyFingerprint: bytecode.RuntimeKeyFingerprint(key)})
	}
}

func (ledger *mapStateLedger) mutation(dict map[string]any, instruction int, nodeID, operation, key string, value any, deleted bool) {
	id, ok := ledger.resourceID(dict)
	if !ok {
		return
	}
	before := ledger.versions[id]
	ledger.versions[id]++
	fingerprint, _ := bytecode.RuntimeValueFingerprint(value)
	ledger.record(bytecode.MapStateEvent{ResourceID: id, Instruction: instruction, NodeID: nodeID, Operation: operation, KeyFingerprint: bytecode.RuntimeKeyFingerprint(key), ValueFingerprint: fingerprint, VersionBefore: before, VersionAfter: ledger.versions[id], DeletedExisting: deleted})
}

func (ledger *mapStateLedger) read(dict map[string]any, instruction int, nodeID, key string, hit bool, value any) {
	id, ok := ledger.resourceID(dict)
	if !ok {
		return
	}
	fingerprint, _ := bytecode.RuntimeValueFingerprint(value)
	ledger.record(bytecode.MapStateEvent{ResourceID: id, Instruction: instruction, NodeID: nodeID, Operation: "GET", KeyFingerprint: bytecode.RuntimeKeyFingerprint(key), ValueFingerprint: fingerprint, VersionBefore: ledger.versions[id], VersionAfter: ledger.versions[id], Hit: hit})
}

type boundedTrace struct {
	limit     int
	events    []bytecode.ExecutionTraceEvent
	truncated bool
}

func (trace *boundedTrace) record(ip int, inst bytecode.BCInstruction, prog *bytecode.BCProgram) {
	if trace == nil {
		return
	}
	if trace.limit <= 0 || len(trace.events) >= trace.limit {
		trace.truncated = true
		return
	}
	opcode := inst.OpString
	if spec, ok := bytecode.Registry[inst.Op]; ok {
		opcode = spec.Name
	}
	nodeID, _ := prog.TrustedMainOriginAt(ip)
	trace.events = append(trace.events, bytecode.ExecutionTraceEvent{Instruction: ip, Opcode: opcode, NodeID: nodeID})
}

func (trace *boundedTrace) branch(taken bool) {
	if trace == nil || len(trace.events) == 0 {
		return
	}
	trace.events[len(trace.events)-1].BranchTaken = &taken
}

// state records an opaque state-key fingerprint after the VM has consumed the
// actual key. It is runner-owned evidence, not a program-visible value.
func (trace *boundedTrace) state(resource string, key any) {
	if trace == nil || len(trace.events) == 0 {
		return
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%T:%#v", key, key)))
	trace.events[len(trace.events)-1].Resource = resource
	trace.events[len(trace.events)-1].StateKey = hex.EncodeToString(digest[:])
}

type bcStoreRegistry struct {
	mu     sync.Mutex
	stores map[string]*bcMemoryStore
}

type bcMemoryStore struct {
	mu      sync.RWMutex
	records map[string]map[string]any
	file    string
}

func newBCStoreRegistry() *bcStoreRegistry {
	return &bcStoreRegistry{stores: make(map[string]*bcMemoryStore)}
}

func (registry *bcStoreRegistry) open(uri string) (*bcMemoryStore, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	store, ok := registry.stores[uri]
	if !ok {
		store = &bcMemoryStore{records: make(map[string]map[string]any)}
		if strings.HasPrefix(uri, "file://") {
			store.file = strings.TrimPrefix(uri, "file://")
			data, err := os.ReadFile(store.file)
			if err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("read persisted store %q: %w", store.file, err)
			}
			if err == nil {
				if unmarshalErr := json.Unmarshal(data, &store.records); unmarshalErr != nil {
					return nil, fmt.Errorf("decode persisted store %q: %w", store.file, unmarshalErr)
				}
				if store.records == nil {
					store.records = make(map[string]map[string]any)
				}
			}
		}
		registry.stores[uri] = store
	}
	return store, nil
}

func (s *bcMemoryStore) syncToFile(records map[string]map[string]any) error {
	if s.file == "" {
		return nil
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("encode persisted store %q: %w", s.file, err)
	}
	if err := os.WriteFile(s.file, data, 0644); err != nil {
		return fmt.Errorf("write persisted store %q: %w", s.file, err)
	}
	return nil
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
	evidence := RunBytecodeWithEvidence(prog, cliArgs, policy, allowedCaps, in, out, errOut, 0)
	if evidence.RuntimeFailure != nil {
		if errOut == nil {
			errOut = os.Stderr
		}
		fmt.Fprintln(errOut, mustVMErrorString(evidence.RuntimeFailure))
		os.Exit(1)
	}
	return evidence.ExitCode
}

// RunBytecodeWithEvidence executes bytecode without process termination and
// returns a bounded runner-owned trace for trusted consumers such as HFIR
// failure localization. traceLimit is a runner policy, never program input.
func RunBytecodeWithEvidence(prog *bytecode.BCProgram, cliArgs []string, policy ExecutionPolicy, allowedCaps []capability.Capability, in io.Reader, out io.Writer, errOut io.Writer, traceLimit int) (evidence bytecode.ExecutionEvidence) {
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
		trace:       &boundedTrace{limit: traceLimit},
		mapLedger:   newMapStateLedger(),
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
		evidence.Trace = append([]bytecode.ExecutionTraceEvent(nil), vm.trace.events...)
		evidence.TraceTruncated = vm.trace.truncated
		evidence.MapLedgerComplete = vm.mapLedger.complete
		if vm.mapLedger.complete {
			evidence.MapStateEvents = append([]bytecode.MapStateEvent(nil), vm.mapLedger.events...)
		}
		if r := recover(); r != nil {
			if exit, ok := r.(VmExit); ok {
				evidence.ExitCode = exit.code
			} else if vmerr, ok := r.(*VMError); ok {
				if nodeID, ok := prog.TrustedMainOriginAt(vmerr.Instruction); ok {
					vmerr.NodeID = nodeID
				}
				evidence.RuntimeFailure = &bytecode.RuntimeFailure{Code: vmerr.Code, Instruction: vmerr.Instruction, Opcode: vmerr.Opcode, NodeID: vmerr.NodeID, Message: vmerr.Message}
			} else {
				evidence.RuntimeFailure = &bytecode.RuntimeFailure{Code: "VM_INTERNAL", Instruction: vm.ip, Message: fmt.Sprintf("%v", r)}
			}
		}
		bytecode.SealExecutionEvidence(prog, &evidence)
	}()

	vm.run(vm.insts, vm.env)
	return evidence
}

func mustVMErrorString(failure *bytecode.RuntimeFailure) string {
	return (&VMError{Phase: "runtime", Code: failure.Code, Function: "main", Instruction: failure.Instruction, Opcode: failure.Opcode, NodeID: failure.NodeID, Message: failure.Message}).Error()
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

func (vm *BCVM) requireCapability(cap capability.Capability, op bytecode.Opcode) {
	for _, allowed := range vm.AllowedCaps {
		if allowed == cap {
			return
		}
	}
	panic(NewRuntimeError("CAPABILITY_DENIED", "main", vm.ip, op, "capability denied: %s", cap))
}

func (vm *BCVM) requireStoreCapabilities(uri string, op bytecode.Opcode) {
	required, valid := capability.StoreRequirements(uri)
	if !valid {
		panic(NewRuntimeError("INVALID_STORE_URI", "main", vm.ip, op, "store URI must use memory:// or file:// with a non-empty name"))
	}
	for _, requiredCap := range required {
		vm.requireCapability(requiredCap, op)
	}
}

func (vm *BCVM) persistStore(store *bcMemoryStore, records map[string]map[string]any, op bytecode.Opcode) {
	if err := store.syncToFile(records); err != nil {
		panic(NewRuntimeError("STORE_PERSISTENCE_WRITE_FAILED", "main", vm.ip, op, "%v", err))
	}
	store.records = records
}

func cloneStoreRecord(record map[string]any) map[string]any {
	clone := make(map[string]any, len(record))
	for key, value := range record {
		clone[key] = cloneStoreValue(value)
	}
	return clone
}

func cloneStoreRecords(records map[string]map[string]any) map[string]map[string]any {
	clone := make(map[string]map[string]any, len(records))
	for key, record := range records {
		clone[key] = cloneStoreRecord(record)
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
		vm.trace.record(ip, inst, vm.prog)
		// MaxInstructions is the maximum number of instructions allowed. Check
		// before incrementing so an exact budget succeeds and even MaxInt cannot
		// overflow into effectively unlimited execution.
		if vm.Limits.MaxInstructions <= 0 || vm.executed >= vm.Limits.MaxInstructions {
			panic(NewRuntimeError("LIMIT_EXCEEDED", "main", vm.ip, inst.Op, "instruction limit exceeded"))
		}
		vm.executed++

		spec := bytecode.Registry[inst.Op]
		if spec.Capability != capability.None {
			vm.requireCapability(spec.Capability, inst.Op)
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
			vm.requireStoreCapabilities(uri, inst.Op)
			store, err := vm.stores.open(uri)
			if err != nil {
				code := "STORE_PERSISTENCE_READ_FAILED"
				if strings.Contains(err.Error(), "decode persisted store") {
					code = "STORE_INVALID_PERSISTED_JSON"
				}
				panic(NewRuntimeError(code, "main", vm.ip, inst.Op, "%v", err))
			}
			env.vars[inst.StringOperand] = store
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
			if store.file != "" {
				vm.requireCapability(capability.Filesystem, inst.Op)
			}
			store.mu.Lock()
			records := cloneStoreRecords(store.records)
			records[key] = cloneStoreRecord(record)
			vm.persistStore(store, records, inst.Op)
			store.mu.Unlock()
		case bytecode.OpStoreGet:
			key := fmt.Sprint(vm.pop(inst.Op))
			store := vm.storeHandle(env, inst.StringOperand, inst.Op)
			if store.file != "" {
				vm.requireCapability(capability.Filesystem, inst.Op)
			}
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
			if store.file != "" {
				vm.requireCapability(capability.Filesystem, inst.Op)
			}
			store.mu.Lock()
			records := cloneStoreRecords(store.records)
			delete(records, key)
			vm.persistStore(store, records, inst.Op)
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
				if req.Body == nil {
					panic("request body is nil")
				}
				// Bound request body read to 10MB to prevent unbounded memory DoS
				const maxBodyBytes = 10 * 1024 * 1024
				limitedReader := io.LimitReader(req.Body, maxBodyBytes+1)
				readData, err := io.ReadAll(limitedReader)
				if err != nil {
					panic(fmt.Sprintf("failed to read request body: %v", err))
				}
				if len(readData) > maxBodyBytes {
					panic(fmt.Sprintf("request body exceeds maximum limit of %d bytes", maxBodyBytes))
				}
				data = readData
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
				childVM := &BCVM{prog: vm.prog, env: cEnv, stores: vm.stores, Limits: vm.Limits, AllowedCaps: vm.AllowedCaps, Out: vm.Out, ErrOut: vm.ErrOut}
				childVM.run(bodyInsts, cEnv)
			}(capturedEnv)
			ip += bodyLen
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
				childVM := &BCVM{prog: prog, env: reqEnv, stores: vm.stores, Limits: vm.Limits, AllowedCaps: vm.AllowedCaps, Out: vm.Out, ErrOut: vm.ErrOut}
				func() {
					defer func() {
						if r := recover(); r != nil {
							fmt.Println("HTTP Handler Panic:", r)
						}
					}()
					childVM.run(bodyInsts, reqEnv)
				}()
			})
			ip += bodyLen
		case bytecode.OpHttpServerServe:
			muxAny, _ := env.get("__http_mux")
			mux := muxAny.(*http.ServeMux)
			portAny, _ := env.get("__http_port")
			port := portAny.(string)
			fmt.Fprintln(vm.Out, "Listening on "+port)
			err := http.ListenAndServe(":"+port, mux)
			if err != nil {
				panic(err)
			}
		case bytecode.OpHttpReqMethod:
			reqAny, ok := env.get("req")
			if !ok {
				panic("no request context")
			}
			req := reqAny.(*http.Request)
			vm.push(req.Method)
		case bytecode.OpHttpResHeader:
			value := vm.pop(inst.Op)
			name := vm.pop(inst.Op)
			wAny, ok := env.get("w")
			if !ok {
				panic("no response writer")
			}
			w := wAny.(http.ResponseWriter)
			w.Header().Set(fmt.Sprint(name), fmt.Sprint(value))
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
			vm.trace.state("var:"+name, "value")
			if val, ok := env.get(name); ok {
				vm.push(val)
			} else {
				panic("undefined variable: " + name)
			}
		case bytecode.OpStoreVar:
			name := inst.StringOperand
			vm.trace.state("var:"+name, "value")
			env.vars[name] = vm.pop(inst.Op)
		case bytecode.OpSetVar:
			name := inst.StringOperand
			vm.trace.state("var:"+name, "value")
			if !env.set(name, vm.pop(inst.Op)) {
				panic("undefined variable: " + name)
			}
		case bytecode.OpJumpIfFalse:
			cond := vm.pop(inst.Op)
			if !BcToBool(cond) {
				vm.trace.branch(true)
				ip += int(inst.IntOperand)
				continue
			}
			vm.trace.branch(false)
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
				promptStr := fmt.Sprintf("You are a HowlFrame compiler. Synthesize the HowlFrame Lisp code for the function '%s' with parameters %v. Docstring: \"%s\"\n\nReply ONLY with the HowlFrame Lisp code for the function body expressions. Do not include (defun ...). Do not include markdown formatting.\nFor example, if the docstring says \"Returns the sum of a and b\", you reply:\n(+ a b)", fn.Name, fn.Params, fn.Docstring)
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

			callEnv := NewBcEnv(env)
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
			sepVal := vm.pop(inst.Op)
			sVal := vm.pop(inst.Op)
			sep, ok1 := sepVal.(string)
			s, ok2 := sVal.(string)
			if !ok1 || !ok2 {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "str_split expected string, got %T and %T", sVal, sepVal))
			}
			vm.push(BcSliceToAny(strings.Split(s, sep)))
		case bytecode.OpStrJoin:
			sepVal := vm.pop(inst.Op)
			listVal := vm.pop(inst.Op)
			sep, ok1 := sepVal.(string)
			list, ok2 := listVal.([]any)
			if !ok1 || !ok2 {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "str_join expected list and string, got %T and %T", listVal, sepVal))
			}
			var strs []string
			for _, item := range list {
				strs = append(strs, fmt.Sprint(item))
			}
			vm.push(strings.Join(strs, sep))
		case bytecode.OpRegexMatch:
			sVal := vm.pop(inst.Op)
			patternVal := vm.pop(inst.Op)
			s, ok1 := sVal.(string)
			pattern, ok2 := patternVal.(string)
			if !ok1 || !ok2 {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "regex_match expected string pattern and string, got %T and %T", patternVal, sVal))
			}
			matched, err := regexp.MatchString(pattern, s)
			if err != nil {
				panic(NewRuntimeError("INVALID_REGEX", "main", ip, inst.Op, "invalid regex: %v", err))
			}
			vm.push(matched)
		case bytecode.OpMakeList:
			num := int(inst.IntOperand)
			items := make([]any, 0, num)
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
			nodeID, _ := vm.prog.TrustedMainOriginAt(ip)
			vm.mapLedger.init(dict, ip, nodeID)
			vm.push(dict)
		case bytecode.OpAppend:
			varName := inst.StringOperand
			item := vm.pop(inst.Op)

			current, ok := env.get(varName)
			if !ok {
				panic(NewRuntimeError("UNDEFINED_VAR", "main", ip, inst.Op, "undefined variable: %s", varName))
			}
			items, ok := current.([]any)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "append expected list, got %T", current))
			}
			vm.trace.state("list:"+varName, len(items))
			newItems := append(append([]any{}, items...), item)
			env.set(varName, newItems)
		case bytecode.OpMapSet:
			varName := inst.StringOperand
			val := vm.pop(inst.Op)
			keyAny := vm.pop(inst.Op)
			key := fmt.Sprint(keyAny)
			current, ok := env.get(varName)
			if !ok {
				panic(NewRuntimeError("UNDEFINED_VAR", "main", ip, inst.Op, "undefined variable: %s", varName))
			}
			dict, ok := current.(map[string]any)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "map_set expected dict, got %T", current))
			}
			dict[key] = val
			vm.trace.state("map:"+varName, key)
			nodeID, _ := vm.prog.TrustedMainOriginAt(ip)
			vm.mapLedger.mutation(dict, ip, nodeID, "SET", key, val, false)
		case bytecode.OpMapDelete:
			varName := inst.StringOperand
			keyAny := vm.pop(inst.Op)
			key := fmt.Sprint(keyAny)
			current, ok := env.get(varName)
			if !ok {
				panic(NewRuntimeError("UNDEFINED_VAR", "main", ip, inst.Op, "undefined variable: %s", varName))
			}
			dict, ok := current.(map[string]any)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "map_delete expected dict, got %T", current))
			}
			_, deleted := dict[key]
			delete(dict, key)
			vm.trace.state("map:"+varName, key)
			nodeID, _ := vm.prog.TrustedMainOriginAt(ip)
			vm.mapLedger.mutation(dict, ip, nodeID, "DELETE", key, nil, deleted)
		case bytecode.OpMapGet:
			varName := inst.StringOperand
			keyAny := vm.pop(inst.Op)
			key := fmt.Sprint(keyAny)
			current, ok := env.get(varName)
			if !ok {
				panic(NewRuntimeError("UNDEFINED_VAR", "main", ip, inst.Op, "undefined variable: %s", varName))
			}
			dict, ok := current.(map[string]any)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "map_get expected dict, got %T", current))
			}
			if val, ok := dict[key]; ok {
				vm.trace.state("map:"+varName, key)
				nodeID, _ := vm.prog.TrustedMainOriginAt(ip)
				vm.mapLedger.read(dict, ip, nodeID, key, true, val)
				vm.push(val)
			} else {
				vm.trace.state("map:"+varName, key)
				nodeID, _ := vm.prog.TrustedMainOriginAt(ip)
				vm.mapLedger.read(dict, ip, nodeID, key, false, nil)
				vm.push("")
			}
		case bytecode.OpListGet:
			varName := inst.StringOperand
			idxAny := vm.pop(inst.Op)

			idxStr := strings.TrimSpace(fmt.Sprint(idxAny))
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "list_get index must be a number, got %T", idxAny))
			}
			vm.trace.state("list:"+varName, idx)

			current, ok := env.get(varName)
			if !ok {
				panic(NewRuntimeError("UNDEFINED_VAR", "main", ip, inst.Op, "undefined variable: %s", varName))
			}
			items, ok := current.([]any)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "list_get expected list, got %T", current))
			}
			if idx < 0 || idx >= len(items) {
				vm.push("")
			} else {
				vm.push(items[idx])
			}
		case bytecode.OpListLen:
			val := vm.pop(inst.Op)
			items, ok := val.([]any)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "list_len expected list, got %T", val))
			}
			vm.push(int64(len(items)))
		case bytecode.OpIsNil:
			val := vm.pop(inst.Op)
			vm.push(val == nil)
		case bytecode.OpTimeNow:
			vm.push(int64(time.Now().Unix()))
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
		if bf == 0 {
			panic("division by zero")
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
