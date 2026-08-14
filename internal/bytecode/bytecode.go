package bytecode

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/howlcipher/howlframe/internal/ast"
	"strconv"
)

type BCInstruction struct {
	OpString string `json:"op"`
	Op       Opcode `json:"-"`

	StringOperand  string `json:"string_operand,omitempty"`
	StringOperand2 string `json:"string_operand_2,omitempty"`
	StringOperand3 string `json:"string_operand_3,omitempty"`
	IntOperand     int64  `json:"int_operand,omitempty"`
	IntOperand2    int64  `json:"int_operand_2,omitempty"`
	IntOperand3    int64  `json:"int_operand_3,omitempty"`
	ValueOperand   any    `json:"value_operand,omitempty"`

	// semanticOrigin is attached only by the direct HFIR lowerer. It is kept
	// private so it cannot alter the durable HFBC encoding or be supplied by a
	// bytecode artifact consumer.
	semanticOrigin string
}

func (inst *BCInstruction) UnmarshalJSON(data []byte) error {
	type Alias BCInstruction
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(inst),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	inst.Op, _ = NameToOpcode(inst.OpString)
	return nil
}

type BCProgram struct {
	Version   int                    `json:"version"`
	Functions map[string]*BCFunction `json:"functions"`
	Main      []BCInstruction        `json:"main"`

	// mainOrigins is an ephemeral direct-lowering sidecar. It is deliberately
	// unexported: provenance is useful only while the trusted lowerer program
	// remains in memory and is not a change to the HFBC artifact contract.
	mainOrigins              []string
	mainOriginHash           [32]byte
	mainLocalizationIdentity string
}

// ExecutionTraceEvent is runner-created execution evidence. Program code and
// model transport have no way to supply or widen it.
type ExecutionTraceEvent struct {
	Instruction int    `json:"instruction"`
	Opcode      string `json:"opcode"`
	NodeID      string `json:"node_id,omitempty"`
	BranchTaken *bool  `json:"branch_taken,omitempty"`
	Resource    string `json:"resource,omitempty"`
	StateKey    string `json:"state_key,omitempty"`
}

// MapStateEvent is an ephemeral, runner-created account of completed map
// operations. It never enters HFBC: it exists only inside sealed execution
// evidence for bounded HFIR localization.
type MapStateEvent struct {
	Sequence         uint64 `json:"sequence"`
	ResourceID       uint64 `json:"resource_id"`
	Instruction      int    `json:"instruction"`
	NodeID           string `json:"node_id"`
	Operation        string `json:"operation"`
	KeyFingerprint   string `json:"key_fingerprint,omitempty"`
	ValueFingerprint string `json:"value_fingerprint,omitempty"`
	VersionBefore    uint64 `json:"version_before"`
	VersionAfter     uint64 `json:"version_after"`
	Hit              bool   `json:"hit,omitempty"`
	DeletedExisting  bool   `json:"deleted_existing,omitempty"`
}

// ExpectedValueObservation is an opaque host-owned oracle token. Its private
// seal prevents JSON/model transport from inventing a value fingerprint.
type ExpectedValueObservation struct {
	Fingerprint string `json:"fingerprint"`
	Value       string `json:"value"`
	seal        [32]byte
}

