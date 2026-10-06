package vm

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/htmlescape"
	"github.com/howlcipher/howlframe/internal/httpreq"
	"github.com/howlcipher/howlframe/internal/ir"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"io"
	"math"
	"math/bits"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"sort"
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
	funcs       map[string]*InterpFunc
	args        []string
	In          io.Reader
	Out         io.Writer
	ErrOut      io.Writer
	lineReader  *bufio.Reader
	AllowedCaps []capability.Capability
}

func (interp *Interpreter) requireCapability(cap capability.Capability, node *ast.Node) {
	for _, allowed := range interp.AllowedCaps {
		if allowed == cap {
			return
		}
	}
	InterpErr(fmt.Sprintf("capability denied: %s", cap), node)
}

type VmExit struct {
	code int
}

func (e VmExit) Code() int {
	return e.code
}

type interpError struct {
	reason string
	line   int
	col    int
}

func InterpErr(reason string, node *ast.Node) {
	line, col := 0, 0
	if node != nil {
		line, col = node.Line, node.Column
	}
	panic(interpError{reason: reason, line: line, col: col})
}

// escapeHTMLText encodes a string for HTML text or a quoted attribute.
// Both contexts use the same five-character encoding. A non-string fails
// closed; the operation grants no capability.
func escapeHTMLText(val any, opName string, node *ast.Node) string {
	s, ok := val.(string)
	if !ok {
		InterpErr(fmt.Sprintf("TYPE_ERROR: %s expected string, got %T", opName, val), node)
	}
	return htmlescape.Escape(s)
}

// sortedMapKeys returns dict keys in UTF-8 byte order as a non-nil list.
// sort.Strings is that order: Go compares strings as bytes, the same rule
// the Go backend emits. It is not UTF-16 code-unit order. A supplementary
// plane key (for example U+1F600) sorts after U+F000 here, and before it
// under JavaScript's default Array sort. Go randomizes map iteration. An
// empty dict must stay an empty list so JSON callers see [] rather than null.
func sortedMapKeys(dict map[string]any) []any {
	keys := make([]string, 0, len(dict))
	for key := range dict {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]any, len(keys))
	for i, key := range keys {
		out[i] = key
	}
	return out
}

// Interpret executes a cli_app AST directly and returns a process exit code.
// http_server/web_app roots are rejected with a clear error — Phase 1 is
// cli_app only, per docs/direct_execution_design.md.
// Enforces allowedCaps matching RunBytecodeWithPolicy (HOWL-CANON-002).
func Interpret(root *ast.Node, args []string, allowedCaps []capability.Capability, in io.Reader, out io.Writer, errOut io.Writer) (exitCode int) {
	interp := &Interpreter{
		funcs:       make(map[string]*InterpFunc),
		args:        args,
		In:          in,
		Out:         out,
		ErrOut:      errOut,
		AllowedCaps: allowedCaps,
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
			if interpErr, ok := r.(interpError); ok {
				errOutObj := ast.ErrorOutput{Reason: interpErr.reason, Line: interpErr.line, Column: interpErr.col}
				b, _ := json.Marshal(errOutObj)
				if interp.ErrOut != nil {
					fmt.Fprintln(interp.ErrOut, string(b))
				} else {
					fmt.Println(string(b))
				}
				exitCode = 1
				return
			}
			panic(r)
		}
	}()

	if root == nil || root.Type != "List" || len(root.Children) == 0 || root.Children[0].Type != "SYMBOL" {
		InterpErr("Expected cli_app as root symbol", root)
	}
	rootSym := root.Children[0].Value
	if rootSym != "cli_app" {
		InterpErr(fmt.Sprintf("-run only supports cli_app in Phase 1 (see docs/direct_execution_design.md); got %q", rootSym), root.Children[0])
	}

	globalEnv := NewInterpEnv(nil)

	for _, child := range root.Children[1:] {
		if IsDefun(child) {
			interp.registerDefun(child)
		}
	}

	for _, child := range root.Children[1:] {
		if !IsDefun(child) {
			interp.eval(child, globalEnv)
		}
	}
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

	if reqCap := capability.ForConstruct(head); reqCap != capability.None {
		interp.requireCapability(reqCap, node)
	}
	if head == "neural_circuit" || head == "ephemeral_circuit" {
		interp.requireCapability(capability.Network, node)
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
		exitCode, err := ToInt(val)
		if err != nil {
			InterpErr(fmt.Sprintf("exit expects a numeric code, got %T", val), node.Children[1])
		}
		panic(VmExit{code: int(exitCode)})
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
			InterpErr(fmt.Sprintf("neural_circuit: network request failed: %v", err), node)
		}
		defer resp.Body.Close()
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			InterpErr(fmt.Sprintf("neural_circuit: response decode failed: %v", err), node)
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
			InterpErr(fmt.Sprintf("ephemeral_circuit: model creation failed: %v", err), node)
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
			InterpErr(fmt.Sprintf("ephemeral_circuit: network request failed: %v", err), node)
		}
		defer resp.Body.Close()
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			InterpErr(fmt.Sprintf("ephemeral_circuit: response decode failed: %v", err), node)
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
			InterpErr(fmt.Sprintf("achieve: network request failed: %v", err), node)
		}
		defer resp.Body.Close()
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			InterpErr(fmt.Sprintf("achieve: response decode failed: %v", err), node)
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
			InterpErr(fmt.Sprintf("confidence: network request failed: %v", err), node)
		}
		defer resp.Body.Close()
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			InterpErr(fmt.Sprintf("confidence: response decode failed: %v", err), node)
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
		value := interp.eval(node.Children[1], env)
		switch typed := value.(type) {
		case int64:
			return typed
		case float64:
			if math.IsNaN(typed) || math.IsInf(typed, 0) || typed < -9223372036854775808.0 || typed >= 9223372036854775808.0 {
				InterpErr(fmt.Sprintf("CONVERSION_ERROR: cannot convert %v to int", typed), node.Children[1])
			}
			return int64(typed)
		case string:
			parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
			if err != nil {
				InterpErr(fmt.Sprintf("CONVERSION_ERROR: cannot convert %q to int", typed), node.Children[1])
			}
			return parsed
		default:
			InterpErr(fmt.Sprintf("CONVERSION_ERROR: cannot convert %T to int", value), node.Children[1])
		}
	case "to_float":
		if len(node.Children) != 2 {
			InterpErr("to_float expects (to_float val)", node)
		}
		value := interp.eval(node.Children[1], env)
		switch typed := value.(type) {
		case int64:
			return float64(typed)
		case float64:
			return typed
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
			if err != nil {
				InterpErr(fmt.Sprintf("CONVERSION_ERROR: cannot convert %q to float", typed), node.Children[1])
			}
			return parsed
		default:
			InterpErr(fmt.Sprintf("CONVERSION_ERROR: cannot convert %T to float", value), node.Children[1])
		}
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
	case "map_keys":
		if len(node.Children) != 2 {
			InterpErr("map_keys expects (map_keys dict)", node)
		}
		val := interp.eval(node.Children[1], env)
		dict, ok := val.(map[string]any)
		if !ok {
			InterpErr(fmt.Sprintf("TYPE_ERROR: map_keys expected dict, got %T", val), node.Children[1])
		}
		return sortedMapKeys(dict)
	case "req_query", "req_header", "req_path":
		if len(node.Children) != 3 {
			InterpErr(fmt.Sprintf("%s expects (%s req name)", head, head), node)
		}
		recv := interp.eval(node.Children[1], env)
		name := interp.eval(node.Children[2], env)
		var value string
		var err error
		switch head {
		case "req_header":
			value, err = httpreq.Header(recv, name)
		case "req_path":
			value, err = httpreq.Path(recv, name)
		default:
			value, err = httpreq.Query(recv, name)
		}
		if err != nil {
			InterpErr(err.Error(), node)
		}
		return value
	case "is_nil":
		if len(node.Children) != 2 {
			InterpErr("is_nil expects (is_nil val)", node)
		}
		val := interp.eval(node.Children[1], env)
		return val == nil
	case "encode_json":
		if len(node.Children) != 2 {
			InterpErr("encode_json expects (encode_json val)", node)
		}
		val := interp.eval(node.Children[1], env)
		b, err := json.Marshal(normalizeForJSON(val))
		if err != nil {
			InterpErr(fmt.Sprintf("encode_json: %v", err), node)
		}
		return string(b)
	case "html_escape", "attr_escape":
		if len(node.Children) != 2 {
			InterpErr(fmt.Sprintf("%s expects (%s text)", head, head), node)
		}
		return escapeHTMLText(interp.eval(node.Children[1], env), head, node)
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
	case "exec":
		// requireCapability already ran for process. Spawn only after that grant.
		if len(node.Children) < 2 {
			InterpErr("exec expects (exec cmd args...)", node)
		}
		cmd := fmt.Sprint(interp.eval(node.Children[1], env))
		args := make([]string, 0, len(node.Children)-2)
		for _, argNode := range node.Children[2:] {
			args = append(args, fmt.Sprint(interp.eval(argNode, env)))
		}
		out, err := exec.Command(cmd, args...).CombinedOutput()
		if err != nil {
			InterpErr(fmt.Sprintf("IO_ERROR: exec failed: %v", err), node)
		}
		return out
	case "read_file":
		// requireCapability already ran for filesystem. Read only after that grant.
		if len(node.Children) != 2 {
			InterpErr("read_file expects (read_file path)", node)
		}
		path := fmt.Sprint(interp.eval(node.Children[1], env))
		b, err := os.ReadFile(path)
		if err != nil {
			InterpErr(fmt.Sprintf("IO_ERROR: read_file failed: %v", err), node)
		}
		return b
	case "write_file":
		// requireCapability already ran for filesystem. Write only after that grant.
		{
			if len(node.Children) != 3 {
				InterpErr("write_file expects (write_file path data)", node)
			}
			writePath := fmt.Sprint(interp.eval(node.Children[1], env))
			var data []byte
			switch v := interp.eval(node.Children[2], env).(type) {
			case string:
				data = []byte(v)
			case []byte:
				data = v
			default:
				InterpErr(fmt.Sprintf("TYPE_ERROR: write_file expected string data, got %T", v), node)
			}
			if err := os.WriteFile(writePath, data, 0644); err != nil {
				InterpErr(fmt.Sprintf("IO_ERROR: write_file failed: %v", err), node)
			}
			return nil
		}
	case "mkdir":
		// requireCapability already ran for filesystem. Create the directory only after that grant.
		{
			if len(node.Children) != 2 {
				InterpErr("mkdir expects (mkdir path)", node)
			}
			dirPath := fmt.Sprint(interp.eval(node.Children[1], env))
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				InterpErr(fmt.Sprintf("IO_ERROR: mkdir failed: %v", err), node)
			}
			return nil
		}
	case "fetch":
		// requireCapability already ran for network. The request is sent only
		// after that grant, so a denial cannot open a connection.
		if len(node.Children) != 3 && len(node.Children) != 4 {
			InterpErr("fetch expects (fetch url method [body])", node)
		}
		urlStr := fmt.Sprint(interp.eval(node.Children[1], env))
		method := fmt.Sprint(interp.eval(node.Children[2], env))
		var body io.Reader
		if len(node.Children) == 4 {
			body = strings.NewReader(fmt.Sprint(interp.eval(node.Children[3], env)))
		}
		req, err := http.NewRequest(method, urlStr, body)
		if err != nil {
			InterpErr(fmt.Sprintf("IO_ERROR: fetch request creation failed: %v", err), node)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			InterpErr(fmt.Sprintf("IO_ERROR: fetch failed: %v", err), node)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			InterpErr(fmt.Sprintf("IO_ERROR: fetch body read failed: %v", err), node)
		}
		return b
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
		// Synthesis sends a model request, so it needs the same network grant
		// the bytecode VM requires for a lazy_synthesize CALL. Check before
		// the request is built so a denied run never reaches the model host.
		interp.requireCapability(capability.Network, node)
		promptStr := fmt.Sprintf("You are a HowlFrame compiler. Synthesize the HowlFrame Lisp code for the function '%s' with parameters %v. Docstring: \"%s\"\n\nReply ONLY with the HowlFrame Lisp code for the function body expressions. Do not include (defun ...). Do not include markdown formatting.\nFor example, if the docstring says \"Returns the sum of a and b\", you reply:\n(+ a b)", fn.name, fn.params, fn.docstring)
		reqBody, _ := json.Marshal(map[string]any{
			"model":  "llama3",
			"prompt": promptStr,
			"stream": false,
		})
		resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
		if err != nil {
			InterpErr(fmt.Sprintf("lazy_synthesize: network request failed: %v", err), node)
		}
		var res struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			resp.Body.Close()
			InterpErr(fmt.Sprintf("lazy_synthesize: response decode failed: %v", err), node)
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
		InterpErr(fmt.Sprintf("TYPE_ERROR: append expected list, got %T", current), listNode)
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
		InterpErr(fmt.Sprintf("TYPE_ERROR: map_set expected dict, got %T", current), dictNode)
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
		InterpErr(fmt.Sprintf("TYPE_ERROR: map_delete expected dict, got %T", current), dictNode)
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
	var current any
	if dictNode.Type == "SYMBOL" {
		var ok bool
		current, ok = env.get(dictNode.Value)
		if !ok {
			InterpErr(fmt.Sprintf("undefined variable: %s", dictNode.Value), dictNode)
		}
	} else {
		current = interp.eval(dictNode, env)
	}
	d, ok := current.(map[string]any)
	if !ok {
		// A missing key is "". Reading through that sentinel, or any other
		// non-dict, is a type error rather than another absence value.
		InterpErr(fmt.Sprintf("TYPE_ERROR: map_get expected dict, got %T", current), dictNode)
	}
	key := fmt.Sprint(interp.eval(node.Children[2], env))
	if val, ok := d[key]; ok {
		return val
	}
	// Missing keys are the empty string. A missing store record is nil.
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
		InterpErr(fmt.Sprintf("TYPE_ERROR: list_get expected list, got %T", current), listNode)
	}
	idxVal := interp.eval(node.Children[2], env)
	idx, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(idxVal)))
	if err != nil {
		InterpErr(fmt.Sprintf("TYPE_ERROR: list_get index must be a number, got %T", idxVal), node.Children[2])
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
	switch op {
	case "+", "-", "*":
		return interpNumericArithmetic(op, a, b, node)
	case "/":
		return interpNumericDiv(a, b, node)
	case "<", ">", "<=", ">=":
		return interpNumericRelational(op, a, b, node)
	}
	InterpErr(fmt.Sprintf("unknown numeric operator: %s", op), node)
	return nil
}

