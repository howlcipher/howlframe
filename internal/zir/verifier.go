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
		Code:        code,
		Severity:    sev,
		Message:     msg,
		Location:    loc,
		RelatedNode: related,
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
