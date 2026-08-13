package hfir

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/howlcipher/howlframe/internal/bytecode"
)

// LocalizationEvidence is supplied by trusted verification, lowering, runner,
// and oracle machinery. It is intentionally not part of model transport.
type LocalizationEvidence struct {
	GraphHash       string
	GraphVersion    string
	Diagnostics     []Diagnostic
	Execution       *bytecode.ExecutionEvidence
	ExpectedOutcome string
	ActualOutcome   string
	ExpectedValue   *bytecode.ExpectedValueObservation
}

type SelectionReason string

const (
	SelectionRuntime          SelectionReason = "RUNTIME_ERROR_ORIGIN"
	SelectionBehavioral       SelectionReason = "OUTPUT_ANCHOR"
	SelectionDependency       SelectionReason = "DIRECT_DATA_DEPENDENCY"
	SelectionExecutedProducer SelectionReason = "EXECUTED_VALUE_PRODUCER"
	SelectionLastWriter       SelectionReason = "LAST_STATE_WRITER"
	SelectionBranchControl    SelectionReason = "EXECUTED_BRANCH_CONDITION"
	SelectionNegativeDelete   SelectionReason = "NEGATIVE_STATE_EFFECTIVE_DELETE"
	SelectionNegativeValue    SelectionReason = "NEGATIVE_STATE_VALUE_MATCH"
)

const maxNegativeStateCandidates = 4

// NegativeStateEvidence is runner-derived, read-only explanation for a map
// miss. Candidate node IDs never grant authority by themselves.
type NegativeStateEvidence struct {
	ReaderNodeID     NodeID   `json:"reader_node_id"`
	ResourceID       uint64   `json:"resource_id"`
	ReadSequence     uint64   `json:"read_sequence"`
	RequestedKey     string   `json:"requested_key_fingerprint"`
	Cause            string   `json:"cause"`
	CandidateNodeIDs []NodeID `json:"candidate_node_ids,omitempty"`
}

// CandidateSelection is deterministic explanatory evidence, not an editable
// field. Rank is stable only within a single localized result.
type CandidateSelection struct {
	NodeID NodeID          `json:"node_id"`
	Reason SelectionReason `json:"reason"`
	Rank   int             `json:"rank"`
}

// CandidateRepairRegion is a hash-bound automatic replacement for manually
// supplied Phase-2 target IDs. Context retains the existing fail-closed
// transaction contract and is recomputed by ApplyRepair.
type CandidateRepairRegion struct {
	GraphHash      string                 `json:"graph_hash"`
	GraphVersion   string                 `json:"graph_version"`
	RegionHash     string                 `json:"region_hash"`
	Selections     []CandidateSelection   `json:"selections"`
	Context        RepairContext          `json:"context"`
	MaxRegionNodes int                    `json:"max_region_nodes"`
	NegativeState  *NegativeStateEvidence `json:"negative_state,omitempty"`
}

// DerivedControlRelation is an internal semantic view derived from validated
// Phase-1 roles. It is not accepted from model transport or serialized HFIR.
type DerivedControlRelation struct {
	Controller NodeID `json:"controller"`
	Controlled NodeID `json:"controlled"`
	Role       string `json:"role"`
	Ordinal    int    `json:"ordinal"`
}