func interpNumericArithmetic(op string, a, b any, node *ast.Node) any {
	ai, aIsInt := asInt64(a)
	bi, bIsInt := asInt64(b)
	if aIsInt && bIsInt {
		switch op {
		case "+":
			return checkedAddInt64(ai, bi, node)
		case "-":
			return checkedSubInt64(ai, bi, node)
		case "*":
			return checkedMulInt64(ai, bi, node)
		}
	}
	af, aOk := toFloat64(a)
	bf, bOk := toFloat64(b)
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
	}
	return nil
}

func interpNumericDiv(a, b any, node *ast.Node) any {
	af, aOk := toFloat64(a)
	bf, bOk := toFloat64(b)
	if !aOk || !bOk {
		InterpErr(fmt.Sprintf("/ requires numeric operands, got %T and %T", a, b), node)
	}
	if bf == 0 {
		InterpErr("division by zero", node)
	}
	res := af / bf
	if math.IsNaN(res) || math.IsInf(res, 0) {
		InterpErr("division produced an invalid floating-point result", node)
	}
	return res
}

func interpNumericRelational(op string, a, b any, node *ast.Node) bool {
	cmp, numeric := compareNumeric(a, b)
	if !numeric {
		InterpErr(fmt.Sprintf("%s requires numeric operands, got %T and %T", op, a, b), node)
	}
	switch op {
	case "<":
		return cmp < 0
	case ">":
		return cmp > 0
	case "<=":
		return cmp <= 0
	case ">=":
		return cmp >= 0
	}
	return false
}

func checkedAddInt64(a, b int64, node *ast.Node) int64 {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		InterpErr("integer overflow", node)
	}
	return a + b
}

func checkedSubInt64(a, b int64, node *ast.Node) int64 {
	if (b > 0 && a < math.MinInt64+b) || (b < 0 && a > math.MaxInt64+b) {
		InterpErr("integer overflow", node)
	}
	return a - b
}

func checkedMulInt64(a, b int64, node *ast.Node) int64 {
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	// sign-extend the low 63 bits; the high word must equal either all 0s or all 1s of that extension
	expectedHi := uint64(lo>>63) * ^uint64(0)
	if hi != expectedHi {
		InterpErr("integer overflow", node)
	}
	return int64(lo)
}

