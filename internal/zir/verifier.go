package zir

import (
	"zero/internal/ast"
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
		capReq := getCapability(node.Kind)
		if capReq != "" {
			hasCap := false
			for _, eff := range node.Effects {
				if eff.Capability == capReq {
					hasCap = true
					break
				}
			}
			if !hasCap {
				node.Effects = append(node.Effects, Effect{Type: "capability", Capability: capReq})
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
		// never reaches ZIR) and case two is common - confirmed empirically
		// by running this check against the full tests/*.zero corpus, where
		// it false-positived on 12 of ~90 fixtures (bugs.md #42). The
		// production gate (zero.go's runZirGate) therefore does NOT treat
		// this code as blocking; it is still computed and returned here
		// unchanged so any future consumer with better information can use
		// it, but do not add it to zero.go's zirBlockingCodes without first
		// fixing the underlying checker/ZIR information gap in #41.
		if node.Kind == "symbol" && node.Type.Kind == ast.Unknown {
			v.addDiagnostic("ZIR_UNBOUND_REF", SeverityError, "Unbound variable or function reference: "+node.Value, node.Provenance, node.ID)
		}

		// 3. Cycle & Reference Validation
		for _, dataEdge := range node.DataInputs {
			if _, exists := v.Graph.nodeMap[dataEdge.SourceNode]; !exists {
				v.addDiagnostic("ZIR_INVALID_REF", SeverityError, "Invalid data input reference to missing node", node.Provenance, node.ID)
			}
		}
		for _, ctrlEdge := range node.ControlEdges {
			if _, exists := v.Graph.nodeMap[ctrlEdge]; !exists {
				v.addDiagnostic("ZIR_INVALID_REF", SeverityError, "Invalid control edge reference to missing node", node.Provenance, node.ID)
			}
		}

		// 4. Target Feasibility
		if v.Target != "" && !isFeasible(node.Kind, v.Target) {
			v.addDiagnostic("ZIR_TARGET_INFEASIBLE", SeverityError, "Node kind '"+node.Kind+"' is not feasible for target '"+v.Target+"'", node.Provenance, node.ID)
		}
	}

	return v.Diagnostics
}

// getCapability is a hardcoded, ZIR-local capability mapping. It duplicates
// internal/bytecode/opcode.go's OpInfo table, which is the capability source
// of truth for the -allow-caps/-run-bc bytecode VM path. The two are not
// wired together (see improvement #94 in improvements.md, filed alongside
// this comment): merging them would mean either importing internal/bytecode
// into internal/zir (a backwards layering dependency - zir is meant to be
// target/backend-independent) or refactoring bytecode.go's opcode switch,
// both out of scope for this phase. Keep this list in sync by hand until #94
// lands.
func getCapability(kind string) string {
	switch kind {
	case "db_connect", "sql_query", "store_open", "store_put", "store_get", "store_delete":
		return "database"
	case "fetch", "res", "res_json", "http_server_start", "http_route", "http_server_serve", "llm_generate", "achieve":
		return "network"
	case "read_file", "write_file", "mkdir":
		return "filesystem"
	case "exec", "spawn", "spawn_agent":
		return "process"
	case "env":
		return "environment"
	}
	return ""
}

// isFeasible has real rejection rules only for target == "wasm" today. Every
// other target identity (including ones introduced by zero.go's production
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