// DerivePhase1ControlRelations exposes only real executable containment and
// branch relationships. A malformed canonical node yields no relation rather
// than a guessed edge.
func DerivePhase1ControlRelations(graph *Graph) []DerivedControlRelation {
	if graph == nil {
		return nil
	}
	var relations []DerivedControlRelation
	for _, node := range graph.Nodes {
		if node == nil {
			continue
		}
		switch node.Kind {
		case "program", "sequence":
			for index, input := range node.DataInputs {
				if input.Name == "body" && graph.NodeByID(input.SourceNode) != nil {
					relations = append(relations, DerivedControlRelation{Controller: node.ID, Controlled: input.SourceNode, Role: "body", Ordinal: index})
				}
			}
		case "let":
			if len(node.DataInputs) == 2 && node.DataInputs[0].Name == "value" && node.DataInputs[1].Name == "body" && graph.NodeByID(node.DataInputs[1].SourceNode) != nil {
				relations = append(relations, DerivedControlRelation{Controller: node.ID, Controlled: node.DataInputs[1].SourceNode, Role: "body", Ordinal: 1})
			}
		case "if":
			if len(node.DataInputs) >= 2 && len(node.DataInputs) <= 3 && node.DataInputs[0].Name == "condition" {
				for index := 1; index < len(node.DataInputs); index++ {
					input := node.DataInputs[index]
					if (input.Name == "then" || input.Name == "else") && graph.NodeByID(input.SourceNode) != nil {
						relations = append(relations, DerivedControlRelation{Controller: node.ID, Controlled: input.SourceNode, Role: input.Name, Ordinal: index})
					}
				}
			}
		}
	}
	sort.Slice(relations, func(i, j int) bool {
		if relations[i].Controller != relations[j].Controller {
			return relations[i].Controller < relations[j].Controller
		}
		return relations[i].Ordinal < relations[j].Ordinal
	})
	return relations
}