// compareNumeric compares two numeric values. It returns the signed comparison
// result (-1, 0, 1) and true when both operands are numeric. Mixed comparisons
// never round an exact integer through float64; they are compared as int64 when
// the float operand is a whole number inside the int64 range.
func compareNumeric(a, b any) (int, bool) {
	ai, aInt := asInt64(a)
	bi, bInt := asInt64(b)
	if aInt && bInt {
		switch {
		case ai < bi:
			return -1, true
		case ai > bi:
			return 1, true
		}
		return 0, true
	}
	af, aFloat := toFloat64(a)
	bf, bFloat := toFloat64(b)
	if !aFloat || !bFloat {
		return 0, false
	}
	// Mixed int/float: keep precision by comparing as integers when possible.
	if aInt && bFloat && floatIsWholeInt64(bf) {
		bi2 := int64(bf)
		switch {
		case ai < bi2:
			return -1, true
		case ai > bi2:
			return 1, true
		}
		return 0, true
	}
	if aFloat && bInt && floatIsWholeInt64(af) {
		ai2 := int64(af)
		switch {
		case ai2 < bi:
			return -1, true
		case ai2 > bi:
			return 1, true
		}
		return 0, true
	}
	switch {
	case af < bf:
		return -1, true
	case af > bf:
		return 1, true
	}
	return 0, true
}

func floatIsWholeInt64(f float64) bool {
	if math.IsNaN(f) || math.IsInf(f, 0) || f < math.MinInt64 || f > math.MaxInt64 {
		return false
	}
	return math.Trunc(f) == f
}

func toFloat64(v any) (float64, bool) {
	switch t := v.(type) {
	case int64:
		return float64(t), true
	case float64:
		return t, true
	case int:
		return float64(t), true
	}
	return 0, false
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
	case int:
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
	if eq, ok := numericEqual(a, b); ok {
		return eq
	}
	return a == b
}

// numericEqual compares two numeric operands by value regardless of their Go
// representation. The VM produces float64 for literals, arithmetic and JSON,
// but int64 for list_len and time_now, so interface equality would report
// int64(0) != float64(0). ok is false when either operand is not numeric.
func numericEqual(a, b any) (eq bool, ok bool) {
	ai, aInt := asInt64(a)
	bi, bInt := asInt64(b)
	if aInt && bInt {
		return ai == bi, true
	}
	af, aFloat := a.(float64)
	bf, bFloat := b.(float64)
	switch {
	case aFloat && bFloat:
		return af == bf, true
	case aInt && bFloat:
		return intEqualsFloat(ai, bf), true
	case aFloat && bInt:
		return intEqualsFloat(bi, af), true
	}
	return false, false
}

func asInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case int64:
		return t, true
	case int:
		return int64(t), true
	}
	return 0, false
}

// intEqualsFloat is exact: it never rounds the integer through float64, so
// 2^53+1 is not equal to 2^53.
func intEqualsFloat(i int64, f float64) bool {
	if f != math.Trunc(f) || f < -9.223372036854775808e18 || f >= 9.223372036854775808e18 {
		return false
	}
	return int64(f) == i
}

func SliceToAny(strs []string) []any {
	out := make([]any, len(strs))
	for i, s := range strs {
		out[i] = s
	}
	return out
}

type BCVM struct {
	receipt         *receiptRecorder
	receiptDecision *effectDecision

	ctx         context.Context
	allocations *allocationBudget
	prog        *bytecode.BCProgram
	stack       []any
	env         *BcEnv
	stores      *bcStoreRegistry
	ip          int
	insts       []bytecode.BCInstruction
	args        []string
	executed    int
	spawnDepth  int
	callDepth   int
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
	return runBytecodeEvidence(prog, cliArgs, policy, allowedCaps, in, out, errOut, traceLimit, nil)
}

func runBytecodeEvidence(prog *bytecode.BCProgram, cliArgs []string, policy ExecutionPolicy, allowedCaps []capability.Capability, in io.Reader, out io.Writer, errOut io.Writer, traceLimit int, recorder *receiptRecorder) (evidence bytecode.ExecutionEvidence) {
	ctx := context.Background()
	if policy.Deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, policy.Deadline)
		defer cancel()
	}
	vm := &BCVM{
		ctx: ctx, allocations: &allocationBudget{}, receipt: recorder,
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
			} else if ret, ok := r.(VmReturn); ok {
				// Top-level RETURN matches Interpret's returnSignal: an int64
				// becomes the process exit code; other values exit 0.
				if ret.val != nil {
					if num, ok := ret.val.(int64); ok {
						evidence.ExitCode = int(num)
					}
				}
			} else if vmerr, ok := r.(*VMError); ok {
				if nodeID, ok := prog.TrustedMainOriginAt(vmerr.Instruction); ok {
					vmerr.NodeID = nodeID
				}
				evidence.RuntimeFailure = &bytecode.RuntimeFailure{Code: vmerr.Code, Instruction: vmerr.Instruction, Opcode: vmerr.Opcode, NodeID: vmerr.NodeID, Message: vmerr.Message}
			} else {
				evidence.RuntimeFailure = &bytecode.RuntimeFailure{Code: "VM_INTERNAL", Instruction: vm.ip, Message: fmt.Sprintf("%v", r)}
			}
		}
		if recorder != nil {
			recorder.finish(vm.executed)
		}
		bytecode.SealExecutionEvidence(prog, &evidence)
	}()

	vm.run(vm.insts, vm.env)
	return evidence
}

func writeRuntimeFailure(errOut io.Writer, failure *bytecode.RuntimeFailure) {
	if errOut == nil {
		errOut = os.Stderr
	}
	fmt.Fprintln(errOut, mustVMErrorString(failure))
}

func mustVMErrorString(failure *bytecode.RuntimeFailure) string {
	return (&VMError{Phase: "runtime", Code: failure.Code, Function: "main", Instruction: failure.Instruction, Opcode: failure.Opcode, NodeID: failure.NodeID, Message: failure.Message}).Error()
}

func (vm *BCVM) push(v any) {
	vm.stack = append(vm.stack, v)
}

// truncateStack drops operands above height. It never grows the stack, so a
// frame that already consumed below height is left for pop to report.
func (vm *BCVM) truncateStack(height int) {
	if height >= 0 && len(vm.stack) > height {
		clear(vm.stack[height:])
		vm.stack = vm.stack[:height]
	}
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
			vm.recordEffect(cap, op, "allowed")
			return
		}
	}
	vm.recordEffect(cap, op, "denied")
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

// popCheckedString pops a value and asserts it is a string, panicking with a
// structured TYPE_ERROR labelled by msgPrefix otherwise. Shared by the
// file/network instructions' operand checks to avoid repeating the
// pop/assert/panic triple per operand.
func (vm *BCVM) popCheckedString(inst bytecode.BCInstruction, ip int, msgPrefix string) string {
	val := vm.pop(inst.Op)
	s, ok := val.(string)
	if !ok {
		panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, msgPrefix+", got %T", val))
	}
	return s
}

// requireHTTPMux fetches and type-checks the "__http_mux" environment
// variable the HTTP routing instructions share, panicking with a structured
// error labelled by opName if the server was never started or the stored
// value is not a *http.ServeMux. Shared by every instruction that needs the
// active mux.
func requireHTTPDispatch(env *BcEnv, ip int, inst bytecode.BCInstruction, opName string) *httpreq.Server {
	raw, ok := env.get("__http_dispatch")
	if !ok {
		panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "http server not started"))
	}
	server, ok := raw.(*httpreq.Server)
	if !ok {
		panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, opName+" expected request dispatcher, got %T", raw))
	}
	return server
}

func panicRequestRead(ip int, op bytecode.Opcode, err error) {
	code := "RUNTIME_ERROR"
	msg := err.Error()
	if he, ok := err.(*httpreq.Error); ok {
		code = he.Code
		msg = he.Message
	}
	panic(NewRuntimeError(code, "main", ip, op, "%s", msg))
}

func (vm *BCVM) readRequestField(ip int, inst bytecode.BCInstruction, kind string) string {
	nameVal := vm.pop(inst.Op)
	recvVal := vm.pop(inst.Op)
	var value string
	var err error
	switch kind {
	case "req_header":
		value, err = httpreq.Header(recvVal, nameVal)
	case "req_path":
		value, err = httpreq.Path(recvVal, nameVal)
	default:
		value, err = httpreq.Query(recvVal, nameVal)
	}
	if err != nil {
		panicRequestRead(ip, inst.Op, err)
	}
	return value
}

