// Package optimization compiles checked optimize_signature declarations into
// deterministic metadata plans for external optimization tooling.
package optimization

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/checker"
)

// FormatV1 identifies the serialized optimization plan schema.
const FormatV1 = "howlframe.optimization_plan/v1"

// CandidatePlan is one labeled prompt or program variant.
type CandidatePlan struct {
	Label   string `json:"label"`
	Payload string `json:"payload"`
}

// SignaturePlan describes optimization intent without executing any command or
// changing source code.
type SignaturePlan struct {
	Name       string          `json:"name"`
	Metric     string          `json:"metric"`
	Tests      []string        `json:"tests"`
	Candidates []CandidatePlan `json:"candidates"`
	Line       int             `json:"line"`
	Column     int             `json:"column"`
	BodyType   ast.ValueKind   `json:"body_type"`
}

// ProgramPlan contains all checked optimization signatures in source order.
type ProgramPlan struct {
	Format     string          `json:"format"`
	Signatures []SignaturePlan `json:"signatures"`
}

// CompileAnalysis creates a deterministic, side-effect-free optimization plan.
func CompileAnalysis(analysis *checker.Analysis) ProgramPlan {
	plan := ProgramPlan{
		Format:     FormatV1,
		Signatures: []SignaturePlan{},
	}
	if analysis == nil {
		return plan
	}

	for _, signature := range analysis.OptimizationSignatures {
		candidates := make([]CandidatePlan, 0, len(signature.Candidates))
		for _, candidate := range signature.Candidates {
			candidates = append(candidates, CandidatePlan{
				Label:   candidate.Label,
				Payload: candidate.Payload,
			})
		}
		tests := append([]string(nil), signature.Tests...)
		if tests == nil {
			tests = []string{}
		}
		plan.Signatures = append(plan.Signatures, SignaturePlan{
			Name:       signature.Name,
			Metric:     signature.Metric,
			Tests:      tests,
			Candidates: candidates,
			Line:       signature.Line,
			Column:     signature.Column,
			BodyType:   signature.BodyType.Kind,
		})
	}
	return plan
}