// LocalizeFailure derives a bounded repair region from trusted evidence. It
// never accepts model-authored node IDs and fails closed on stale evidence,
// untrusted runtime provenance, authority denials, or an oversized region.
func LocalizeFailure(candidate Candidate, evidence LocalizationEvidence) (CandidateRepairRegion, []Diagnostic) {
	if candidate.Graph == nil || candidate.Hash == "" || GraphHash(candidate.Graph) != candidate.Hash || evidence.GraphHash != candidate.Hash || evidence.GraphVersion != candidate.Graph.Version {
		return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_STALE", "localization evidence does not match the canonical graph")}
	}
	program, compileDiagnostics := CompileCandidate(candidate)
	if len(compileDiagnostics) != 0 {
		return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "current graph cannot reproduce trusted direct lowering")}
	}
	if evidence.Execution != nil && !bytecode.TrustedExecutionEvidence(program, evidence.Execution) {
		return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "execution evidence was not minted for the current direct-lowered graph")}
	}
	selections := make(map[NodeID]SelectionReason)
	core := make(map[NodeID]bool)
	var negativeState *NegativeStateEvidence
	add := func(id NodeID, reason SelectionReason) {
		if id != "" && candidate.Graph.NodeByID(id) != nil {
			if _, exists := selections[id]; !exists {
				selections[id] = reason
			}
		}
	}
	// Diagnostics are intentionally not localization seeds. They are exported
	// transport data and have no runner seal; accepting them would let a model
	// name any canonical node. Runtime evidence is the trusted semantic path.
	if evidence.Execution != nil && evidence.Execution.RuntimeFailure != nil {
		failure := evidence.Execution.RuntimeFailure
		if failure.Code == "CAPABILITY_DENIED" || failure.Code == "LIMIT_EXCEEDED" {
			return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_AUTHORITY", "runtime authority failures are not semantic repair targets")}
		}
		if failure.NodeID == "" || candidate.Graph.NodeByID(NodeID(failure.NodeID)) == nil {
			return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "runtime failure has no verified HFIR origin")}
		}
		add(NodeID(failure.NodeID), SelectionRuntime)
		core[NodeID(failure.NodeID)] = true
	}
	if evidence.ExpectedOutcome != evidence.ActualOutcome {
		if evidence.Execution == nil || evidence.Execution.TraceTruncated {
			return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_TRACE", "behavioral mismatch requires a complete trusted execution trace")}
		}
		anchor := behavioralAnchor(evidence.Execution.Trace)
		if anchor == "" {
			return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_BEHAVIOR", "behavioral mismatch has no traced output or exit origin")}
		}
		add(anchor, SelectionBehavioral)
		executed := executedNodeIDs(evidence.Execution.Trace)
		dependencies := executedDataClosure(candidate.Graph, anchor, executed)
		negative, negativeDiagnostic := negativeStateCandidates(candidate.Graph, evidence, anchor)
		negativeState = negative
		if negativeDiagnostic != nil {
			return CandidateRepairRegion{GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, NegativeState: negative}, []Diagnostic{*negativeDiagnostic}
		}
		if negative != nil {
			// A direct absent map read is not independently editable. Its key
			// producer is only context until an effective delete or uniquely
			// value-correlated live writer proves a causal mutation.
			dependencies = nil
			if negative.Cause == "DELETED" && len(negative.CandidateNodeIDs) == 1 {
				writer := negative.CandidateNodeIDs[0]
				add(writer, SelectionNegativeDelete)
				for _, dependency := range executedWriterDependencies(candidate.Graph, writer, executed) {
					add(dependency, SelectionDependency)
					if isLeaf(candidate.Graph, dependency) {
						core[dependency] = true
					}
				}
			} else if negative.Cause == "VALUE_MATCH" && len(negative.CandidateNodeIDs) == 1 {
				writer := negative.CandidateNodeIDs[0]
				add(writer, SelectionNegativeValue)
				for _, dependency := range executedWriterDependencies(candidate.Graph, writer, executed) {
					add(dependency, SelectionDependency)
					if isLeaf(candidate.Graph, dependency) {
						core[dependency] = true
					}
				}
			} else {
				return CandidateRepairRegion{GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, NegativeState: negative}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_AMBIGUOUS_STATE_CAUSE", "absent map key has no uniquely proven causal writer")}
			}
		}
		for _, dependency := range dependencies {
			add(dependency, SelectionExecutedProducer)
		}
		writers := lastWriters(candidate.Graph, evidence.Execution.Trace, anchor)
		if negative != nil {
			writers = nil
		}
		for _, writer := range writers {
			add(writer, SelectionLastWriter)
			for _, dependency := range executedWriterDependencies(candidate.Graph, writer, executed) {
				add(dependency, SelectionDependency)
				if isLeaf(candidate.Graph, dependency) {
					core[dependency] = true
				}
			}
		}
		if len(writers) == 0 {
			for _, dependency := range dependencies {
				if isLeaf(candidate.Graph, dependency) {
					core[dependency] = true
				}
			}
		}
		for _, condition := range executedBranchConditions(candidate.Graph, evidence.Execution.Trace, anchor) {
			add(condition, SelectionBranchControl)
		}
	}
	if len(selections) == 0 {
		return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_UNAVAILABLE", "failure evidence does not identify a semantic node")}
	}
	targets := sortedNodeIDs(core)
	if len(targets) == 0 {
		return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_UNAVAILABLE", "failure evidence did not yield repair seeds")}
	}
	context, err := NewLocalizedRepairContext(candidate, targets)
	if err != nil {
		return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_BOUNDS", err.Error())}
	}
	result := CandidateRepairRegion{GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, Context: context, MaxRegionNodes: maxRepairRegionNodes, NegativeState: negativeState}
	for index, id := range targets {
		result.Selections = append(result.Selections, CandidateSelection{NodeID: id, Reason: selections[id], Rank: index + 1})
	}
	encoded, _ := json.Marshal(struct {
		GraphHash  string
		Selections []CandidateSelection
		Editable   []NodeID
	}{result.GraphHash, result.Selections, result.Context.EditableNodeIDs})
	digest := sha256.Sum256(encoded)
	result.RegionHash = hex.EncodeToString(digest[:])
	return result, nil
}

func behavioralAnchor(trace []bytecode.ExecutionTraceEvent) NodeID {
	for index := len(trace) - 1; index >= 0; index-- {
		event := trace[index]
		if event.NodeID != "" && (event.Opcode == "PRINT" || event.Opcode == "STDERR" || event.Opcode == "EXIT") {
			return NodeID(event.NodeID)
		}
	}
	return ""
}

func dataClosure(graph *Graph, start NodeID, depth int) []NodeID {
	seen := make(map[NodeID]bool)
	var visit func(NodeID, int)
	visit = func(id NodeID, remaining int) {
		if remaining <= 0 || seen[id] {
			return
		}
		node := graph.NodeByID(id)
		if node == nil {
			return
		}
		for _, input := range node.DataInputs {
			if graph.NodeByID(input.SourceNode) != nil {
				seen[input.SourceNode] = true
				visit(input.SourceNode, remaining-1)
			}
		}
	}
	visit(start, depth)
	return sortedNodeIDs(seen)
}