// bytesToAnySlice converts a []byte into a []any of float64 elements, the
// representation HowlFrame values use for byte data on the operand stack.
// Shared by every instruction that returns raw bytes (fetch responses, file
// reads, subprocess output) to avoid repeating the same conversion loop.
func bytesToAnySlice(b []byte) []any {
	var bytesAny []any
	for _, bb := range b {
		bytesAny = append(bytesAny, float64(bb))
	}
	return bytesAny
}

// popCheckedStatus pops a value and converts it to an int HTTP status code,
// accepting float64/int64/int and panicking with a structured TYPE_ERROR
// labelled by opName otherwise. Shared by the response instructions.
func (vm *BCVM) popCheckedStatus(inst bytecode.BCInstruction, ip int, opName string) int {
	statusVal := vm.pop(inst.Op)
	switch s := statusVal.(type) {
	case float64:
		return int(s)
	case int64:
		return int(s)
	case int:
		return s
	default:
		panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, opName+" expected number status, got %T", statusVal))
	}
}

// bytesFromNumberList converts a []any of numeric elements (float64/int64/int)
// into a []byte, panicking with a structured TYPE_ERROR labelled by opName on
// any non-numeric element. Shared by write_file, parse_json and bytes_to_string.
func bytesFromNumberList(items []any, ip int, inst bytecode.BCInstruction, opName string) []byte {
	var data []byte
	for _, bb := range items {
		var bNum float64
		switch n := bb.(type) {
		case float64:
			bNum = n
		case int64:
			bNum = float64(n)
		case int:
			bNum = float64(n)
		default:
			panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, opName+" byte list element expected number, got %T", bb))
		}
		data = append(data, byte(bNum))
	}
	return data
}

