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
}

type SelectionReason string

const (
	SelectionDiagnostic    SelectionReason = "diagnostic"
	SelectionRuntime       SelectionReason = "runtime_origin"
	SelectionBehavioral    SelectionReason = "behavioral_output"
	SelectionDependency    SelectionReason = "behavioral_dependency"
	SelectionLastWriter    SelectionReason = "behavioral_last_writer"
	SelectionBranchControl SelectionReason = "executed_branch_control"
)

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
	GraphHash      string               `json:"graph_hash"`
	GraphVersion   string               `json:"graph_version"`
	RegionHash     string               `json:"region_hash"`
	Selections     []CandidateSelection `json:"selections"`
	Context        RepairContext        `json:"context"`
	MaxRegionNodes int                  `json:"max_region_nodes"`
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
	selections := make(map[NodeID]SelectionReason)
	repairSeeds := make(map[NodeID]bool)
	add := func(id NodeID, reason SelectionReason) {
		if id != "" && candidate.Graph.NodeByID(id) != nil {
			if _, exists := selections[id]; !exists {
				selections[id] = reason
			}
		}
	}
	for _, diagnostic := range evidence.Diagnostics {
		add(diagnostic.RelatedNode, SelectionDiagnostic)
		if diagnostic.RelatedNode != "" {
			repairSeeds[diagnostic.RelatedNode] = true
		}
	}
	if evidence.Execution != nil && evidence.Execution.RuntimeFailure != nil {
		failure := evidence.Execution.RuntimeFailure
		if failure.Code == "CAPABILITY_DENIED" || failure.Code == "LIMIT_EXCEEDED" {
			return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_AUTHORITY", "runtime authority failures are not semantic repair targets")}
		}
		if failure.NodeID == "" || candidate.Graph.NodeByID(NodeID(failure.NodeID)) == nil {
			return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_PROVENANCE", "runtime failure has no verified HFIR origin")}
		}
		add(NodeID(failure.NodeID), SelectionRuntime)
		repairSeeds[NodeID(failure.NodeID)] = true
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
		dependencies := dataClosure(candidate.Graph, anchor, 2)
		for _, dependency := range dependencies {
			add(dependency, SelectionDependency)
		}
		writers := lastWriters(candidate.Graph, evidence.Execution.Trace, anchor)
		for _, writer := range writers {
			add(writer, SelectionLastWriter)
			repairSeeds[writer] = true
			for _, dependency := range dataClosure(candidate.Graph, writer, 1) {
				add(dependency, SelectionDependency)
				repairSeeds[dependency] = true
			}
		}
		if len(writers) == 0 {
			repairSeeds[anchor] = true
			for _, dependency := range dependencies {
				repairSeeds[dependency] = true
			}
		}
		for _, condition := range executedBranchConditions(candidate.Graph, evidence.Execution.Trace, anchor) {
			add(condition, SelectionBranchControl)
			repairSeeds[condition] = true
		}
	}
	if len(selections) == 0 {
		return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_UNAVAILABLE", "failure evidence does not identify a semantic node")}
	}
	targets := sortedNodeIDs(repairSeeds)
	if len(targets) == 0 {
		return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_UNAVAILABLE", "failure evidence did not yield repair seeds")}
	}
	context, err := NewSemanticRepairContext(candidate, targets)
	if err != nil {
		return CandidateRepairRegion{}, []Diagnostic{localizationDiagnostic("HFIR_LOCALIZATION_BOUNDS", err.Error())}
	}
	result := CandidateRepairRegion{GraphHash: candidate.Hash, GraphVersion: candidate.Graph.Version, Context: context, MaxRegionNodes: maxRepairRegionNodes}
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

func lastWriters(graph *Graph, trace []bytecode.ExecutionTraceEvent, anchor NodeID) []NodeID {
	anchorNode := graph.NodeByID(anchor)
	if anchorNode == nil {
		return nil
	}
	var resource string
	if anchorNode.Kind == "print" && len(anchorNode.DataInputs) == 1 {
		value := graph.NodeByID(anchorNode.DataInputs[0].SourceNode)
		if value != nil && value.Kind == "map_get" {
			resource = value.Value
		}
	}
	if resource == "" {
		return nil
	}
	for index := len(trace) - 1; index >= 0; index-- {
		node := graph.NodeByID(NodeID(trace[index].NodeID))
		if node != nil && (node.Kind == "map_set" || node.Kind == "map_delete") && node.Value == resource {
			return []NodeID{node.ID}
		}
	}
	return nil
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