func executedNodeIDs(trace []bytecode.ExecutionTraceEvent) map[NodeID]bool {
	result := make(map[NodeID]bool, len(trace))
	for _, event := range trace {
		if event.NodeID != "" {
			result[NodeID(event.NodeID)] = true
		}
	}
	return result
}

func executedDataClosure(graph *Graph, start NodeID, executed map[NodeID]bool) []NodeID {
	seen := make(map[NodeID]bool)
	var visit func(NodeID)
	visit = func(id NodeID) {
		node := graph.NodeByID(id)
		if node == nil {
			return
		}
		for _, input := range node.DataInputs {
			if !seen[input.SourceNode] && executed[input.SourceNode] {
				seen[input.SourceNode] = true
				visit(input.SourceNode)
			}
		}
	}
	visit(start)
	return sortedNodeIDs(seen)
}

func isLeaf(graph *Graph, id NodeID) bool {
	node := graph.NodeByID(id)
	return node != nil && len(node.DataInputs) == 0
}

// lastWriters finds the last executed mutation that produced the state read by
// an observed output. It intentionally derives the resource and opaque key
// from runner-sealed trace events, never from node labels or model transport.
// Only the executable state forms with a matching runtime observation are
// considered; this is bounded provenance, not a dynamic taint engine.
func lastWriters(graph *Graph, trace []bytecode.ExecutionTraceEvent, anchor NodeID) []NodeID {
	anchorNode := graph.NodeByID(anchor)
	if anchorNode == nil || len(anchorNode.DataInputs) != 1 || anchorNode.DataInputs[0].Name != "value" {
		return nil
	}
	value := graph.NodeByID(anchorNode.DataInputs[0].SourceNode)
	if value == nil {
		return nil
	}
	resource, writerKinds, ok := observedStateResource(value)
	if !ok {
		return nil
	}
	anchorIndex := -1
	for index := len(trace) - 1; index >= 0; index-- {
		if NodeID(trace[index].NodeID) == anchor {
			anchorIndex = index
			break
		}
	}
	if anchorIndex < 0 {
		return nil
	}
	readIndex := -1
	for index := anchorIndex - 1; index >= 0; index-- {
		event := trace[index]
		if NodeID(event.NodeID) == value.ID && event.Resource == resource && event.StateKey != "" {
			readIndex = index
			break
		}
	}
	if readIndex < 0 {
		return nil
	}
	key := trace[readIndex].StateKey
	var fallback NodeID
	fallbackAmbiguous := false
	for index := readIndex - 1; index >= 0; index-- {
		event := trace[index]
		node := graph.NodeByID(NodeID(event.NodeID))
		if node != nil && writerKinds[node.Kind] && event.Resource == resource {
			if node.Kind == "map_set" || node.Kind == "map_delete" {
				if fallback == "" {
					fallback = node.ID
				} else if fallback != node.ID {
					fallbackAmbiguous = true
				}
			}
			if event.StateKey == key {
				return []NodeID{node.ID}
			}
		}
	}
	// A failed map lookup has no matching writer. A single preceding map
	// mutation is a bounded fallback for a likely wrong-key mutation. Several
	// possible writers are deliberately left unselected rather than guessing;
	// matching keys always win and later writes after the read are never
	// considered.
	if fallback != "" && !fallbackAmbiguous {
		return []NodeID{fallback}
	}
	return nil
}

type replayedMapState struct {
	initialized          bool
	version              uint64
	active               map[string]bytecode.MapStateEvent
	lastEffectiveDeletes map[string]bytecode.MapStateEvent
}

