package hfir

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// ModelAdapterSchemaVersion identifies the public, provider-neutral candidate
// transport. It is deliberately separate from Graph.Version.
const ModelAdapterSchemaVersion = "hfir-model-adapter/v1"

// ModelRepairSchemaVersion identifies the intentionally narrow Phase-1 delta
// transport. It is not the full semantic patch protocol tracked by #91.
const ModelRepairSchemaVersion = "hfir-model-repair/v1"

const (
	modelAdapterTarget = "model-adapter"
	maxCandidateNodes  = 128
	maxNodeInputs      = 32
	maxTransportBytes  = 64 * 1024
	maxValueBytes      = 4 * 1024
)

// Adapter is a provider-neutral boundary. Implementations may call any model,
// but program semantics remain the JSON transport decoded by this package.
type Adapter interface {
	Generate(context.Context, GenerationRequest) (CandidateResponse, error)
	Repair(context.Context, RepairRequest) (RepairResponse, error)
}

// GenerationRequest contains task context, not an execution grant. Metadata
// is accounting-only and cannot become graph semantics.
type GenerationRequest struct {
	Task              string
	SupportedFeatures []string
	Metadata          AdapterMetadata
}

type RepairRequest struct {
	Task        string
	Diagnostics []Diagnostic
	Context     RepairContext
	Metadata    AdapterMetadata
}

type CandidateResponse struct {
	Transport json.RawMessage
	Metadata  AdapterMetadata
}

type RepairResponse struct {
	Delta    json.RawMessage
	Metadata AdapterMetadata
}

// AdapterMetadata supports observability without representing program intent.
type AdapterMetadata struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	Attempt  int    `json:"attempt,omitempty"`
	Tokens   int    `json:"tokens,omitempty"`
}

// Candidate is the only accepted model program representation. Graph is built
// mechanically from Transport; no AST or target-language source is involved.
type Candidate struct {
	Graph *Graph
	Hash  string
}

// RepairContext names the only graph region a repair may modify. It is made by
// trusted machinery after a diagnostic; models do not choose its hash or set.
type RepairContext struct {
	GraphHash               string           `json:"graph_hash"`
	TargetNodeIDs           []NodeID         `json:"target_node_ids"`
	Neighborhood            []NodeID         `json:"neighborhood"`
	Nodes                   []RepairNodeView `json:"nodes"`
	AllowedReferenceNodeIDs []NodeID         `json:"allowed_reference_node_ids"`
	MaxOperations           int              `json:"max_operations"`
}

// RepairNodeView is the deterministic, bounded semantic neighborhood sent to
// a provider. It is not a mutable graph payload.
type RepairNodeView struct {
	ID     NodeID     `json:"id"`
	Kind   string     `json:"kind"`
	Value  string     `json:"value,omitempty"`
	Inputs []DataEdge `json:"inputs,omitempty"`
}

type candidateTransport struct {
	SchemaVersion string          `json:"schema_version"`
	GraphVersion  string          `json:"graph_version"`
	EntryNode     NodeID          `json:"entry_node"`
	Nodes         []transportNode `json:"nodes"`
}

type transportNode struct {
	ID          NodeID          `json:"id"`
	Kind        string          `json:"kind"`
	Value       string          `json:"value"`
	LiteralKind string          `json:"literal_kind,omitempty"`
	Inputs      []transportEdge `json:"inputs,omitempty"`
	Provenance  transportOrigin `json:"provenance"`
}

type transportEdge struct {
	Role   string `json:"role"`
	NodeID NodeID `json:"node_id"`
}

// transportOrigin is deliberately a bounded label rather than source code,
// a file path, or provider-specific prompt information.
type transportOrigin struct {
	Label string `json:"label"`
}

type repairTransport struct {
	SchemaVersion     string            `json:"schema_version"`
	ExpectedGraphHash string            `json:"expected_graph_hash"`
	Operations        []repairOperation `json:"operations"`
}

type repairOperation struct {
	Operation        string        `json:"operation"`
	TargetNodeID     NodeID        `json:"target_node_id"`
	ExpectedNodeHash string        `json:"expected_node_hash"`
	Replacement      transportNode `json:"replacement"`
}