// RuntimeFailure is a serializable projection of a VM failure for consumers
// that must not depend on the VM package.
type RuntimeFailure struct {
	Code        string `json:"code"`
	Instruction int    `json:"instruction"`
	Opcode      string `json:"opcode,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	Message     string `json:"message"`
}

// ExecutionEvidence is bounded trusted runner output. Trace limits are set by
// the caller, never by the program.
type ExecutionEvidence struct {
	ExitCode          int                   `json:"exit_code"`
	Trace             []ExecutionTraceEvent `json:"trace"`
	TraceTruncated    bool                  `json:"trace_truncated,omitempty"`
	RuntimeFailure    *RuntimeFailure       `json:"runtime_failure,omitempty"`
	MapStateEvents    []MapStateEvent       `json:"map_state_events,omitempty"`
	MapLedgerComplete bool                  `json:"map_ledger_complete,omitempty"`

	// seal is intentionally private. Only the VM can mint evidence accepted by
	// the localizer; copied, JSON-decoded, or changed evidence fails closed.
	seal [32]byte
}

var executionEvidenceKey = newExecutionEvidenceKey()
var expectedValueKey = newExecutionEvidenceKey()

// NewExpectedValueObservation mints an opaque token for a host-owned expected
// scalar. It is intentionally useful only to in-process trusted integrations;
// JSON-decoded or modified observations fail verification.
func NewExpectedValueObservation(value string) ExpectedValueObservation {
	fingerprint, _ := RuntimeValueFingerprint(value)
	observation := ExpectedValueObservation{Fingerprint: fingerprint, Value: value}
	observation.seal = expectedValueMAC(observation.Fingerprint + "\x00" + observation.Value)
	return observation
}

func TrustedExpectedValueObservation(observation ExpectedValueObservation) bool {
	expected := expectedValueMAC(observation.Fingerprint + "\x00" + observation.Value)
	return observation.Fingerprint != "" && hmac.Equal(observation.seal[:], expected[:])
}

// RuntimeKeyFingerprint mirrors map addressing, which coerces keys through
// fmt.Sprint before accessing map[string]any.
func RuntimeKeyFingerprint(key string) string {
	return runtimeFingerprint("map-key", key)
}

// RuntimeValueFingerprint admits only scalars whose runtime representation is
// stable enough for a bounded oracle comparison.
func RuntimeValueFingerprint(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return runtimeFingerprint("string", typed), true
	case int:
		return runtimeFingerprint("int", fmt.Sprint(typed)), true
	case int64:
		return runtimeFingerprint("int64", fmt.Sprint(typed)), true
	case float64:
		return runtimeFingerprint("float64", fmt.Sprint(typed)), true
	case bool:
		return runtimeFingerprint("bool", fmt.Sprint(typed)), true
	case nil:
		return runtimeFingerprint("nil", ""), true
	default:
		return "", false
	}
}

func runtimeFingerprint(domain, value string) string {
	digest := sha256.Sum256([]byte(domain + ":" + value))
	return fmt.Sprintf("sha256:%x", digest[:])
}

func expectedValueMAC(fingerprint string) [32]byte {
	mac := hmac.New(sha256.New, expectedValueKey[:])
	_, _ = mac.Write([]byte(fingerprint))
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

func newExecutionEvidenceKey() [32]byte {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		panic("bytecode execution evidence key: " + err.Error())
	}
	return key
}

// SealExecutionEvidence marks runner-created evidence as bound to the exact
// direct-lowered main program. It is not a durable artifact feature.
func SealExecutionEvidence(prog *BCProgram, evidence *ExecutionEvidence) {
	if prog == nil || evidence == nil || !hasTrustedMainOrigins(prog) {
		return
	}
	evidence.seal = executionEvidenceMAC(prog, *evidence)
}

// TrustedExecutionEvidence reports whether evidence was minted by this
// process's runner for an unmodified direct-lowered main program.
func TrustedExecutionEvidence(prog *BCProgram, evidence *ExecutionEvidence) bool {
	if prog == nil || evidence == nil || !hasTrustedMainOrigins(prog) {
		return false
	}
	expected := executionEvidenceMAC(prog, *evidence)
	return hmac.Equal(evidence.seal[:], expected[:])
}

func hasTrustedMainOrigins(prog *BCProgram) bool {
	return prog != nil && len(prog.Main) > 0 && len(prog.mainOrigins) == len(prog.Main) && prog.mainOriginHash == mainInstructionHash(prog.Main) && prog.mainLocalizationIdentity != ""
}

func executionEvidenceMAC(prog *BCProgram, evidence ExecutionEvidence) [32]byte {
	payload := struct {
		ProgramHash       [32]byte
		ExitCode          int
		Trace             []ExecutionTraceEvent
		TraceTruncated    bool
		RuntimeFailure    *RuntimeFailure
		MapStateEvents    []MapStateEvent
		MapLedgerComplete bool
	}{mainEvidenceHash(prog), evidence.ExitCode, evidence.Trace, evidence.TraceTruncated, evidence.RuntimeFailure, evidence.MapStateEvents, evidence.MapLedgerComplete}
	encoded, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, executionEvidenceKey[:])
	_, _ = mac.Write(encoded)
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

func mainEvidenceHash(prog *BCProgram) [32]byte {
	payload := struct {
		Instructions [32]byte
		Origins      []string
		Identity     string
	}{mainInstructionHash(prog.Main), prog.mainOrigins, prog.mainLocalizationIdentity}
	encoded, _ := json.Marshal(payload)
	return sha256.Sum256(encoded)
}

// SetSemanticOrigin marks an instruction while direct HFIR lowering is in
// progress. The marker remains private to bytecode serialization.
func SetSemanticOrigin(inst *BCInstruction, nodeID string) {
	if inst != nil {
		inst.semanticOrigin = nodeID
	}
}

// HasSemanticOrigin reports whether a lowerer has already marked an
// instruction. It does not reveal or permit artifact-supplied provenance.
func HasSemanticOrigin(inst BCInstruction) bool {
	return inst.semanticOrigin != ""
}

// AttachTrustedMainOrigins derives and attaches a verified source map from
// lowerer-marked instructions. It rejects incomplete input rather than
// creating a partially trustworthy map.
func (prog *BCProgram) AttachTrustedMainOrigins() bool {
	if prog == nil || len(prog.Main) == 0 {
		return false
	}
	origins := make([]string, len(prog.Main))
	for index := range prog.Main {
		if prog.Main[index].semanticOrigin == "" {
			return false
		}
		origins[index] = prog.Main[index].semanticOrigin
	}
	prog.mainOrigins = origins
	prog.mainOriginHash = mainInstructionHash(prog.Main)
	return true
}

// BindLocalizationIdentity attaches the canonical HFIR graph hash after
// direct lowering. It is ephemeral and makes bytecode-identical graphs with
// different semantics/provenance reject each other's execution evidence.
func (prog *BCProgram) BindLocalizationIdentity(graphHash string) bool {
	if prog == nil || graphHash == "" || !prog.AttachTrustedMainOrigins() {
		return false
	}
	prog.mainLocalizationIdentity = graphHash
	return true
}

// TrustedMainOriginAt returns provenance only while the exact bytecode Main
// slice produced by the trusted lowerer remains intact.
func (prog *BCProgram) TrustedMainOriginAt(offset int) (string, bool) {
	if prog == nil || offset < 0 || offset >= len(prog.Main) || len(prog.mainOrigins) != len(prog.Main) || prog.mainOriginHash != mainInstructionHash(prog.Main) {
		return "", false
	}
	origin := prog.mainOrigins[offset]
	return origin, origin != ""
}

func mainInstructionHash(instructions []BCInstruction) [32]byte {
	encoded, _ := json.Marshal(instructions)
	return sha256.Sum256(encoded)
}

type BCFunction struct {
	Params         []string        `json:"params"`
	Instructions   []BCInstruction `json:"instructions"`
	LazySynthesize bool            `json:"lazy_synthesize,omitempty"`
	Docstring      string          `json:"docstring,omitempty"`
	Name           string          `json:"name,omitempty"`
}

// compiler state
type BCCompiler struct {
	prog  *BCProgram
	funcs map[string]*BCFunction
}

func CompileToBytecode(ast *ast.Node) *BCProgram {
	comp := &BCCompiler{
		prog: &BCProgram{
			Version:   1,
			Functions: make(map[string]*BCFunction),
		},
		funcs: make(map[string]*BCFunction),
	}

	// Phase 1 interpreter expects ast to be a single block (cli_app) usually,
	// but ast is just a ast.Node.
	mainInsts := comp.compileNode(ast)
	comp.prog.Main = mainInsts
	comp.prog.Functions = comp.funcs
	return comp.prog
}

func (c *BCCompiler) compileNode(node *ast.Node) []BCInstruction {
	if node == nil {
		return nil
	}
	var insts []BCInstruction
	if node.Type == "INT" {
		val, _ := strconv.ParseInt(node.Value, 10, 64)
		insts = append(insts, BCInstruction{OpString: "LOAD_CONST", Op: OpLoadConst, ValueOperand: float64(val)})
		return insts
	}
	if node.Type == "FLOAT" {
		val, _ := strconv.ParseFloat(node.Value, 64)
		insts = append(insts, BCInstruction{OpString: "LOAD_CONST", Op: OpLoadConst, ValueOperand: val})
		return insts
	}
	if node.Type == "STRING" {
		insts = append(insts, BCInstruction{OpString: "LOAD_CONST", Op: OpLoadConst, ValueOperand: node.Value})
		return insts
	}
	if node.Type == "SYMBOL" {
		if node.Value == "true" || node.Value == "false" {
			insts = append(insts, BCInstruction{OpString: "LOAD_CONST", Op: OpLoadConst, ValueOperand: node.Value == "true"})
			return insts
		}
		insts = append(insts, BCInstruction{OpString: "LOAD_VAR", Op: OpLoadVar, StringOperand: node.Value})
		return insts
	}

	if node.Type == "List" && len(node.Children) > 0 {
		head := node.Children[0].Value
		switch head {
		case "cli_app":
			for _, child := range node.Children[1:] {
				insts = append(insts, c.compileNode(child)...)
			}
		case "defun":
			funcName := node.Children[1].Value
			var params []string
			for _, p := range node.Children[2].Children {
				params = append(params, p.Value)
			}
			var bodyInsts []BCInstruction
			for _, child := range node.Children[3:] {
				bodyInsts = append(bodyInsts, c.compileNode(child)...)
			}
			c.funcs[funcName] = &BCFunction{
				Name:         funcName,
				Params:       params,
				Instructions: bodyInsts,
			}
		case "lazy_synthesize":
			funcName := node.Children[1].Value
			var params []string
			for _, p := range node.Children[2].Children {
				params = append(params, p.Value)
			}
			docstring := node.Children[3].Value
			c.funcs[funcName] = &BCFunction{
				Name:           funcName,
				Params:         params,
				LazySynthesize: true,
				Docstring:      docstring,
			}
		case "type_hint", "type_hints", "type_param":
			// Type annotations carry no runtime meaning; they emit no
			// instructions by design. They need explicit cases rather
			// than falling through to the default below, which now
			// fails closed (bugs.md #45).

		case "try_let":
			binding := node.Children[1]
			varName := binding.Children[0].Value
			valInsts := c.compileNode(binding.Children[1])

			catchNode := node.Children[2]
			errVar := catchNode.Children[1].Value
			catchBodyInsts := c.compileNode(catchNode.Children[2])
			successBodyInsts := c.compileNode(node.Children[3])

			insts = append(insts, BCInstruction{OpString: "TRY_LET", Op: OpTryLet, StringOperand: varName, StringOperand2: errVar, IntOperand: int64(len(valInsts)), IntOperand2: int64(len(catchBodyInsts)), IntOperand3: int64(len(successBodyInsts))})
			insts = append(insts, valInsts...)
			insts = append(insts, catchBodyInsts...)
			insts = append(insts, successBodyInsts...)
		case "db_connect":
			varName := node.Children[1].Value
			driverNode := node.Children[2]
			dsnNode := node.Children[3]
			insts = append(insts, BCInstruction{OpString: "DB_CONNECT", Op: OpDbConnect, StringOperand: varName, StringOperand2: driverNode.Value, StringOperand3: dsnNode.Value})
		case "sql_query":
			dbVar := node.Children[1].Value
			queryStr := node.Children[2].Value
			insts = append(insts, BCInstruction{OpString: "SQL_QUERY", Op: OpSqlQuery, StringOperand: dbVar, StringOperand2: queryStr})
		case "store_open":
			if len(node.Children) != 3 {
				ast.ReportError(`store_open expects (store_open handle "memory://name" or "file://name")`, node.Line, node.Column)
			}
			handleNode := node.Children[1]
			uriNode := node.Children[2]
			if handleNode.Type != "SYMBOL" {
				ast.ReportError("store_open handle must be a symbol", handleNode.Line, handleNode.Column)
			}
			if uriNode.Type != "STRING" {
				ast.ReportError("store_open URI must be a string", uriNode.Line, uriNode.Column)
			}
			insts = append(insts, BCInstruction{
				OpString:       "STORE_OPEN",
				Op:             OpStoreOpen,
				StringOperand:  handleNode.Value,
				StringOperand2: uriNode.Value,
			})
		case "store_put":
			if len(node.Children) != 4 {
				ast.ReportError("store_put expects (store_put handle key record)", node.Line, node.Column)
			}
			handleNode := node.Children[1]
			if handleNode.Type != "SYMBOL" {
				ast.ReportError("store_put handle must be a symbol", handleNode.Line, handleNode.Column)
			}
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, c.compileNode(node.Children[3])...)
			insts = append(insts, BCInstruction{OpString: "STORE_PUT", Op: OpStorePut, StringOperand: handleNode.Value})
		case "store_get":
			if len(node.Children) != 3 {
				ast.ReportError("store_get expects (store_get handle key)", node.Line, node.Column)
			}
			handleNode := node.Children[1]
			if handleNode.Type != "SYMBOL" {
				ast.ReportError("store_get handle must be a symbol", handleNode.Line, handleNode.Column)
			}
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{OpString: "STORE_GET", Op: OpStoreGet, StringOperand: handleNode.Value})
		case "store_delete":
			if len(node.Children) != 3 {
				ast.ReportError("store_delete expects (store_delete handle key)", node.Line, node.Column)
			}
			handleNode := node.Children[1]
			if handleNode.Type != "SYMBOL" {
				ast.ReportError("store_delete handle must be a symbol", handleNode.Line, handleNode.Column)
			}
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{OpString: "STORE_DELETE", Op: OpStoreDelete, StringOperand: handleNode.Value})
		case "fetch":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{OpString: "FETCH", Op: OpFetch})
		case "read_file":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "READ_FILE", Op: OpReadFile})
		case "write_file":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{OpString: "WRITE_FILE", Op: OpWriteFile})
		case "mkdir":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "MKDIR", Op: OpMkdir})
		case "exec":
			insts = append(insts, c.compileNode(node.Children[1])...)
			for _, arg := range node.Children[2:] {
				insts = append(insts, c.compileNode(arg)...)
			}
			insts = append(insts, BCInstruction{OpString: "EXEC", Op: OpExec, IntOperand: int64(float64(len(node.Children) - 2))})
		case "parse_json":
			bodyVar := node.Children[2].Value
			insts = append(insts, BCInstruction{OpString: "PARSE_JSON", Op: OpParseJson, StringOperand: bodyVar})
		case "spawn":
			lambdaNode := node.Children[1]
			bodyInsts := c.compileNode(lambdaNode.Children[2])
			insts = append(insts, BCInstruction{OpString: "SPAWN", Op: OpSpawn, IntOperand: int64(float64(len(bodyInsts)))})
			insts = append(insts, bodyInsts...)
		case "spawn_agent":
			agentName := node.Children[1].Value
			taskInsts := c.compileNode(node.Children[2])
			insts = append(insts, taskInsts...)
			insts = append(insts, BCInstruction{OpString: "SPAWN_AGENT", Op: OpSpawnAgent, StringOperand: agentName})
		case "task":
			insts = append(insts, BCInstruction{OpString: "TASK", Op: OpTask, StringOperand: node.Children[1].Value})
		case "res":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, c.compileNode(node.Children[3])...)
			insts = append(insts, BCInstruction{OpString: "RES", Op: OpRes})
		case "res_json":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{OpString: "RES_JSON", Op: OpResJson})
		case "res_header":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{OpString: "HTTP_RES_HEADER", Op: OpHttpResHeader})
		case "req_method":
			insts = append(insts, BCInstruction{OpString: "HTTP_REQ_METHOD", Op: OpHttpReqMethod})
		case "http_server":
			portNode := node.Children[1]
			insts = append(insts, BCInstruction{OpString: "HTTP_SERVER_START", Op: OpHttpServerStart, StringOperand: portNode.Value})
			for _, child := range node.Children[2:] {
				if child.Type == "List" && len(child.Children) > 0 && child.Children[0].Value == "route" {
					path := child.Children[1].Value
					reqVar := child.Children[2].Children[1].Children[0].Value
					bodyInsts := c.compileNode(child.Children[2].Children[2])
					insts = append(insts, BCInstruction{OpString: "HTTP_ROUTE", Op: OpHttpRoute, StringOperand: path, StringOperand2: reqVar, IntOperand: int64(float64(len(bodyInsts)))})
					insts = append(insts, bodyInsts...)
				} else {
					insts = append(insts, c.compileNode(child)...)
				}
			}
			insts = append(insts, BCInstruction{OpString: "HTTP_SERVER_SERVE", Op: OpHttpServerServe})
		case "confidence":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "CONFIDENCE", Op: OpConfidence})

		case "achieve":
			if len(node.Children) != 3 {
				ast.ReportError("achieve expects (achieve target constraint)", node.Line, node.Column)
			}
			targetStr := ast.Stringify(node.Children[1])
			constraintStr := ast.Stringify(node.Children[2])
			insts = append(insts, BCInstruction{OpString: "LOAD_CONST", Op: OpLoadConst, ValueOperand: targetStr})
			insts = append(insts, BCInstruction{OpString: "LOAD_CONST", Op: OpLoadConst, ValueOperand: constraintStr})
			insts = append(insts, BCInstruction{OpString: "ACHIEVE", Op: OpAchieve})

		case "neural_circuit":
			if len(node.Children) < 3 {
				ast.ReportError("neural_circuit expects (neural_circuit (args...) \"instruction\")", node.Line, node.Column)
			}
			argsNode := node.Children[1]
			insts = append(insts, c.compileNode(node.Children[2])...)
			for _, arg := range argsNode.Children {
				insts = append(insts, c.compileNode(arg)...)
			}
			insts = append(insts, BCInstruction{OpString: "NEURAL_CIRCUIT", Op: OpNeuralCircuit, IntOperand: int64(len(argsNode.Children))})

		case "ephemeral_circuit":
			if len(node.Children) < 3 {
				ast.ReportError("ephemeral_circuit expects (ephemeral_circuit (args...) \"instruction\")", node.Line, node.Column)
			}
			argsNode := node.Children[1]
			insts = append(insts, c.compileNode(node.Children[2])...)
			for _, arg := range argsNode.Children {
				insts = append(insts, c.compileNode(arg)...)
			}
			insts = append(insts, BCInstruction{OpString: "EPHEMERAL_CIRCUIT", Op: OpEphemeralCircuit, IntOperand: int64(len(argsNode.Children))})

		case "llm_generate":
			insts = append(insts, c.compileNode(node.Children[1])...)
			modelStr := "llama3"
			if len(node.Children) >= 3 {
				modelStr = node.Children[2].Value
			}
			insts = append(insts, BCInstruction{OpString: "LLM_GENERATE", Op: OpLlmGenerate, StringOperand: modelStr})

		case "schema_bridge":
			insts = append(insts, c.compileNode(node.Children[2])...)

		case "optimize_signature":
			insts = append(insts, c.compileNode(node.Children[len(node.Children)-1])...)

		case "let":
			binding := node.Children[1]
			varName := binding.Children[0].Value
			valInsts := c.compileNode(binding.Children[1])

			insts = append(insts, valInsts...)
			insts = append(insts, BCInstruction{OpString: "STORE_VAR", Op: OpStoreVar, StringOperand: varName})
			insts = append(insts, c.compileNode(node.Children[2])...)
		case "set":
			varName := node.Children[1].Value
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{OpString: "SET_VAR", Op: OpSetVar, StringOperand: varName})
		case "if":
			condInsts := c.compileNode(node.Children[1])
			thenInsts := c.compileNode(node.Children[2])
			var elseInsts []BCInstruction
			if len(node.Children) == 4 {
				elseInsts = c.compileNode(node.Children[3])
			}
			insts = append(insts, condInsts...)

			// JUMP_IF_FALSE to else (or end)
			if len(elseInsts) == 0 {
				insts = append(insts, BCInstruction{OpString: "JUMP_IF_FALSE", Op: OpJumpIfFalse, IntOperand: int64(float64(len(thenInsts) + 1))})
				insts = append(insts, thenInsts...)
			} else {
				insts = append(insts, BCInstruction{OpString: "JUMP_IF_FALSE", Op: OpJumpIfFalse, IntOperand: int64(float64(len(thenInsts) + 2))})
				insts = append(insts, thenInsts...)
				insts = append(insts, BCInstruction{OpString: "JUMP", Op: OpJump, IntOperand: int64(float64(len(elseInsts) + 1))}) // jump past else
				insts = append(insts, elseInsts...)
			}
		case "while":
			condInsts := c.compileNode(node.Children[1])
			bodyInsts := c.compileNode(node.Children[2])

			insts = append(insts, condInsts...)
			// jump to end if false
			insts = append(insts, BCInstruction{OpString: "JUMP_IF_FALSE", Op: OpJumpIfFalse, IntOperand: int64(float64(len(bodyInsts) + 2))})
			insts = append(insts, bodyInsts...)
			// jump back to cond
			insts = append(insts, BCInstruction{OpString: "JUMP", Op: OpJump, IntOperand: int64(float64(-(len(condInsts) + 1 + len(bodyInsts))))})
		case "for":
			// (for item list body)
			itemName := node.Children[1].Value
			listInsts := c.compileNode(node.Children[2])
			bodyInsts := c.compileNode(node.Children[3])

			insts = append(insts, listInsts...)
			insts = append(insts, BCInstruction{OpString: "FOR_INIT", Op: OpForInit}) // pops list, pushes iterator

			loopStart := len(insts)
			insts = append(insts, BCInstruction{OpString: "FOR_NEXT", Op: OpForNext, StringOperand: itemName, IntOperand: int64(float64(len(bodyInsts) + 2))}) // if done, jump to end
			insts = append(insts, bodyInsts...)
			insts = append(insts, BCInstruction{OpString: "JUMP", Op: OpJump, IntOperand: int64(float64(-(len(insts) - loopStart)))})
		case "return":
			if len(node.Children) > 1 {
				insts = append(insts, c.compileNode(node.Children[1])...)
			} else {
				insts = append(insts, BCInstruction{OpString: "LOAD_CONST", Op: OpLoadConst, ValueOperand: nil})
			}
			insts = append(insts, BCInstruction{OpString: "RETURN", Op: OpReturn})
		case "print":
			for _, arg := range node.Children[1:] {
				insts = append(insts, c.compileNode(arg)...)
			}
			insts = append(insts, BCInstruction{OpString: "PRINT", Op: OpPrint, IntOperand: int64(float64(len(node.Children) - 1))})
		case "stderr":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "STDERR", Op: OpStderr})
		case "exit":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "EXIT", Op: OpExit})
		case "read_line":
			insts = append(insts, BCInstruction{OpString: "READ_LINE", Op: OpReadLine})
		case "call":
			funcName := node.Children[1].Value
			for _, arg := range node.Children[2:] {
				insts = append(insts, c.compileNode(arg)...)
			}
			insts = append(insts, BCInstruction{OpString: "CALL", Op: OpCall, StringOperand: funcName, IntOperand: int64(float64(len(node.Children) - 2))})
		case "+", "-", "*", "/", "<", ">", "<=", ">=", "==", "!=", "=", "and", "or":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, c.compileNode(node.Children[2])...)
			op := head
			if op == "=" {
				op = "=="
			}
			insts = append(insts, BCInstruction{OpString: "BINOP", Op: OpBinop, StringOperand: op})
		case "to_int", "to_float", "to_string", "bytes_to_string":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "CONVERT", Op: OpConvert, StringOperand: head})
		case "str_split":
			insts = append(insts, c.compileNode(node.Children[1])...) // string
			insts = append(insts, c.compileNode(node.Children[2])...) // sep
			insts = append(insts, BCInstruction{OpString: "STR_SPLIT", Op: OpStrSplit})
		case "str_join":
			insts = append(insts, c.compileNode(node.Children[1])...) // list
			insts = append(insts, c.compileNode(node.Children[2])...) // sep
			insts = append(insts, BCInstruction{OpString: "STR_JOIN", Op: OpStrJoin})
		case "regex_match":
			insts = append(insts, c.compileNode(node.Children[1])...) // regex
			insts = append(insts, c.compileNode(node.Children[2])...) // string
			insts = append(insts, BCInstruction{OpString: "REGEX_MATCH", Op: OpRegexMatch})
		case "list_len":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "LIST_LEN", Op: OpListLen})
		case "is_nil":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "IS_NIL", Op: OpIsNil})
		case "list":
			for _, child := range node.Children[1:] {
				insts = append(insts, c.compileNode(child)...)
			}
			insts = append(insts, BCInstruction{OpString: "MAKE_LIST", Op: OpMakeList, IntOperand: int64(float64(len(node.Children) - 1))})
		case "dict":
			for _, child := range node.Children[1:] {
				if len(child.Children) == 2 {
					insts = append(insts, c.compileNode(child.Children[0])...) // key
					insts = append(insts, c.compileNode(child.Children[1])...) // value
				}
			}
			insts = append(insts, BCInstruction{OpString: "MAKE_DICT", Op: OpMakeDict, IntOperand: int64(float64(len(node.Children) - 1))})
		case "append":
			insts = append(insts, c.compileNode(node.Children[2])...)
			insts = append(insts, BCInstruction{OpString: "APPEND", Op: OpAppend, StringOperand: node.Children[1].Value})
		case "map_set":
			insts = append(insts, c.compileNode(node.Children[2])...) // key
			insts = append(insts, c.compileNode(node.Children[3])...) // value
			insts = append(insts, BCInstruction{OpString: "MAP_SET", Op: OpMapSet, StringOperand: node.Children[1].Value})
		case "map_delete":
			insts = append(insts, c.compileNode(node.Children[2])...) // key
			insts = append(insts, BCInstruction{OpString: "MAP_DELETE", Op: OpMapDelete, StringOperand: node.Children[1].Value})
		case "map_get":
			insts = append(insts, c.compileNode(node.Children[2])...) // key
			insts = append(insts, BCInstruction{OpString: "MAP_GET", Op: OpMapGet, StringOperand: node.Children[1].Value})
		case "list_get":
			insts = append(insts, c.compileNode(node.Children[2])...) // index
			insts = append(insts, BCInstruction{OpString: "LIST_GET", Op: OpListGet, StringOperand: node.Children[1].Value})
		case "cli_args":
			if len(node.Children) == 2 {
				insts = append(insts, c.compileNode(node.Children[1])...)
				insts = append(insts, BCInstruction{OpString: "CLI_ARGS_GET", Op: OpCliArgsGet})
			} else {
				insts = append(insts, BCInstruction{OpString: "CLI_ARGS", Op: OpCliArgs})
			}
		case "sleep":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "SLEEP", Op: OpSleep})
		case "env":
			insts = append(insts, c.compileNode(node.Children[1])...)
			insts = append(insts, BCInstruction{OpString: "ENV", Op: OpEnv})
		case "optimize_block":
			if len(node.Children) < 4 {
				ast.ReportError("optimize_block expects (optimize_block \"metric_name\" threshold_ms body...)", node.Line, node.Column)
			}
			for _, child := range node.Children[3:] {
				insts = append(insts, c.compileNode(child)...)
			}
		case "do":
			for _, child := range node.Children[1:] {
				insts = append(insts, c.compileNode(child)...)
			}
		default:
			// Fail-closed backstop (bugs.md #45). This switch used to
			// have no default, so an unrecognized head compiled to zero
			// instructions and the construct - plus everything nested
			// inside it - vanished from the artifact with exit code 0.
			//
			// Programs reaching here have already passed
			// hfir.VerifyConstructs via howlframe.go's runHFIRGate, so this is
			// unreachable through the CLI. It exists so a future caller
			// that compiles without that gate still cannot emit a
			// silently truncated program.
			//
			// A strict allow-list is safe because the structural lists
			// that are not constructs - let/try_let bindings, dict
			// key/value pairs, defun and lambda parameter lists - are
			// destructured by their own cases above and never reach
			// compileNode in head position.
			ast.ReportError(fmt.Sprintf("bytecode compiler has no lowering for construct %q", head), node.Children[0].Line, node.Children[0].Column)
		}
	}
	return insts
}