// negativeStateCandidates replays runner-sealed completed map operations to
// prove an observed direct map_get miss. It never interprets labels, source
// names, or event recency as causal authority.
func negativeStateCandidates(graph *Graph, evidence LocalizationEvidence, anchor NodeID) (*NegativeStateEvidence, *Diagnostic) {
	reader := directMapRead(graph, anchor)
	if reader == "" {
		return nil, nil
	}
	if evidence.Execution == nil || evidence.Execution.TraceTruncated || !evidence.Execution.MapLedgerComplete {
		diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "negative state localization requires complete sealed map history")
		return nil, &diagnostic
	}
	states := make(map[uint64]*replayedMapState)
	var previous uint64
	for _, event := range evidence.Execution.MapStateEvents {
		if event.Sequence != previous+1 || event.ResourceID == 0 || event.NodeID == "" {
			diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "map state ledger is not a valid monotonic runner history")
			return nil, &diagnostic
		}
		previous = event.Sequence
		state := states[event.ResourceID]
		if state == nil {
			state = &replayedMapState{active: make(map[string]bytecode.MapStateEvent), lastEffectiveDeletes: make(map[string]bytecode.MapStateEvent)}
			states[event.ResourceID] = state
		}
		switch event.Operation {
		case "INIT":
			if state.initialized && state.version != 0 {
				diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "map ledger reinitializes an active resource")
				return nil, &diagnostic
			}
			state.initialized = true
			if event.KeyFingerprint != "" {
				state.active[event.KeyFingerprint] = event
			}
		case "SET":
			if !state.initialized || event.KeyFingerprint == "" || event.VersionBefore != state.version || event.VersionAfter != state.version+1 {
				diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "map set ledger version is invalid")
				return nil, &diagnostic
			}
			state.version = event.VersionAfter
			state.active[event.KeyFingerprint] = event
			delete(state.lastEffectiveDeletes, event.KeyFingerprint)
		case "DELETE":
			if !state.initialized || event.KeyFingerprint == "" || event.VersionBefore != state.version || event.VersionAfter != state.version+1 {
				diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "map delete ledger version is invalid")
				return nil, &diagnostic
			}
			_, existed := state.active[event.KeyFingerprint]
			if existed != event.DeletedExisting {
				diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "map delete ledger does not match replayed state")
				return nil, &diagnostic
			}
			state.version = event.VersionAfter
			delete(state.active, event.KeyFingerprint)
			if existed {
				state.lastEffectiveDeletes[event.KeyFingerprint] = event
			}
		case "GET":
			if !state.initialized || event.KeyFingerprint == "" || event.VersionBefore != state.version || event.VersionAfter != state.version {
				diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "map get ledger version is invalid")
				return nil, &diagnostic
			}
			_, hit := state.active[event.KeyFingerprint]
			if hit != event.Hit {
				diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "map get ledger hit result does not match replayed state")
				return nil, &diagnostic
			}
			if NodeID(event.NodeID) != reader || event.Hit {
				continue
			}
			result := &NegativeStateEvidence{ReaderNodeID: reader, ResourceID: event.ResourceID, ReadSequence: event.Sequence, RequestedKey: event.KeyFingerprint, Cause: "NEVER_PRESENT"}
			if deletion, ok := state.lastEffectiveDeletes[event.KeyFingerprint]; ok {
				result.Cause = "DELETED"
				result.CandidateNodeIDs = []NodeID{NodeID(deletion.NodeID)}
				return result, nil
			}
			var candidates []bytecode.MapStateEvent
			for key, writer := range state.active {
				if key != event.KeyFingerprint && writer.Operation == "SET" && writer.Sequence < event.Sequence {
					candidates = append(candidates, writer)
				}
			}
			sort.Slice(candidates, func(i, j int) bool { return candidates[i].Sequence < candidates[j].Sequence })
			if len(candidates) > maxNegativeStateCandidates {
				diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_BOUNDS", "negative state candidate set exceeds the safe ceiling")
				return result, &diagnostic
			}
			if canUseExpectedMapValue(evidence, graph, anchor) {
				for _, writer := range candidates {
					if writer.ValueFingerprint == evidence.ExpectedValue.Fingerprint {
						result.CandidateNodeIDs = append(result.CandidateNodeIDs, NodeID(writer.NodeID))
					}
				}
				if len(result.CandidateNodeIDs) > 0 {
					result.Cause = "VALUE_MATCH"
					return result, nil
				}
			}
			for _, writer := range candidates {
				result.CandidateNodeIDs = append(result.CandidateNodeIDs, NodeID(writer.NodeID))
			}
			return result, nil
		default:
			diagnostic := localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "map ledger contains an unknown operation")
			return nil, &diagnostic
		}
	}
	return nil, nil
}

