package hfir

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/capability"
)

type Verifier struct {
	Graph       *Graph
	Diagnostics []Diagnostic
	Target      string
}

func NewVerifier(g *Graph, target string) *Verifier {
	return &Verifier{
		Graph:       g,
		Diagnostics: make([]Diagnostic, 0),
		Target:      target,
	}
}

func (v *Verifier) addDiagnostic(code string, sev Severity, msg string, loc Provenance, related NodeID) {
	v.Diagnostics = append(v.Diagnostics, Diagnostic{
		Code:            code,
		Severity:        sev,
		Message:         msg,
		Location:        loc,
		RelatedNode:     related,
		ContractVersion: DiagnosticContractVersion,
		Target:          v.Target,
	})
}

func (v *Verifier) Verify() []Diagnostic {
	v.Diagnostics = make([]Diagnostic, 0)

	for _, node := range v.Graph.Nodes {
		// 1. Effect Inference & Capability Requirements (#87 / #79)
		capReq := capability.ForConstruct(node.Kind)
		if string(capReq) != "" {
			hasCap := false
			for _, eff := range node.Effects {
				if eff.Capability == string(capReq) {
					hasCap = true
					break
				}
			}
			if !hasCap {
				node.Effects = append(node.Effects, Effect{Type: "capability", Capability: string(capReq)})
			}
		}

		// 2. Unbound Reference Check (#77)
		//
		// checker.Analysis.infer (internal/checker/types.go) sets a bare
		// SYMBOL's Inferred type to ast.Unknown for two different reasons: a
		// genuinely undefined identifier (which already fails checker.Check
		// with its own diagnostic before this ever runs), and a variable
		// legitimately bound via try_let/catch or bound to the result of a
		// dynamically-typed primitive (llm_generate, generics, an HTTP lambda
		// parameter's field access, etc.) - a common, intentional pattern the
		// checker allows without a diagnostic. The checked AST's TypeInfo
		// does not preserve which case produced Unknown, so on real programs
		// this diagnostic has no reachable true positive today (case one
		// never reaches HFIR) and case two is common - confirmed empirically
		// by running this check against the full tests/*.howl corpus, where
		// it false-positived on 12 of ~90 fixtures (bugs.md #42). The
		// production gate (howlframe.go's runHFIRGate) therefore does NOT treat
		// this code as blocking; it is still computed and returned here
		// unchanged so any future consumer with better information can use
		// it, but do not add it to howlframe.go's hfirBlockingCodes without first
		// fixing the underlying checker/HFIR information gap in #41.
		if node.Kind == "symbol" && node.Type.Kind == ast.Unknown {
			v.addDiagnostic("HFIR_UNBOUND_REF", SeverityError, "Unbound variable or function reference: "+node.Value, node.Provenance, node.ID)
		}

		// 3. Cycle & Reference Validation
		for _, dataEdge := range node.DataInputs {
			if _, exists := v.Graph.nodeMap[dataEdge.SourceNode]; !exists {
				v.addDiagnostic("HFIR_INVALID_REF", SeverityError, "Invalid data input reference to missing node", node.Provenance, node.ID)
			}
		}
		for _, ctrlEdge := range node.ControlEdges {
			if _, exists := v.Graph.nodeMap[ctrlEdge]; !exists {
				v.addDiagnostic("HFIR_INVALID_REF", SeverityError, "Invalid control edge reference to missing node", node.Provenance, node.ID)
			}
		}

		// 4. Target Feasibility
		if v.Target != "" && !isFeasible(node.Kind, v.Target) {
			v.addDiagnostic("HFIR_TARGET_INFEASIBLE", SeverityError, "Node kind '"+node.Kind+"' is not feasible for target '"+v.Target+"'", node.Provenance, node.ID)
		}
	}

	return v.Diagnostics
}

// isFeasible has real rejection rules only for target == "wasm" today. Every
// other target identity (including ones introduced by howlframe.go's production
// wiring in improvement #87 Phase 2, e.g. "bytecode"/"interpreter"/"go"/
// "javascript") is permissive by default - a passing result for those
// targets does not mean real per-target feasibility coverage exists yet.
func isFeasible(kind string, target string) bool {
	if target == "wasm" {
		// Wasm backend doesn't support complex networking, process, or file IO natively without imports
		// For the sake of the verifier, we flag them if needed, but we'll leave it permissive for now.
		switch kind {
		case "exec", "spawn_agent", "http_server_start":
			return false
		}
	}
	return true
}
