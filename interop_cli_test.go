package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// flagsDefinedIn returns the flag names registered in fn: calls such as
// fs.String("name", ...) on a FlagSet variable, or flag.Bool("name", ...)
// on the package-level set when pkgLevel is true.
func flagsDefinedIn(t *testing.T, file *ast.File, fn string, pkgLevel bool) []string {
	t.Helper()
	var names []string
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != fn {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch sel.Sel.Name {
			case "String", "Bool", "Int", "Int64", "Duration", "Float64", "Uint":
			default:
				return true
			}
			recv, ok := sel.X.(*ast.Ident)
			if !ok || (recv.Name == "flag") != pkgLevel {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			name, _ := strconv.Unquote(lit.Value)
			names = append(names, name)
			return true
		})
	}
	sort.Strings(names)
	return names
}

func trimmedFlags(flags []string, skip ...string) []string {
	var out []string
	for _, f := range flags {
		name := strings.TrimLeft(f, "-")
		keep := true
		for _, s := range skip {
			keep = keep && name != s
		}
		if keep {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func TestInteropDescriptorMatchesFlagSets(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "howlframe.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]interopCommand{}
	for _, c := range interopCommands {
		byPath[strings.Join(c.Path, " ")] = c
	}
	for path, fn := range map[string]string{"check": "checkSource", "build": "buildSource", "run": "runArtifact", "inspect": "inspectArtifact"} {
		got := trimmedFlags(byPath[path].Flags)
		want := flagsDefinedIn(t, file, fn, false)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("interop %q flags = %v, %s defines %v", path, got, fn, want)
		}
	}
	// --version is answered before flag parsing, so it is not in the set.
	got := trimmedFlags(byPath[""].Flags, "version")
	want := flagsDefinedIn(t, file, "main", true)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("interop global flags = %v, main defines %v", got, want)
	}

	// Every subcommand runSubcommand dispatches is described, and nothing else.
	var dispatched []string
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "runSubcommand" {
			continue
		}
		sw := fd.Body.List[len(fd.Body.List)-1].(*ast.SwitchStmt)
		for _, stmt := range sw.Body.List {
			for _, e := range stmt.(*ast.CaseClause).List {
				name, _ := strconv.Unquote(e.(*ast.BasicLit).Value)
				dispatched = append(dispatched, name)
			}
		}
	}
	var described []string
	for path := range byPath {
		if path != "" {
			described = append(described, path)
		}
	}
	sort.Strings(dispatched)
	sort.Strings(described)
	if strings.Join(dispatched, ",") != strings.Join(described, ",") {
		t.Errorf("runSubcommand dispatches %v, descriptor describes %v", dispatched, described)
	}
}

func TestInteropCommandPrintsAValidDescriptor(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "howlframe")
	if out, err := exec.Command("go", "build", "-o", bin, "howlframe.go").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "interop", "--json").Output()
	if err != nil {
		t.Fatalf("interop --json: %v", err)
	}
	var d struct {
		Schema, Component, Version string
		HFBCFormat                 string `json:"hfbc_format"`
		Commands                   []struct {
			Path  []string
			Flags []string
			JSON  *bool `json:"json"`
			Emits []string
		}
		ExitCodes map[string]string `json:"exit_codes"`
	}
	if err := json.Unmarshal(out, &d); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if d.Schema != "howl.interop/v1" || d.Component != "howlframe" || d.Version != Version || d.HFBCFormat != HFBCFormatVersion {
		t.Fatalf("header = %+v", d)
	}
	flagRe := regexp.MustCompile(`^--?[A-Za-z0-9][A-Za-z0-9_-]*$`)
	schemaRe := regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+/v[0-9]+$`)
	for _, c := range d.Commands {
		if c.Path == nil || c.Flags == nil || c.JSON == nil {
			t.Errorf("command %v lacks path, flags, or json", c.Path)
		}
		for _, f := range c.Flags {
			if !flagRe.MatchString(f) {
				t.Errorf("command %v: bad flag %q", c.Path, f)
			}
		}
		for _, s := range c.Emits {
			if !schemaRe.MatchString(s) {
				t.Errorf("command %v: bad schema id %q", c.Path, s)
			}
		}
	}
	if len(d.ExitCodes) == 0 {
		t.Error("no exit codes")
	}
	if err := exec.Command(bin, "interop", "--bogus").Run(); err == nil {
		t.Error("interop accepted an unknown argument")
	}
}
