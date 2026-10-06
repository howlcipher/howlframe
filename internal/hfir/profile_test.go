package hfir

import (
	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/construct"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGovernedProfile(t *testing.T) {
	for _, e := range construct.GovernedExcluded {
		t.Run(e.Name, func(t *testing.T) {
			if _, ok := construct.Lookup(e.Name); !ok {
				t.Fatal("registry drift")
			}
			p := parser.NewParser(lexer.NewLexer("(cli_app \"x\"\n ("+e.Name+"))"), "x.howl")
			root := p.ParseExpression()
			d := VerifyProfile(root, construct.ProfileGoverned, "x.howl", "")
			if len(d) != 1 || d[0].Code != "PROFILE_FORBIDDEN_CONSTRUCT" || !strings.Contains(d[0].Message, e.Name) || d[0].Location.Line != 2 {
				t.Fatal(d)
			}
			if len(VerifyProfile(root, construct.ProfileDefault, "x.howl", "go")) != 0 {
				t.Fatal("default changed")
			}
		})
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "inside.howl"), []byte("(do (print 1))"), 0600)
	for _, tt := range []struct {
		path string
		bad  bool
	}{{"inside.howl", false}, {"../outside.howl", true}} {
		p := parser.NewParser(lexer.NewLexer(`(cli_app "x" (include "`+tt.path+`"))`), "x.howl")
		d := VerifyProfile(p.ParseExpression(), construct.ProfileGoverned, filepath.Join(dir, "x.howl"), "")
		if (len(d) > 0) != tt.bad {
			t.Fatal(d)
		}
	}
	for _, target := range []string{"go", "js", "javascript"} {
		p := parser.NewParser(lexer.NewLexer(`(cli_app "x" (print 1))`), "x.howl")
		if len(VerifyProfile(p.ParseExpression(), construct.ProfileGoverned, "x.howl", target)) != 1 {
			t.Fatal(target)
		}
	}
	if _, err := construct.LookupProfile("unknown"); err == nil {
		t.Fatal("unknown accepted")
	}
}

func TestGovernedImportAndStructuralBoundaries(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "nested")
	if err := os.Mkdir(inner, 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(inner, "a.howl"), []byte(`(module (include "../../outside.howl"))`), 0600)
	parse := func(s string) *ast.Node { return parser.NewParser(lexer.NewLexer(s), "input.howl").ParseExpression() }
	d := VerifyProfile(parse(`(cli_app (include "nested/a.howl"))`), construct.ProfileGoverned, filepath.Join(dir, "input.howl"), "")
	if len(d) != 1 || !strings.Contains(d[0].Message, "include") || !strings.HasSuffix(d[0].Location.Filename, "a.howl") {
		t.Fatal(d)
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "external.howl"), []byte(`(do (print 1))`), 0600)
	if err := os.Symlink(filepath.Join(outside, "external.howl"), filepath.Join(dir, "link.howl")); err != nil {
		t.Fatal(err)
	}
	d = VerifyProfile(parse(`(cli_app (include "link.howl"))`), construct.ProfileGoverned, filepath.Join(dir, "input.howl"), "")
	if len(d) != 1 {
		t.Fatal(d)
	}
	for _, s := range []string{`(cli_app (let (exec 1) (print exec)))`, `(cli_app (defun f (exec) (print exec)))`, `(cli_app (dict ("exec" 1)))`} {
		if d := VerifyProfile(parse(s), construct.ProfileGoverned, "input.howl", ""); len(d) > 0 {
			t.Fatalf("structural false positive: %s %+v", s, d)
		}
	}
	d = VerifyProfile(parse(`(cli_app (with_context (x) (exec "echo" "x")))`), construct.ProfileGoverned, "input.howl", "")
	if len(d) != 1 || !strings.Contains(d[0].Message, "exec") {
		t.Fatal(d)
	}
}

func TestGovernedSymlinkUsesLexicalImportDirectory(t *testing.T) {
	dir := t.TempDir()
	rootDir := filepath.Join(dir, "root")
	nested := filepath.Join(rootDir, "nested")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(nested, "a.howl"), []byte(`(module (include "../outside.howl"))`), 0600)
	os.WriteFile(filepath.Join(dir, "outside.howl"), []byte(`(do (print 1))`), 0600)
	if err := os.Symlink(filepath.Join(nested, "a.howl"), filepath.Join(rootDir, "link.howl")); err != nil {
		t.Fatal(err)
	}
	p := parser.NewParser(lexer.NewLexer(`(cli_app (include "link.howl"))`), "input.howl")
	d := VerifyProfile(p.ParseExpression(), construct.ProfileGoverned, filepath.Join(rootDir, "input.howl"), "")
	if len(d) != 1 || !strings.Contains(d[0].Message, "include") {
		t.Fatalf("lexical outside-root import was missed: %+v", d)
	}
}