func (vm *BCVM) run(insts []bytecode.BCInstruction, env *BcEnv) any {
	if vm.ctx == nil {
		vm.ctx = context.Background()
	}
	if vm.allocations == nil {
		vm.allocations = &allocationBudget{}
	}
	if vm.stores == nil {
		vm.stores = newBCStoreRegistry()
	}

	ip := 0
	// loopBases records the operand stack height at each active FOR_INIT in
	// this frame. FOR_NEXT trims anything the loop body left behind, so a
	// discarded value can never be mistaken for the iterator state.
	var loopBases []int
	for ip < len(insts) {
		vm.ip = ip
		inst := insts[ip]
		vm.checkDeadline(ip, inst.Op)
		vm.trace.record(ip, inst, vm.prog)
		// MaxInstructions is the maximum number of instructions allowed. Check
		// before incrementing so an exact budget succeeds and even MaxInt cannot
		// overflow into effectively unlimited execution.
		if vm.Limits.MaxInstructions <= 0 || vm.executed >= vm.Limits.MaxInstructions {
			panic(NewRuntimeError("LIMIT_EXCEEDED", "main", vm.ip, inst.Op, "instruction limit exceeded"))
		}
		vm.executed++
		if vm.receipt != nil {
			vm.receiptDecision = &effectDecision{summary: vm.effectSummary(inst, env)}
		}

		spec := bytecode.Registry[inst.Op]
		if spec.Capability != capability.None {
			vm.requireCapability(spec.Capability, inst.Op)
		}

		switch inst.Op {

		case bytecode.OpTryLet:
			varName := inst.StringOperand
			errVar := inst.StringOperand2
			remaining := int64(len(insts) - ip - 1)
			for segment, length := range []int64{inst.IntOperand, inst.IntOperand2, inst.IntOperand3} {
				if length < 0 || length > remaining {
					panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "try_let %s length %d out of range [0,%d]", [...]string{"value", "catch", "success"}[segment], length, remaining))
				}
				remaining -= length
			}
			valLen := int(inst.IntOperand)
			catchLen := int(inst.IntOperand2)
			successLen := int(inst.IntOperand3)

			valInsts := insts[ip+1 : ip+1+valLen]
			catchInsts := insts[ip+1+valLen : ip+1+valLen+catchLen]
			successInsts := insts[ip+1+valLen+catchLen : ip+1+valLen+catchLen+successLen]

			var tryErr error
			tryHeight := len(vm.stack)
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
				// Drop partial operands the failed expression pushed.
				vm.truncateStack(tryHeight)
				env.vars[errVar] = tryErr.Error()
				vm.run(catchInsts, env)
			} else {
				env.vars[varName] = vm.pop(inst.Op)
				vm.truncateStack(tryHeight)
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
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "db_connect failed: %v", err))
			}
			env.vars[varName] = db
		case bytecode.OpSqlQuery:
			dbVar := inst.StringOperand
			queryStr := inst.StringOperand2
			dbAny, ok := env.get(dbVar)
			if !ok {
				panic(NewRuntimeError("UNDEFINED_VAR", "main", ip, inst.Op, "undefined db: %s", dbVar))
			}
			db, ok := dbAny.(*sql.DB)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "sql_query expected *sql.DB, got %T", dbAny))
			}
			rows, err := db.Query(queryStr)
			if err != nil {
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "sql_query failed: %v", err))
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
		case bytecode.OpStoreKeys:
			store := vm.storeHandle(env, inst.StringOperand, inst.Op)
			if store.file != "" {
				vm.requireCapability(capability.Filesystem, inst.Op)
			}
			store.mu.RLock()
			keys := make([]any, 0, len(store.records))
			for key := range store.records {
				keys = append(keys, key)
			}
			store.mu.RUnlock()
			// Go randomizes map iteration order. Sorting keeps enumeration
			// deterministic, which every caller listing records depends on.
			sort.Slice(keys, func(i, j int) bool {
				return keys[i].(string) < keys[j].(string)
			})
			vm.push(keys)
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
			method := vm.popCheckedString(inst, ip, "fetch expected string method")
			urlStr := vm.popCheckedString(inst, ip, "fetch expected string url")
			max := vm.Limits.MaxFetchBodyBytes
			if max <= 0 {
				panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, inst.Op, "fetch body exceeds maximum limit of %d bytes", max))
			}
			req, err := http.NewRequestWithContext(vm.ctx, method, urlStr, nil)
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "fetch request creation failed: %v", err))
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "fetch failed: %v", err))
			}
			defer resp.Body.Close()
			readLimit := int64(max)
			if readLimit < math.MaxInt64 {
				readLimit++
			}
			b, err := io.ReadAll(io.LimitReader(resp.Body, readLimit))
			vm.checkDeadline(ip, inst.Op)
			if len(b) > max {
				panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, inst.Op, "fetch body exceeds maximum limit of %d bytes", max))
			}
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "fetch body read failed: %v", err))
			}
			vm.chargeBytes(len(b), ip, inst.Op)
			vm.push(bytesToAnySlice(b))
		case bytecode.OpReadFile:
			path := vm.popCheckedString(inst, ip, "read_file expected string")
			b, err := os.ReadFile(path)
			if err != nil {
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "read_file failed: %v", err))
			}
			vm.chargeBytes(len(b), ip, inst.Op)
			vm.push(bytesToAnySlice(b))
		case bytecode.OpWriteFile:
			dataAny := vm.pop(inst.Op)
			path := vm.popCheckedString(inst, ip, "write_file expected string path")
			var data []byte
			switch v := dataAny.(type) {
			case string:
				data = []byte(v)
			case []byte:
				data = v
			case []any:
				data = bytesFromNumberList(v, ip, inst, "write_file")
			default:
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "write_file expected string or byte list data, got %T", dataAny))
			}
			err := os.WriteFile(path, data, 0644)
			if err != nil {
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "write_file failed: %v", err))
			}
		case bytecode.OpMkdir:
			path := vm.popCheckedString(inst, ip, "mkdir expected string")
			err := os.MkdirAll(path, 0755)
			if err != nil {
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "mkdir failed: %v", err))
			}
		case bytecode.OpExec:
			numArgs := int(inst.IntOperand)
			var args []string
			for i := 0; i < numArgs; i++ {
				args = append([]string{fmt.Sprint(vm.pop(inst.Op))}, args...)
			}
			cmdStr := fmt.Sprint(vm.pop(inst.Op))
			max := vm.Limits.MaxExecOutputBytes
			if max <= 0 {
				panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, inst.Op, "exec output exceeds maximum limit of %d bytes", max))
			}
			cmdCtx, cancel := context.WithCancel(vm.ctx)
			cmd := exec.CommandContext(cmdCtx, cmdStr, args...)
			output := &limitedOutput{max: max, cancel: cancel}
			cmd.Stdout, cmd.Stderr = output, output
			// Bound inherited pipe waits when the runner has a deadline.
			if vm.ctx.Done() != nil {
				cmd.WaitDelay = 100 * time.Millisecond
			}
			err := cmd.Run()
			cancel()
			vm.checkDeadline(ip, inst.Op)
			if output.exceeded {
				panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, inst.Op, "exec output exceeds maximum limit of %d bytes", max))
			}
			out := output.buffer.Bytes()
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "exec failed: %v", err))
			}
			vm.chargeBytes(len(out), ip, inst.Op)
			vm.push(bytesToAnySlice(out))
		case bytecode.OpParseJson:
			bodyVar := inst.StringOperand
			var data []byte
			if bodyVar == "req.body" {
				reqAny, ok := env.get("req")
				if !ok {
					panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "undefined req"))
				}
				req, ok := reqAny.(*http.Request)
				if !ok {
					panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "parse_json expected *http.Request, got %T", reqAny))
				}
				if req.Body == nil {
					panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "request body is nil"))
				}
				// Bound request body read to 10MB to prevent unbounded memory DoS
				const maxBodyBytes = 10 * 1024 * 1024
				limitedReader := io.LimitReader(req.Body, maxBodyBytes+1)
				readData, err := io.ReadAll(limitedReader)
				if err != nil {
					panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "failed to read request body: %v", err))
				}
				if len(readData) > maxBodyBytes {
					panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, inst.Op, "request body exceeds maximum limit of %d bytes", maxBodyBytes))
				}
				data = readData
				req.Body = io.NopCloser(bytes.NewBuffer(data))
			} else {
				bodyAny, ok := env.get(bodyVar)
				if !ok {
					panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "undefined var: "+bodyVar))
				}
				if s, ok := bodyAny.(string); ok {
					data = []byte(s)
				} else if b, ok := bodyAny.([]byte); ok {
					data = b
				} else if b, ok := bodyAny.([]any); ok {
					data = bytesFromNumberList(b, ip, inst, "parse_json")
				}
			}
			result, err := decodeJSONValue(data)
			if err != nil {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "parse_json failed: %v", err))
			}
			vm.push(result)
		case bytecode.OpSpawn:
			remaining := len(insts) - ip - 1
			if inst.IntOperand < 0 || inst.IntOperand > int64(remaining) {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "spawn body length %d out of range [0,%d]", inst.IntOperand, remaining))
			}
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
				childVM := &BCVM{ctx: vm.ctx, allocations: vm.allocations, receipt: vm.receipt, prog: vm.prog, env: cEnv, stores: vm.stores, Limits: vm.Limits, AllowedCaps: vm.AllowedCaps, Out: vm.Out, ErrOut: vm.ErrOut}
				childVM.run(bodyInsts, cEnv)
			}(capturedEnv)
			ip += bodyLen
		case bytecode.OpRes:
			bodyAny := vm.pop(inst.Op)
			contentTypeVal := vm.pop(inst.Op)
			contentType, ok := contentTypeVal.(string)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "res expected string content type, got %T", contentTypeVal))
			}
			status := vm.popCheckedStatus(inst, ip, "res")
			wAny, ok := env.get("w")
			if !ok {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "no response writer"))
			}
			w, ok := wAny.(http.ResponseWriter)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "res expected http.ResponseWriter, got %T", wAny))
			}
			w.Header().Set("Content-Type", contentType)
			w.WriteHeader(status)
			if s, ok := bodyAny.(string); ok {
				w.Write([]byte(s))
			} else if b, ok := bodyAny.([]byte); ok {
				w.Write(b)
			} else if b, ok := bodyAny.([]any); ok {
				w.Write(bytesFromNumberList(b, ip, inst, "res"))
			}
		case bytecode.OpResJson:
			data := vm.pop(inst.Op)
			status := vm.popCheckedStatus(inst, ip, "res_json")
			wAny, ok := env.get("w")
			if !ok {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "no response writer"))
			}
			w, ok := wAny.(http.ResponseWriter)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "res_json expected http.ResponseWriter, got %T", wAny))
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(data)
		case bytecode.OpHttpServerStart:
			port := inst.StringOperand
			mux := http.NewServeMux()
			env.vars["__http_mux"] = mux
			env.vars["__http_port"] = port
			env.vars["__http_dispatch"] = httpreq.NewServer(mux)
		case bytecode.OpHttpRoute:
			path := inst.StringOperand
			reqVar := inst.StringOperand2
			remaining := len(insts) - ip - 1
			if inst.IntOperand < 0 || inst.IntOperand > int64(remaining) {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "http_route body length %d out of range [0,%d]", inst.IntOperand, remaining))
			}
			bodyLen := int(inst.IntOperand)
			bodyInsts := insts[ip+1 : ip+1+bodyLen]

			dispatch := requireHTTPDispatch(env, ip, inst, "http_route")

			capturedEnv := NewBcEnv(nil)
			for e := env; e != nil; e = e.parent {
				for k, v := range e.vars {
					if _, exists := capturedEnv.vars[k]; !exists {
						capturedEnv.vars[k] = v
					}
				}
			}

			prog := vm.prog
			if err := dispatch.Handle(path, func(w http.ResponseWriter, r *http.Request) {
				recorder := &httpResponseRecorder{ResponseWriter: w}
				reqEnv := NewBcEnv(capturedEnv)
				reqEnv.vars["w"] = http.ResponseWriter(recorder)
				reqEnv.vars[reqVar] = r
				reqEnv.vars["req"] = r
				childVM := &BCVM{ctx: vm.ctx, allocations: vm.allocations, receipt: vm.receipt, prog: prog, env: reqEnv, stores: vm.stores, Limits: vm.Limits, AllowedCaps: vm.AllowedCaps, Out: vm.Out, ErrOut: vm.ErrOut}
				func() {
					defer func() {
						recovered := recover()
						if recovered == nil {
							return
						}
						// A panicking handler must not reach the client as a
						// success. Without an explicit response Go sends 200
						// with an empty body, which makes a CAPABILITY_DENIED
						// denial indistinguishable from a completed request.
						failure, ok := recovered.(*VMError)
						if !ok {
							failure = &VMError{
								Phase:   "runtime",
								Code:    "HANDLER_PANIC",
								Message: fmt.Sprint(recovered),
							}
						}
						fmt.Fprintln(vm.ErrOut, "HTTP handler failure:", failure.Error())
						if recorder.wrote {
							// The handler already committed a response; the
							// status line cannot be rewritten, so surface the
							// failure on the server side only.
							return
						}
						recorder.Header().Set("Content-Type", "application/json")
						recorder.WriteHeader(http.StatusInternalServerError)
						_ = json.NewEncoder(recorder).Encode(failure)
					}()
					childVM.run(bodyInsts, reqEnv)
				}()
			}); err != nil {
				panicRequestRead(ip, inst.Op, err)
			}
			ip += bodyLen
		case bytecode.OpHttpServerServe:
			dispatch := requireHTTPDispatch(env, ip, inst, "http_server_serve")
			portAny, ok := env.get("__http_port")
			if !ok {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "http server port not configured"))
			}
			port, ok := portAny.(string)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "http_server_serve expected string port, got %T", portAny))
			}
			fmt.Fprintln(vm.Out, "Listening on "+port)
			err := http.ListenAndServe(":"+port, dispatch)
			if err != nil {
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "http listen failed: %v", err))
			}
		case bytecode.OpHttpReqMethod:
			reqAny, ok := env.get("req")
			if !ok {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "no request context"))
			}
			req, ok := reqAny.(*http.Request)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "http_req_method expected *http.Request, got %T", reqAny))
			}
			vm.push(req.Method)
		case bytecode.OpHttpReqQuery:
			vm.push(vm.readRequestField(ip, inst, "req_query"))
		case bytecode.OpHttpReqHeader:
			vm.push(vm.readRequestField(ip, inst, "req_header"))
		case bytecode.OpHttpReqPath:
			vm.push(vm.readRequestField(ip, inst, "req_path"))
		case bytecode.OpHttpResHeader:
			valueVal := vm.pop(inst.Op)
			nameVal := vm.pop(inst.Op)
			name, okName := nameVal.(string)
			if !okName {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "http_res_header expected string name, got %T", nameVal))
			}
			value, okVal := valueVal.(string)
			if !okVal {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "http_res_header expected string value, got %T", valueVal))
			}
			wAny, ok := env.get("w")
			if !ok {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "no response writer"))
			}
			w, ok := wAny.(http.ResponseWriter)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "http_res_header expected http.ResponseWriter, got %T", wAny))
			}
			w.Header().Set(name, value)
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
			resp, err := vm.postModel("http://localhost:11434/api/generate", reqBody, ip, inst.Op)
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "neural_circuit request failed: %v", err))
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "neural_circuit response decode failed: %v", err))
			}
			vm.chargeAlloc(len(res.Response), ip, inst.Op)
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
			createResp, err := vm.postModel("http://localhost:11434/api/create", createReq, ip, inst.Op)
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "ephemeral_circuit model create failed: %v", err))
			}
			createResp.Body.Close()

			func() {
				delReq, _ := json.Marshal(map[string]any{"name": modelName})
				req, _ := http.NewRequestWithContext(vm.ctx, "DELETE", "http://localhost:11434/api/delete", bytes.NewReader(delReq))
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
			resp, err := vm.postModel("http://localhost:11434/api/generate", reqBody, ip, inst.Op)
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "ephemeral_circuit request failed: %v", err))
			}
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "ephemeral_circuit response decode failed: %v", err))
			}
			resp.Body.Close()
			vm.chargeAlloc(len(res.Response), ip, inst.Op)
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
			resp, err := vm.postModel("http://localhost:11434/api/generate", reqBody, ip, inst.Op)
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "achieve request failed: %v", err))
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "achieve response decode failed: %v", err))
			}
			vm.chargeAlloc(len(res.Response), ip, inst.Op)
			vm.push(res.Response)

		case bytecode.OpConfidence:
			promptStrAny := vm.pop(inst.Op)
			promptStr := fmt.Sprintf("%v", promptStrAny)
			reqBody, _ := json.Marshal(map[string]any{
				"model":  "llama3",
				"prompt": "Evaluate the probability of this statement being true. Return ONLY a float between 0.0 and 1.0. Statement: " + promptStr,
				"stream": false,
			})
			resp, err := vm.postModel("http://localhost:11434/api/generate", reqBody, ip, inst.Op)
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "confidence request failed: %v", err))
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "confidence response decode failed: %v", err))
			}
			val, _ := strconv.ParseFloat(strings.TrimSpace(res.Response), 64)
			vm.push(val)

		case bytecode.OpLlmGenerate:
			modelStr := inst.StringOperand
			promptVal := vm.pop(inst.Op)
			promptStr, ok := promptVal.(string)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "llm_generate expected string prompt, got %T", promptVal))
			}
			reqBody, _ := json.Marshal(map[string]any{
				"model":  modelStr,
				"prompt": promptStr,
				"stream": false,
			})
			resp, err := vm.postModel("http://localhost:11434/api/generate", reqBody, ip, inst.Op)
			if err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "llm_generate request failed: %v", err))
			}
			defer resp.Body.Close()
			var res struct {
				Response string `json:"response"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				vm.checkDeadline(ip, inst.Op)
				panic(NewRuntimeError("IO_ERROR", "main", ip, inst.Op, "llm_generate response decode failed: %v", err))
			}
			vm.chargeAlloc(len(res.Response), ip, inst.Op)
			vm.push(res.Response)

		case bytecode.OpLoadConst:
			vm.push(inst.ValueOperand)
		case bytecode.OpLoadVar:
			name := inst.StringOperand
			vm.trace.state("var:"+name, "value")
			if val, ok := env.get(name); ok {
				vm.push(val)
			} else {
				panic(NewRuntimeError("UNDEFINED_VAR", "main", ip, inst.Op, "undefined variable: %s", name))
			}
		case bytecode.OpStoreVar:
			name := inst.StringOperand
			vm.trace.state("var:"+name, "value")
			env.vars[name] = vm.pop(inst.Op)
		case bytecode.OpSetVar:
			name := inst.StringOperand
			vm.trace.state("var:"+name, "value")
			if !env.set(name, vm.pop(inst.Op)) {
				panic(NewRuntimeError("UNDEFINED_VAR", "main", ip, inst.Op, "undefined variable: %s", name))
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
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "for requires a list, got %T", list))
			}
			loopBases = append(loopBases, len(vm.stack))
			vm.push(items)
			vm.push(0.0) // index
		case bytecode.OpForNext:
			varName := inst.StringOperand
			offset := int(inst.IntOperand)

			if n := len(loopBases); n > 0 {
				vm.truncateStack(loopBases[n-1] + 2)
			}
			idxAny := vm.pop(inst.Op)
			itemsAny := vm.pop(inst.Op)

			items, ok := itemsAny.([]any)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "for requires a list, got %T", itemsAny))
			}
			idx, err := ToInt(idxAny)
			if err != nil {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "for index must be a number, got %T", idxAny))
			}
			if f, ok := idxAny.(float64); ok && float64(int64(f)) != f {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "for index must be an integer, got %v", f))
			}

			if idx < 0 || idx > int64(len(items)) {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "for index %d out of range [0,%d)", idx, len(items)))
			}
			if idx == int64(len(items)) {
				if n := len(loopBases); n > 0 {
					loopBases = loopBases[:n-1]
				}
				ip += offset
				continue
			}

			env.vars[varName] = items[idx]
			vm.push(items)
			vm.push(float64(idx + 1))
		case bytecode.OpCall:
			funcName := inst.StringOperand
			numArgs := int(inst.IntOperand)

			fn, ok := vm.prog.Functions[funcName]
			if !ok {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "undefined function: %s", funcName))
			}

			if fn.LazySynthesize {
				vm.requireCapability(capability.Network, inst.Op)

				promptStr := fmt.Sprintf("You are a HowlFrame compiler. Synthesize the HowlFrame Lisp code for the function '%s' with parameters %v. Docstring: \"%s\"\n\nReply ONLY with the HowlFrame Lisp code for the function body expressions. Do not include (defun ...). Do not include markdown formatting.\nFor example, if the docstring says \"Returns the sum of a and b\", you reply:\n(+ a b)", fn.Name, fn.Params, fn.Docstring)
				reqBody, _ := json.Marshal(map[string]any{
					"model":  "llama3",
					"prompt": promptStr,
					"stream": false,
				})
				resp, err := vm.postModel("http://localhost:11434/api/generate", reqBody, ip, inst.Op)
				if err != nil {
					vm.checkDeadline(ip, inst.Op)
					panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "lazy synthesis network request failed: %v", err))
				}
				var res struct {
					Response string `json:"response"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
					resp.Body.Close()
					vm.checkDeadline(ip, inst.Op)
					panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "lazy synthesis response decode failed: %v", err))
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

				var newProg *bytecode.BCProgram
				func() {
					defer func() {
						if r := recover(); r != nil {
							vm.checkDeadline(ip, inst.Op)
							panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "lazy synthesis compile failed: %v", r))
						}
					}()
					newProg = bytecode.CompileToBytecode(astNode)
				}()
				synthFn, ok := newProg.Functions[fn.Name]
				if !ok {
					vm.checkDeadline(ip, inst.Op)
					panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "lazy synthesis did not produce function: %s", fn.Name))
				}
				fn.Instructions = synthFn.Instructions
				fn.LazySynthesize = false
			}

			if numArgs != len(fn.Params) {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "%s expects %d argument(s), got %d", funcName, len(fn.Params), numArgs))
			}

			var argVals []any
			for i := 0; i < numArgs; i++ {
				argVals = append([]any{vm.pop(inst.Op)}, argVals...)
			}

			if vm.Limits.MaxCallDepth <= 0 || vm.callDepth >= vm.Limits.MaxCallDepth {
				panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, inst.Op, "call depth limit exceeded (max %d)", vm.Limits.MaxCallDepth))
			}
			vm.callDepth++

			callEnv := NewBcEnv(env)
			for i, p := range fn.Params {
				callEnv.vars[p] = argVals[i]
			}

			// The callee shares the operand stack. Restore the caller's height
			// after it returns, so values it discarded, or loop state left by a
			// return from inside a for, never leak into the caller's operands.
			callerHeight := len(vm.stack)
			var result any
			func() {
				defer func() { vm.callDepth-- }()
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
			vm.truncateStack(callerHeight)
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
			result := BcBinop(op, a, b)
			if str, ok := result.(string); ok {
				vm.chargeAlloc(len(str), ip, inst.Op)
			}
			vm.push(result)
		case bytecode.OpConvert:
			a := vm.pop(inst.Op)
			target := inst.StringOperand
			converted := BcConvert(target, a)
			if str, ok := converted.(string); ok {
				vm.chargeAlloc(len(str), ip, inst.Op)
			}
			vm.push(converted)
		case bytecode.OpStrSplit:
			sepVal := vm.pop(inst.Op)
			sVal := vm.pop(inst.Op)
			sep, ok1 := sepVal.(string)
			s, ok2 := sVal.(string)
			if !ok1 || !ok2 {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "str_split expected string, got %T and %T", sVal, sepVal))
			}
			parts := strings.Split(s, sep)
			vm.chargeSlots(len(parts), 8, ip, inst.Op)
			for _, part := range parts {
				vm.chargeAlloc(len(part), ip, inst.Op)
			}
			vm.push(BcSliceToAny(parts))
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
			joined := strings.Join(strs, sep)
			vm.chargeAlloc(len(joined), ip, inst.Op)
			vm.push(joined)
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
			vm.chargeSlots(num, 8, ip, inst.Op)
			items := make([]any, num)
			for i := 0; i < num; i++ {
				items[num-1-i] = vm.pop(inst.Op)
				if str, ok := items[num-1-i].(string); ok {
					vm.chargeAlloc(len(str), ip, inst.Op)
				}
			}
			vm.push(items)
		case bytecode.OpMakeDict:
			num := int(inst.IntOperand)
			dict := make(map[string]any)
			// pop pairs: value then key
			pairs := make([]any, num*2)
			for i := 0; i < num*2; i++ {
				pairs[num*2-1-i] = vm.pop(inst.Op)
			}
			for i := 0; i < num*2; i += 2 {
				key := fmt.Sprint(pairs[i])
				dict[key] = pairs[i+1]
			}
			vm.chargeSlots(len(dict), 16, ip, inst.Op)
			for key := range dict {
				vm.chargeAlloc(len(key), ip, inst.Op)
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
			// Approximate list storage as eight bytes per slot, plus new string payload.
			vm.chargeSlots(len(items)+1, 8, ip, inst.Op)
			if str, ok := item.(string); ok {
				vm.chargeAlloc(len(str), ip, inst.Op)
			}
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
			if _, exists := dict[key]; !exists {
				vm.chargeAlloc(16, ip, inst.Op)
				vm.chargeAlloc(len(key), ip, inst.Op)
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
			// A named operand reads that variable and pops only the key.
			// An empty operand pops the dict value as well, so a nested
			// map_get is a path read. Both forms grant nothing.
			keyAny := vm.pop(inst.Op)
			key := fmt.Sprint(keyAny)
			varName := inst.StringOperand
			var current any
			if varName != "" {
				var ok bool
				current, ok = env.get(varName)
				if !ok {
					panic(NewRuntimeError("UNDEFINED_VAR", "main", ip, inst.Op, "undefined variable: %s", varName))
				}
			} else {
				current = vm.pop(inst.Op)
				varName = "value"
			}
			dict, ok := current.(map[string]any)
			if !ok {
				// A missing key is "". map_get of that sentinel is a type
				// error, not a second miss.
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
				// Missing keys are the empty string. A missing store record is nil.
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
		case bytecode.OpMapKeys:
			val := vm.pop(inst.Op)
			dict, ok := val.(map[string]any)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "map_keys expected dict, got %T", val))
			}
			keys := sortedMapKeys(dict)
			vm.chargeSlots(len(keys), 8, ip, inst.Op)
			vm.push(keys)
		case bytecode.OpIsNil:
			val := vm.pop(inst.Op)
			vm.push(val == nil)
		case bytecode.OpTimeNow:
			vm.push(int64(time.Now().Unix()))
		case bytecode.OpEncodeJson:
			val := vm.pop(inst.Op)
			b, err := json.Marshal(normalizeForJSON(val))
			if err != nil {
				panic(NewRuntimeError("ENCODE_JSON_ERROR", "main", ip, inst.Op, "%v", err))
			}
			vm.chargeAlloc(len(b), ip, inst.Op)
			vm.push(string(b))
		case bytecode.OpHTMLEscape, bytecode.OpAttrEscape:
			opName := "html_escape"
			if inst.Op == bytecode.OpAttrEscape {
				opName = "attr_escape"
			}
			val := vm.pop(inst.Op)
			s, ok := val.(string)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "%s expected string, got %T", opName, val))
			}
			escaped := htmlescape.Escape(s)
			vm.chargeAlloc(len(escaped), ip, inst.Op)
			vm.push(escaped)
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
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "cli_args index must be a number, got %T", idxAny))
			}
			if idx >= 0 && idx < len(vm.args) {
				vm.push(vm.args[idx])
			} else {
				vm.push("")
			}
		case bytecode.OpSleep:
			msAny := vm.pop(inst.Op)
			ms, err := ToInt(msAny)
			if err != nil {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "sleep requires number, got %T", msAny))
			}
			timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
			select {
			case <-timer.C:
			case <-vm.ctx.Done():
				timer.Stop()
				vm.checkDeadline(ip, inst.Op)
			}
		case bytecode.OpEnv:
			name := vm.popCheckedString(inst, ip, "env expected string name")
			vm.push(os.Getenv(name))
		case bytecode.OpTask:
			vm.push(inst.StringOperand)
		case bytecode.OpSpawnAgent:
			v := vm.pop(inst.Op)
			task, ok := v.(string)
			if !ok {
				panic(NewRuntimeError("TYPE_ERROR", "main", ip, inst.Op, "spawn_agent expected string task, got %T", v))
			}
			name := inst.StringOperand
			if vm.spawnDepth >= vm.Limits.MaxCallDepth {
				panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, inst.Op, "spawn nesting depth limit exceeded"))
			}
			remaining := len(insts) - ip - 1
			if inst.IntOperand < 0 || inst.IntOperand > int64(remaining) {
				panic(NewRuntimeError("RUNTIME_ERROR", "main", ip, inst.Op, "spawn_agent body length %d out of range [0,%d]", inst.IntOperand, remaining))
			}
			bodyLen := int(inst.IntOperand)
			stackHeight := len(vm.stack)
			failed := false
			fmt.Fprintf(vm.Out, "[Swarm VM] Spawning agent %q for task: %q\n", name, task)
			if bodyLen > 0 {
				bodyInsts := insts[ip+1 : ip+1+bodyLen]
				capturedEnv := NewBcEnv(nil)
				for e := env; e != nil; e = e.parent {
					for k, v := range e.vars {
						if _, exists := capturedEnv.vars[k]; !exists {
							capturedEnv.vars[k] = cloneStoreValue(v)
						}
					}
				}
				// Synchronous children share the buffered reader to preserve stdin order.
				childVM := &BCVM{ctx: vm.ctx, allocations: vm.allocations, receipt: vm.receipt, prog: vm.prog, env: capturedEnv, args: vm.args, In: vm.In, lineReader: vm.lineReader, stores: vm.stores, Limits: vm.Limits, AllowedCaps: vm.AllowedCaps, Out: vm.Out, ErrOut: vm.ErrOut, executed: vm.executed, spawnDepth: vm.spawnDepth + 1, callDepth: vm.callDepth}
				func() {
					defer func() {
						// Account for all child work even when it exits through a panic.
						vm.executed = childVM.executed
						vm.truncateStack(stackHeight)
						if r := recover(); r != nil {
							switch e := r.(type) {
							case VmReturn:
								fmt.Fprintf(vm.ErrOut, "[Swarm VM] Agent %q failed task: %q: RETURN: return from spawn agent body\n", name, task)
							case VmExit:
								fmt.Fprintf(vm.ErrOut, "[Swarm VM] Agent %q failed task: %q: EXIT: exit %d from spawn agent body\n", name, task, e.code)
							case *VMError:
								if e.Code == "LIMIT_EXCEEDED" {
									panic(r)
								}
								fmt.Fprintf(vm.ErrOut, "[Swarm VM] Agent %q failed task: %q: %s: %s\n", name, task, e.Code, e.Message)
							default:
								fmt.Fprintf(vm.ErrOut, "[Swarm VM] Agent %q failed task: %q: VM_INTERNAL: %v\n", name, task, r)
							}
							failed = true
						}
					}()
					childVM.run(bodyInsts, capturedEnv)
				}()
			}
			if !failed {
				fmt.Fprintf(vm.Out, "[Swarm VM] Agent %q completed task: %q\n", name, task)
			}
			ip += bodyLen
		default:
			panic(NewRuntimeError("VM_INTERNAL", "main", ip, inst.Op, "unknown opcode: %s", bytecode.Registry[inst.Op].Name))
		}
		ip++
	}
	return nil
}

func BcToBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	panic(NewRuntimeError("TYPE_ERROR", "main", 0, 0, "expected boolean, got %T", v))
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
	switch op {
	case "+", "-", "*":
		ai, aIsInt := asInt64(a)
		bi, bIsInt := asInt64(b)
		if aIsInt && bIsInt {
			switch op {
			case "+":
				return bcCheckedAddInt64(ai, bi)
			case "-":
				return bcCheckedSubInt64(ai, bi)
			case "*":
				return bcCheckedMulInt64(ai, bi)
			}
		}
		af, aOk := toFloat64(a)
		bf, bOk := toFloat64(b)
		if !aOk || !bOk {
			panic(NewRuntimeError("TYPE_ERROR", "main", 0, 0, "%s requires numeric operands, got %T and %T", op, a, b))
		}
		switch op {
		case "+":
			return af + bf
		case "-":
			return af - bf
		case "*":
			return af * bf
		}
	case "/":
		af, aOk := toFloat64(a)
		bf, bOk := toFloat64(b)
		if !aOk || !bOk {
			panic(NewRuntimeError("TYPE_ERROR", "main", 0, 0, "/ requires numeric operands, got %T and %T", a, b))
		}
		if bf == 0 {
			panic(NewRuntimeError("RUNTIME_ERROR", "main", 0, 0, "division by zero"))
		}
		res := af / bf
		if math.IsNaN(res) || math.IsInf(res, 0) {
			panic(NewRuntimeError("RUNTIME_ERROR", "main", 0, 0, "division produced an invalid floating-point result"))
		}
		return res
	case "<", ">", "<=", ">=":
		cmp, numeric := compareNumeric(a, b)
		if !numeric {
			panic(NewRuntimeError("TYPE_ERROR", "main", 0, 0, "%s requires numeric operands, got %T and %T", op, a, b))
		}
		switch op {
		case "<":
			return cmp < 0
		case ">":
			return cmp > 0
		case "<=":
			return cmp <= 0
		case ">=":
			return cmp >= 0
		}
	}
	panic(NewRuntimeError("VM_INTERNAL", "main", 0, 0, "unknown binop: %s", op))
}