// DecodeCandidate strictly decodes the public transport into the canonical
// Graph. Unknown fields, forbidden backend fields, malformed references, and
// unsupported executable forms are rejected before LowerToBytecode is called.
func DecodeCandidate(data []byte) (Candidate, []Diagnostic) {
	var transport candidateTransport
	if diagnostic := strictDecode(data, &transport); diagnostic != nil {
		return Candidate{}, []Diagnostic{*diagnostic}
	}
	if diagnostic := validateTransport(transport); diagnostic != nil {
		return Candidate{}, []Diagnostic{*diagnostic}
	}
	graph := NewGraph()
	graph.Version = transport.GraphVersion
	for _, source := range transport.Nodes {
		node := nodeFromTransport(source)
		graph.AddNode(node)
	}
	graph.EntryNode = transport.EntryNode
	return Candidate{Graph: graph, Hash: GraphHash(graph)}, nil
}

// CompileCandidate is the model-boundary execution gate. It always verifies
// before direct lowering and never returns a partial program.
func CompileCandidate(candidate Candidate) (*bytecode.BCProgram, []Diagnostic) {
	if candidate.Graph == nil {
		return nil, []Diagnostic{adapterDiagnostic("HFIR_TRANSPORT_INVALID", "candidate graph is required", "")}
	}
	if diagnostics := NewVerifier(candidate.Graph, TargetBytecode).Verify(); hasErrors(diagnostics) {
		return nil, diagnostics
	}
	program, diagnostics := LowerToBytecode(candidate.Graph)
	if len(diagnostics) != 0 {
		return nil, diagnostics
	}
	if err := bytecode.ValidateProgram(program); err != nil {
		return nil, []Diagnostic{adapterDiagnostic("HFIR_BYTECODE_INVALID", err.Error(), candidate.Graph.EntryNode)}
	}
	return program, nil
}

