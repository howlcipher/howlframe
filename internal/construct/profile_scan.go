package construct

import "github.com/howlcipher/howlframe/internal/ast"

// ScanProfile shares the compiler-position traversal used by Scan.
func ScanProfile(root *ast.Node, profile Profile) []Violation {
	var out []Violation
	if profile == ProfileGoverned {
		scanNode(root, "", &out, profile)
	}
	return out
}