func bcCheckedAddInt64(a, b int64) int64 {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		panic(NewRuntimeError("RUNTIME_ERROR", "main", 0, 0, "integer overflow"))
	}
	return a + b
}

func bcCheckedSubInt64(a, b int64) int64 {
	if (b > 0 && a < math.MinInt64+b) || (b < 0 && a > math.MaxInt64+b) {
		panic(NewRuntimeError("RUNTIME_ERROR", "main", 0, 0, "integer overflow"))
	}
	return a - b
}

func bcCheckedMulInt64(a, b int64) int64 {
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	expectedHi := uint64(lo>>63) * ^uint64(0)
	if hi != expectedHi {
		panic(NewRuntimeError("RUNTIME_ERROR", "main", 0, 0, "integer overflow"))
	}
	return int64(lo)
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
	panic(NewRuntimeError("TYPE_ERROR", "main", 0, 0, "expected number, got %T", v))
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
	if eq, ok := numericEqual(a, b); ok {
		return eq
	}
	return a == b
}

func BcConvert(target string, a any) any {
	switch target {
	case "to_int":
		switch t := a.(type) {
		case float64:
			// 2^63 is exactly representable as float64 but does not fit int64.
			if math.IsNaN(t) || math.IsInf(t, 0) || t < -9223372036854775808.0 || t >= 9223372036854775808.0 {
				panic(NewRuntimeError("CONVERSION_ERROR", "main", 0, bytecode.OpConvert, "cannot convert %v to int", t))
			}
			return int64(t)
		case string:
			v, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
			if err != nil {
				panic(NewRuntimeError("CONVERSION_ERROR", "main", 0, bytecode.OpConvert, "cannot convert %q to int", t))
			}
			return v
		case int64:
			return t
		default:
			panic(NewRuntimeError("CONVERSION_ERROR", "main", 0, bytecode.OpConvert, "cannot convert %T to int", a))
		}
	case "to_float":
		switch t := a.(type) {
		case string:
			v, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
			if err != nil {
				panic(NewRuntimeError("CONVERSION_ERROR", "main", 0, bytecode.OpConvert, "cannot convert %q to float", t))
			}
			return v
		case float64:
			return t
		case int64:
			return float64(t)
		default:
			panic(NewRuntimeError("CONVERSION_ERROR", "main", 0, bytecode.OpConvert, "cannot convert %T to float", a))
		}
	case "to_string":
		return fmt.Sprint(a)
	case "bytes_to_string":
		items, ok := a.([]any)
		if !ok {
			panic(NewRuntimeError("TYPE_ERROR", "main", 0, bytecode.OpConvert, "bytes_to_string expected []any, got %T", a))
		}
		return string(bytesFromNumberList(items, 0, bytecode.BCInstruction{Op: bytecode.OpConvert}, "bytes_to_string"))
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

func decodeJSONValue(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values")
		}
		return nil, err
	}
	return normalizeJSONNumbers(value)
}