// GraphHash is a deterministic fingerprint of the canonical semantic graph.
// Effects are excluded because verification infers them after candidate intake.
func GraphHash(graph *Graph) string {
	if graph == nil {
		return ""
	}
	copy := NewGraph()
	copy.Version = graph.Version
	copy.EntryNode = graph.EntryNode
	nodes := append([]*Node(nil), graph.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	for _, node := range nodes {
		if node == nil {
			continue
		}
		clone := *node
		clone.Effects = nil
		clone.DataInputs = append([]DataEdge(nil), node.DataInputs...)
		clone.ControlEdges = append([]NodeID(nil), node.ControlEdges...)
		copy.AddNode(&clone)
	}
	encoded, _ := copy.Serialize()
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

// NodeHash gives a repair precondition for one canonical node.
func NodeHash(node *Node) string {
	if node == nil {
		return ""
	}
	clone := *node
	clone.Effects = nil
	encoded, _ := json.Marshal(clone)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

// NewRepairContext returns a bounded trusted context for a diagnostic target.
// Phase 1 only authorizes replacing an existing target node.
func NewRepairContext(candidate Candidate, target NodeID) (RepairContext, error) {
	if candidate.Graph == nil || candidate.Graph.NodeByID(target) == nil {
		return RepairContext{}, fmt.Errorf("repair target %q does not exist", target)
	}
	neighborhood := []NodeID{target}
	allowedReferences := append([]NodeID(nil), candidate.Graph.NodeByID(target).DataInputsNodeIDs()...)
	for _, node := range candidate.Graph.Nodes {
		for _, edge := range node.DataInputs {
			if edge.SourceNode == target {
				neighborhood = append(neighborhood, node.ID)
			}
		}
	}
	views := make([]RepairNodeView, 0, len(neighborhood))
	for _, id := range neighborhood {
		node := candidate.Graph.NodeByID(id)
		views = append(views, RepairNodeView{ID: node.ID, Kind: node.Kind, Value: node.Value, Inputs: append([]DataEdge(nil), node.DataInputs...)})
	}
	return RepairContext{GraphHash: candidate.Hash, TargetNodeIDs: []NodeID{target}, Neighborhood: neighborhood, Nodes: views, AllowedReferenceNodeIDs: allowedReferences, MaxOperations: 1}, nil
}

// ApplyRepair applies exactly one replacement to an authorized existing node.
// It cannot alter entry/version, add nodes, grant capabilities, or rewrite
// unrelated regions. The caller must re-run verification and lowering.
func ApplyRepair(candidate Candidate, context RepairContext, data []byte) (Candidate, []Diagnostic) {
	if candidate.Graph == nil || candidate.Hash == "" || context.GraphHash != candidate.Hash {
		return Candidate{}, []Diagnostic{adapterDiagnostic("HFIR_REPAIR_STALE", "repair context does not match the current graph", "")}
	}
	var delta repairTransport
	if diagnostic := strictDecode(data, &delta); diagnostic != nil {
		return Candidate{}, []Diagnostic{*diagnostic}
	}
	if delta.SchemaVersion != ModelRepairSchemaVersion || delta.ExpectedGraphHash != candidate.Hash || len(delta.Operations) != 1 || context.MaxOperations != 1 {
		return Candidate{}, []Diagnostic{adapterDiagnostic("HFIR_REPAIR_INVALID", "repair must contain one operation against the current graph hash", "")}
	}
	op := delta.Operations[0]
	if op.Operation != "replace_node" || !containsNodeID(context.TargetNodeIDs, op.TargetNodeID) {
		return Candidate{}, []Diagnostic{adapterDiagnostic("HFIR_REPAIR_SCOPE", "repair operation is outside the authorized target node", op.TargetNodeID)}
	}
	target := candidate.Graph.NodeByID(op.TargetNodeID)
	if target == nil || op.ExpectedNodeHash != NodeHash(target) {
		return Candidate{}, []Diagnostic{adapterDiagnostic("HFIR_REPAIR_STALE", "repair node precondition does not match", op.TargetNodeID)}
	}
	if op.Replacement.ID != op.TargetNodeID {
		return Candidate{}, []Diagnostic{adapterDiagnostic("HFIR_REPAIR_IMMUTABLE", "repair cannot change node identity", op.TargetNodeID)}
	}
	if op.Replacement.Kind != target.Kind || capability.ForConstruct(op.Replacement.Kind) != capability.ForConstruct(target.Kind) {
		return Candidate{}, []Diagnostic{adapterDiagnostic("HFIR_REPAIR_IMMUTABLE", "repair cannot change node kind or introduce a capability effect", op.TargetNodeID)}
	}
	for _, input := range op.Replacement.Inputs {
		if !containsNodeID(context.AllowedReferenceNodeIDs, input.NodeID) {
			return Candidate{}, []Diagnostic{adapterDiagnostic("HFIR_REPAIR_SCOPE", "repair input references a node outside the authorized neighborhood", op.TargetNodeID)}
		}
	}
	transport := transportFromGraph(candidate.Graph)
	for index := range transport.Nodes {
		if transport.Nodes[index].ID == op.TargetNodeID {
			transport.Nodes[index] = op.Replacement
			break
		}
	}
	encoded, _ := json.Marshal(transport)
	updated, diagnostics := DecodeCandidate(encoded)
	if len(diagnostics) != 0 {
		return Candidate{}, diagnostics
	}
	return updated, nil
}

func strictDecode(data []byte, target any) *Diagnostic {
	if len(data) == 0 || len(data) > maxTransportBytes {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_BOUNDS", "model adapter JSON exceeds the Phase-1 size limit", "")
		return &diagnostic
	}
	if err := rejectDuplicateJSONFields(data); err != nil {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_INVALID", "invalid model adapter JSON: "+err.Error(), "")
		return &diagnostic
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_INVALID", "invalid model adapter JSON: "+err.Error(), "")
		return &diagnostic
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_INVALID", "model adapter JSON must contain one document", "")
		return &diagnostic
	}
	return nil
}

func validateTransport(transport candidateTransport) *Diagnostic {
	if transport.SchemaVersion != ModelAdapterSchemaVersion || transport.GraphVersion != "v1" {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_VERSION", "unsupported adapter or graph version", "")
		return &diagnostic
	}
	if len(transport.Nodes) == 0 || len(transport.Nodes) > maxCandidateNodes {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_BOUNDS", "candidate node count is outside the Phase-1 limit", "")
		return &diagnostic
	}
	seen := make(map[NodeID]bool, len(transport.Nodes))
	for _, node := range transport.Nodes {
		if node.ID == "" || seen[node.ID] {
			diagnostic := adapterDiagnostic("HFIR_TRANSPORT_ID", "node IDs must be non-empty and unique", node.ID)
			return &diagnostic
		}
		seen[node.ID] = true
		if diagnostic := validateNode(node); diagnostic != nil {
			return diagnostic
		}
	}
	if !seen[transport.EntryNode] {
		diagnostic := adapterDiagnostic("HFIR_INVALID_REF", "entry_node references a missing node", transport.EntryNode)
		return &diagnostic
	}
	entry := transportNode{}
	for _, node := range transport.Nodes {
		if node.ID == transport.EntryNode {
			entry = node
			break
		}
	}
	if entry.Kind != "program" {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_ENTRY", "entry_node must be a program node", transport.EntryNode)
		return &diagnostic
	}
	for _, node := range transport.Nodes {
		for _, input := range node.Inputs {
			if input.Role == "" || !seen[input.NodeID] {
				diagnostic := adapterDiagnostic("HFIR_INVALID_REF", "input references a missing node or has no role", node.ID)
				return &diagnostic
			}
		}
	}
	if diagnostic := validateDictEntries(transport); diagnostic != nil {
		return diagnostic
	}
	if diagnostic := validateTransportTypes(transport); diagnostic != nil {
		return diagnostic
	}
	if diagnostic := validateNoSharedEffects(transport); diagnostic != nil {
		return diagnostic
	}
	if cycle := transportCycle(transport); cycle != "" {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_CYCLE", "cyclic data dependency is not executable", cycle)
		return &diagnostic
	}
	if unreachable := unreachableTransportNode(transport); unreachable != "" {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_UNREACHABLE", "every candidate node must be reachable from entry_node", unreachable)
		return &diagnostic
	}
	return nil
}

func validateNode(node transportNode) *Diagnostic {
	if node.Provenance.Label == "" || len(node.ID) > 128 || len(node.Kind) > 64 || len(node.Inputs) > maxNodeInputs || len(node.Provenance.Label) > 80 || len(node.Value) > maxValueBytes {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_BOUNDS", "node input count or provenance label exceeds the Phase-1 limit", node.ID)
		return &diagnostic
	}
	roles := nodeRoles(node.Kind)
	if roles == nil {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_KIND", "node kind is outside the Phase-1 executable subset", node.ID)
		return &diagnostic
	}
	if !validRoles(node.Inputs, roles) {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_ROLE", "node inputs do not match required semantic roles", node.ID)
		return &diagnostic
	}
	if requiresValue(node.Kind) && node.Value == "" {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_VALUE", "node requires a non-empty semantic value", node.ID)
		return &diagnostic
	}
	if node.Kind == "const" && (node.LiteralKind != "INT" && node.LiteralKind != "FLOAT" && node.LiteralKind != "STRING") {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_LITERAL", "const node has an unsupported literal_kind", node.ID)
		return &diagnostic
	}
	if node.Kind == "const" {
		if node.LiteralKind == "INT" {
			if _, err := strconv.ParseInt(node.Value, 10, 64); err != nil {
				diagnostic := adapterDiagnostic("HFIR_TRANSPORT_LITERAL", "INT const value is invalid", node.ID)
				return &diagnostic
			}
		}
		if node.LiteralKind == "FLOAT" {
			value, err := strconv.ParseFloat(node.Value, 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				diagnostic := adapterDiagnostic("HFIR_TRANSPORT_LITERAL", "FLOAT const value must be finite", node.ID)
				return &diagnostic
			}
		}
	}
	if node.Kind != "const" && node.LiteralKind != "" {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_LITERAL", "literal_kind is only valid on const nodes", node.ID)
		return &diagnostic
	}
	if node.Kind == "binary" && !allowedBinary(node.Value) {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_VALUE", "binary node has an unsupported operator", node.ID)
		return &diagnostic
	}
	if node.Kind == "convert" && !allowedConvert(node.Value) {
		diagnostic := adapterDiagnostic("HFIR_TRANSPORT_VALUE", "convert node has an unsupported conversion", node.ID)
		return &diagnostic
	}
	return nil
}

func nodeRoles(kind string) []string {
	switch kind {
	case "program", "sequence":
		return []string{"body*"}
	case "const", "symbol":
		return []string{}
	case "let":
		return []string{"value", "body"}
	case "set", "stderr", "exit", "convert", "list_len", "env":
		return []string{"value"}
	case "if":
		return []string{"condition", "then", "else?"}
	case "binary":
		return []string{"left", "right"}
	case "list":
		return []string{"item*"}
	case "dict":
		return []string{"entry*"}
	case "dict_entry":
		return []string{"key", "value"}
	case "print":
		return []string{"value*"}
	case "str_split", "str_join":
		return []string{"value", "separator"}
	case "map_get", "map_delete":
		return []string{"key"}
	case "list_get":
		return []string{"index"}
	case "append":
		return []string{"item"}
	case "map_set":
		return []string{"key", "value"}
	default:
		return nil
	}
}

func validRoles(inputs []transportEdge, wanted []string) bool {
	if len(wanted) == 1 && strings.HasSuffix(wanted[0], "*") {
		name := strings.TrimSuffix(wanted[0], "*")
		for _, input := range inputs {
			if input.Role != name {
				return false
			}
		}
		return true
	}
	minimum := len(wanted)
	optional := false
	if minimum > 0 && strings.HasSuffix(wanted[minimum-1], "?") {
		minimum--
		optional = true
	}
	if len(inputs) != minimum && (!optional || len(inputs) != minimum+1) {
		return false
	}
	for index, input := range inputs {
		if len(input.Role) > 32 || len(input.NodeID) > 128 {
			return false
		}
		name := strings.TrimSuffix(strings.TrimSuffix(wanted[index], "?"), "*")
		if input.Role != name {
			return false
		}
	}
	return true
}

func requiresValue(kind string) bool {
	switch kind {
	case "symbol", "let", "set", "binary", "convert", "map_get", "map_delete", "list_get", "append", "map_set":
		return true
	}
	return false
}

func allowedBinary(value string) bool {
	switch value {
	case "+", "-", "*", "/", "<", ">", "<=", ">=", "==", "!=", "and", "or":
		return true
	}
	return false
}

func allowedConvert(value string) bool {
	switch value {
	case "to_int", "to_float", "to_string", "bytes_to_string":
		return true
	}
	return false
}

func validateTransportTypes(transport candidateTransport) *Diagnostic {
	byID := make(map[NodeID]transportNode, len(transport.Nodes))
	for _, node := range transport.Nodes {
		byID[node.ID] = node
	}
	known := make(map[NodeID]string, len(transport.Nodes))
	var infer func(NodeID) string
	infer = func(id NodeID) string {
		if typ, ok := known[id]; ok {
			return typ
		}
		node := byID[id]
		typ := "unknown"
		switch node.Kind {
		case "const":
			if node.LiteralKind == "STRING" {
				typ = "string"
			} else {
				typ = "number"
			}
		case "list":
			typ = "list"
		case "dict":
			typ = "dict"
		case "env", "str_join", "str_split", "convert":
			typ = "string"
		case "list_len":
			typ = "number"
		case "binary":
			left, right := infer(node.Inputs[0].NodeID), infer(node.Inputs[1].NodeID)
			if node.Value == "and" || node.Value == "or" {
				typ = "bool"
				if !compatibleBoolean(left) || !compatibleBoolean(right) {
					typ = "invalid"
				}
			} else if isComparison(node.Value) {
				typ = "bool"
				if !compatibleBinary(left, right) {
					typ = "invalid"
				}
			} else {
				typ = "number"
				if !compatibleNumber(left) || !compatibleNumber(right) {
					typ = "invalid"
				}
			}
		}
		known[id] = typ
		return typ
	}
	for _, node := range transport.Nodes {
		if infer(node.ID) == "invalid" {
			diagnostic := adapterDiagnostic("HFIR_TRANSPORT_TYPE", "node operands have incompatible known types", node.ID)
			return &diagnostic
		}
	}
	return nil
}

func compatibleNumber(typ string) bool  { return typ == "number" || typ == "unknown" }
func compatibleBoolean(typ string) bool { return typ == "bool" || typ == "unknown" }
func compatibleBinary(left, right string) bool {
	return left == "unknown" || right == "unknown" || left == right || (left == "number" && right == "number")
}
func isComparison(op string) bool {
	switch op {
	case "<", ">", "<=", ">=", "==", "!=":
		return true
	}
	return false
}

func validateNoSharedEffects(transport candidateTransport) *Diagnostic {
	uses := make(map[NodeID]int)
	for _, node := range transport.Nodes {
		for _, input := range node.Inputs {
			uses[input.NodeID]++
		}
	}
	for _, node := range transport.Nodes {
		if uses[node.ID] > 1 && isEffectful(node.Kind) {
			diagnostic := adapterDiagnostic("HFIR_TRANSPORT_EFFECT", "effectful nodes may not have multiple consumers", node.ID)
			return &diagnostic
		}
	}
	return nil
}

func isEffectful(kind string) bool {
	switch kind {
	case "set", "map_set", "map_delete", "append", "print", "stderr", "exit", "env":
		return true
	}
	return false
}

func hasErrors(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == SeverityError {
			return true
		}
	}
	return false
}

func nodeFromTransport(node transportNode) *Node {
	inputs := make([]DataEdge, 0, len(node.Inputs))
	for _, input := range node.Inputs {
		inputs = append(inputs, DataEdge{Name: input.Role, SourceNode: input.NodeID})
	}
	return &Node{ID: node.ID, Kind: node.Kind, Value: node.Value, LiteralKind: node.LiteralKind, DataInputs: inputs, Provenance: Provenance{Filename: "model:" + node.Provenance.Label}}
}

func transportFromGraph(graph *Graph) candidateTransport {
	result := candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: graph.Version, EntryNode: graph.EntryNode, Nodes: make([]transportNode, 0, len(graph.Nodes))}
	for _, node := range graph.Nodes {
		inputs := make([]transportEdge, 0, len(node.DataInputs))
		for _, input := range node.DataInputs {
			inputs = append(inputs, transportEdge{Role: input.Name, NodeID: input.SourceNode})
		}
		result.Nodes = append(result.Nodes, transportNode{ID: node.ID, Kind: node.Kind, Value: node.Value, LiteralKind: node.LiteralKind, Inputs: inputs, Provenance: transportOrigin{Label: strings.TrimPrefix(node.Provenance.Filename, "model:")}})
	}
	return result
}

func transportCycle(transport candidateTransport) NodeID {
	byID := make(map[NodeID]transportNode, len(transport.Nodes))
	for _, node := range transport.Nodes {
		byID[node.ID] = node
	}
	visiting, done := map[NodeID]bool{}, map[NodeID]bool{}
	var visit func(NodeID) NodeID
	visit = func(id NodeID) NodeID {
		if visiting[id] {
			return id
		}
		if done[id] {
			return ""
		}
		visiting[id] = true
		for _, input := range byID[id].Inputs {
			if found := visit(input.NodeID); found != "" {
				return found
			}
		}
		delete(visiting, id)
		done[id] = true
		return ""
	}
	for _, node := range transport.Nodes {
		if found := visit(node.ID); found != "" {
			return found
		}
	}
	return ""
}

func unreachableTransportNode(transport candidateTransport) NodeID {
	byID := make(map[NodeID]transportNode, len(transport.Nodes))
	for _, node := range transport.Nodes {
		byID[node.ID] = node
	}
	seen := make(map[NodeID]bool, len(transport.Nodes))
	var visit func(NodeID)
	visit = func(id NodeID) {
		if seen[id] {
			return
		}
		seen[id] = true
		for _, input := range byID[id].Inputs {
			visit(input.NodeID)
		}
	}
	visit(transport.EntryNode)
	for _, node := range transport.Nodes {
		if !seen[node.ID] {
			return node.ID
		}
	}
	return ""
}

func validateDictEntries(transport candidateTransport) *Diagnostic {
	byID := make(map[NodeID]transportNode, len(transport.Nodes))
	usedByDict := make(map[NodeID]bool)
	for _, node := range transport.Nodes {
		byID[node.ID] = node
	}
	for _, node := range transport.Nodes {
		if node.Kind != "dict" {
			continue
		}
		for _, input := range node.Inputs {
			if byID[input.NodeID].Kind != "dict_entry" {
				diagnostic := adapterDiagnostic("HFIR_TRANSPORT_ROLE", "dict entry input must reference a dict_entry node", node.ID)
				return &diagnostic
			}
			usedByDict[input.NodeID] = true
		}
	}
	for _, node := range transport.Nodes {
		if node.Kind == "dict_entry" && !usedByDict[node.ID] {
			diagnostic := adapterDiagnostic("HFIR_TRANSPORT_ROLE", "dict_entry nodes may only appear in dict entry inputs", node.ID)
			return &diagnostic
		}
	}
	return nil
}

// rejectDuplicateJSONFields rejects JSON that encoding/json would otherwise
// accept with last-field-wins semantics. It also confirms nested JSON is
// syntactically complete before strict struct decoding.
func rejectDuplicateJSONFields(data []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := checkJSONValue(decoder); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}

func checkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if seen[name] {
				return fmt.Errorf("duplicate field %q", name)
			}
			seen[name] = true
			if err := checkJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := checkJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

func containsNodeID(ids []NodeID, wanted NodeID) bool {
	for _, id := range ids {
		if id == wanted {
			return true
		}
	}
	return false
}

func (n *Node) DataInputsNodeIDs() []NodeID {
	result := make([]NodeID, 0, len(n.DataInputs))
	for _, input := range n.DataInputs {
		result = append(result, input.SourceNode)
	}
	return result
}

func adapterDiagnostic(code, message string, node NodeID) Diagnostic {
	return Diagnostic{Code: code, Severity: SeverityError, Message: message, RelatedNode: node, ContractVersion: DiagnosticContractVersion, Target: modelAdapterTarget}
}
