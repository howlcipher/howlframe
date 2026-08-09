package hfir

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/construct"
)

// TargetBytecode is the verifier target identity for the standalone bytecode
// runtime. howlframe.go's hfirTargetBytecode must spell it the same way; the two are
// tied together by TestVerifyConstructsMatchesBytecodeTargetName.
const TargetBytecode = "bytecode"

// VerifyConstructs reports every construct in root that the given target
// cannot execute, as HFIR_TARGET_INFEASIBLE diagnostics in source order.
//
// This deliberately runs over the AST rather than over Graph node kinds.
// LowerAST derives a node's Kind from the head symbol of every list, including
// lists that are not constructs at all - a (let (x 0) body) binding lowers to
// a node with Kind "x", and a try_let's catch clause lowers to a node with
// Kind "catch" even though internal/bytecode's try_let case destructures it
// directly and it never reaches compileNode in head position. A rule matching
// on Kind would therefore reject correct programs. internal/construct's Scan
// keeps the parent and child position the AST still carries, so the
// classification is exact (bugs.md #45).
//
// Only the bytecode target has construct-support rules today. Every other
// target identity - including the empty target used by -validate, which is
// target-independent by contract - returns no diagnostics.
func VerifyConstructs(root *ast.Node, target string) []Diagnostic {
	if target != TargetBytecode || root == nil {
		return nil
	}

	violations := construct.Scan(root)
	if len(violations) == 0 {
		return nil
	}

	diags := make([]Diagnostic, 0, len(violations))
	for _, violation := range violations {
		diags = append(diags, Diagnostic{
			Code:     "HFIR_TARGET_INFEASIBLE",
			Severity: SeverityError,
			Message:  violation.Message(),
			Location: Provenance{
				Filename: root.Filename,
				Line:     violation.Line,
				Column:   violation.Column,
			},
			ContractVersion: DiagnosticContractVersion,
			Target:          target,
		})
	}
	return diags
}