func normalizeJSONNumbers(value any) (any, error) {
	switch typed := value.(type) {
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer, nil
		}
		float, err := typed.Float64()
		if err != nil {
			return nil, err
		}
		return float, nil
	case []any:
		for index, item := range typed {
			normalized, err := normalizeJSONNumbers(item)
			if err != nil {
				return nil, err
			}
			typed[index] = normalized
		}
	case map[string]any:
		for key, item := range typed {
			normalized, err := normalizeJSONNumbers(item)
			if err != nil {
				return nil, err
			}
			typed[key] = normalized
		}
	}
	return value, nil
}

func normalizeForJSON(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeForJSON(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeForJSON(val)
		}
		return out
	case int64:
		return float64(t)
	default:
		return v
	}
}

// httpResponseRecorder tracks whether a route handler committed a response.
// The VM needs this to tell a handler that answered from one that panicked
// before writing anything, so only the latter is converted into a 500.
type httpResponseRecorder struct {
	http.ResponseWriter
	wrote bool
}

func (w *httpResponseRecorder) WriteHeader(status int) {
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *httpResponseRecorder) Write(b []byte) (int, error) {
	w.wrote = true
	return w.ResponseWriter.Write(b)
}

// Shared across child VMs; charges are cumulative and never reclaimed.
type allocationBudget struct {
	sync.Mutex
	memoryBytes int
}

func (vm *BCVM) chargeAlloc(n int, ip int, op bytecode.Opcode) {
	if n <= 0 {
		return
	}
	vm.allocations.Lock()
	defer vm.allocations.Unlock()
	max := vm.Limits.MaxMemoryBytes
	if max <= 0 || vm.allocations.memoryBytes > max-n {
		panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, op, "memory limit exceeded (max %d bytes)", max))
	}
	vm.allocations.memoryBytes += n
}

func (vm *BCVM) chargeSlots(n, size, ip int, op bytecode.Opcode) {
	// Avoid multiplication overflow before charging approximate storage.
	if n > 0 && n > int(^uint(0)>>1)/size {
		panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, op, "memory limit exceeded (max %d bytes)", vm.Limits.MaxMemoryBytes))
	}
	vm.chargeAlloc(n*size, ip, op)
}

func (vm *BCVM) chargeBytes(n, ip int, op bytecode.Opcode) {
	// Source bytes plus the converted any-slice representation (eight bytes/slot).
	vm.chargeAlloc(n, ip, op)
	vm.chargeSlots(n, 8, ip, op)
}

func (vm *BCVM) checkDeadline(ip int, op bytecode.Opcode) {
	if vm.ctx != nil && vm.ctx.Err() != nil {
		panic(NewRuntimeError("LIMIT_EXCEEDED", "main", ip, op, "wall-clock deadline exceeded"))
	}
}

func (vm *BCVM) postModel(url string, body []byte, ip int, op bytecode.Opcode) (*http.Response, error) {
	req, err := http.NewRequestWithContext(vm.ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	vm.checkDeadline(ip, op)
	return resp, err
}

// Returning an error stops copying; cancellation also stops a producing process.
// stdout and stderr share this writer, so os/exec serializes their writes.
type limitedOutput struct {
	buffer   bytes.Buffer
	max      int
	exceeded bool
	cancel   context.CancelFunc
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	if len(p) > b.max-b.buffer.Len() {
		b.exceeded = true
		b.cancel()
		return 0, io.ErrShortWrite
	}
	return b.buffer.Write(p)
}
