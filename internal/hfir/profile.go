package hfir

import (
	"fmt"
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/construct"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"os"
	"path/filepath"
	"strings"
)

// VerifyProfile runs before include expansion, checking imported sources under
// the original input directory (including symlink resolution), then checks code
// positions with the same structural traversal as construct verification.
func VerifyProfile(root *ast.Node, profile construct.Profile, inputFile, target string) []Diagnostic {
	if profile == construct.ProfileDefault || root == nil {
		return nil
	}
	var out []Diagnostic
	add := func(name string, n *ast.Node, file string) {
		out = append(out, Diagnostic{Code: "PROFILE_FORBIDDEN_CONSTRUCT", Severity: SeverityError, Message: fmt.Sprintf("construct %q is forbidden under profile %q", name, profile), Location: Provenance{Filename: file, Line: n.Line, Column: n.Column}, ContractVersion: DiagnosticContractVersion, Target: target})
	}
	if target == "go" || target == "js" || target == "javascript" {
		add(target+" target", root, inputFile)
	}
	rootDir, _ := filepath.Abs(filepath.Dir(inputFile))
	if resolved, err := filepath.EvalSymlinks(rootDir); err == nil {
		rootDir = resolved
	}
	seen := map[string]bool{}
	var visit func(*ast.Node, string, int)
	visit = func(tree *ast.Node, file string, depth int) {
		if tree == nil || depth > 100 {
			return
		}
		for _, v := range construct.ScanProfile(tree, profile) {
			sourceFile := file
			if v.Filename != "" {
				sourceFile = v.Filename
			}
			add(v.Name, &ast.Node{Line: v.Line, Column: v.Column}, sourceFile)
		}
		var imports func(*ast.Node)
		imports = func(n *ast.Node) {
			if n == nil || n.Type != "List" || len(n.Children) == 0 {
				return
			}
			head := n.Children[0].Value
			if (head == "include" || head == "use") && len(n.Children) > 1 {
				path := filepath.Join(filepath.Dir(file), n.Children[1].Value)
				abs, _ := filepath.Abs(path)
				sourcePath := abs
				if resolved, err := filepath.EvalSymlinks(abs); err == nil {
					abs = resolved
				}
				rel, err := filepath.Rel(rootDir, abs)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					add(head, n, file)
					return
				}
				if !seen[sourcePath] {
					seen[sourcePath] = true
					if data, err := os.ReadFile(sourcePath); err == nil {
						p := parser.NewParser(lexer.NewLexer(string(data)), filepath.Base(sourcePath))
						// Expansion resolves nested imports relative to the lexical
						// importing path, even when that file is a symlink.
						visit(p.ParseExpression(), sourcePath, depth+1)
					}
				}
				return
			}
			for _, child := range n.Children {
				imports(child)
			}
		}
		imports(tree)
	}
	visit(root, inputFile, 0)
	return out
}