func directMapRead(graph *Graph, anchor NodeID) NodeID {
	anchorNode := graph.NodeByID(anchor)
	if anchorNode == nil || anchorNode.Kind != "print" || len(anchorNode.DataInputs) != 1 || anchorNode.DataInputs[0].Name != "value" {
		return ""
	}
	value := graph.NodeByID(anchorNode.DataInputs[0].SourceNode)
	if value == nil || value.Kind != "map_get" {
		return ""
	}
	return value.ID
}

func canUseExpectedMapValue(evidence LocalizationEvidence, graph *Graph, anchor NodeID) bool {
	if evidence.ExpectedValue == nil || !bytecode.TrustedExpectedValueObservation(*evidence.ExpectedValue) || evidence.ActualOutcome != "\n" || evidence.ExpectedOutcome != evidence.ExpectedValue.Value+"\n" {
		return false
	}
	return directMapRead(graph, anchor) != ""
}

func observedStateResource(node *Node) (string, map[string]bool, bool) {
	switch node.Kind {
	case "map_get":
		return "map:" + node.Value, map[string]bool{"map_set": true, "map_delete": true}, node.Value != ""
	case "symbol":
		return "var:" + node.Value, map[string]bool{"set": true, "let": true}, node.Value != ""
	case "list_get":
		return "list:" + node.Value, map[string]bool{"append": true}, node.Value != ""
	default:
		return "", nil, false
	}
}

// executedWriterDependencies returns only the inputs that contribute to the
// state mutation. In particular, a let body is not a producer of the binding;
// including it would turn a state observation into a whole-body repair region.
func executedWriterDependencies(graph *Graph, writer NodeID, executed map[NodeID]bool) []NodeID {
	node := graph.NodeByID(writer)
	if node == nil {
		return nil
	}
	inputs := make(map[NodeID]bool)
	for _, input := range node.DataInputs {
		if (node.Kind == "let" && input.Name != "value") || !executed[input.SourceNode] {
			continue
		}
		inputs[input.SourceNode] = true
	}
	result := make(map[NodeID]bool)
	for input := range inputs {
		result[input] = true
		for _, dependency := range executedDataClosure(graph, input, executed) {
			result[dependency] = true
		}
	}
	return sortedNodeIDs(result)
}

func executedBranchConditions(graph *Graph, trace []bytecode.ExecutionTraceEvent, anchor NodeID) []NodeID {
	var conditions []NodeID
	for _, event := range trace {
		if event.BranchTaken == nil || event.NodeID == "" {
			continue
		}
		node := graph.NodeByID(NodeID(event.NodeID))
		if node == nil || node.Kind != "if" || len(node.DataInputs) < 2 {
			continue
		}
		branch := node.DataInputs[1]
		if *event.BranchTaken && len(node.DataInputs) == 3 {
			branch = node.DataInputs[2]
		}
		if containsSemanticNode(graph, branch.SourceNode, anchor) {
			conditions = append(conditions, node.DataInputs[0].SourceNode)
		}
	}
	return conditions
}

func containsSemanticNode(graph *Graph, root, wanted NodeID) bool {
	if root == wanted {
		return true
	}
	node := graph.NodeByID(root)
	if node == nil {
		return false
	}
	for _, input := range node.DataInputs {
		if containsSemanticNode(graph, input.SourceNode, wanted) {
			return true
		}
	}
	return false
}

func localizationDiagnostic(code, message string) Diagnostic {
	return Diagnostic{Code: code, Severity: SeverityError, Message: message, ContractVersion: DiagnosticContractVersion, Target: TargetBytecode}
}

func (region CandidateRepairRegion) String() string {
	return fmt.Sprintf("%s:%s", region.GraphHash, region.RegionHash)
}
