package vm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

func TestFilesystemScopesVMAndInterpreter(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	insideFile := filepath.Join(root, "input")
	outsideFile := filepath.Join(outside, "input")
	for _, path := range []string{insideFile, outsideFile} {
		if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	read, _ := capability.ParseGrant("filesystem:read=" + root)
	write, _ := capability.ParseGrant("filesystem:write=" + root)
	cases := []struct {
		name, body string
		grants     []capability.Capability
		denied     bool
	}{
		{"read inside", fmt.Sprintf("(read_file %q)", insideFile), []capability.Capability{read}, false},
		{"read outside", fmt.Sprintf("(read_file %q)", outsideFile), []capability.Capability{read}, true},
		{"write outside", fmt.Sprintf("(write_file %q \"changed\")", outsideFile), []capability.Capability{write}, true},
		{"mkdir outside", fmt.Sprintf("(mkdir %q)", filepath.Join(outside, "new")), []capability.Capability{write}, true},
		{"parent escape", fmt.Sprintf("(read_file %q)", root+"/../"+filepath.Base(outside)+"/input"), []capability.Capability{read}, true},
		{"read only", fmt.Sprintf("(write_file %q \"changed\")", insideFile), []capability.Capability{read}, true},
		{"write only", fmt.Sprintf("(read_file %q)", insideFile), []capability.Capability{write}, true},
		{"write inside", fmt.Sprintf("(write_file %q \"ok\")", filepath.Join(root, "output")), []capability.Capability{write}, false},
		{"mkdir inside", fmt.Sprintf("(mkdir %q)", filepath.Join(root, "new", "nested")), []capability.Capability{write}, false},
		{"coarse alias", fmt.Sprintf("(read_file %q)", outsideFile), []capability.Capability{capability.Filesystem}, false},
		{"symlink read", fmt.Sprintf("(read_file %q)", filepath.Join(root, "link", "input")), []capability.Capability{read}, true},
		{"symlink write", fmt.Sprintf("(write_file %q \"changed\")", filepath.Join(root, "link", "input")), []capability.Capability{write}, true},
		{"symlink mkdir", fmt.Sprintf("(mkdir %q)", filepath.Join(root, "link", "new")), []capability.Capability{write}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node := parser.NewParser(lexer.NewLexer("(cli_app "+tc.body+")"), "scope.howl").ParseExpression()
			var out, stderr bytes.Buffer
			ev := RunBytecodeWithEvidence(bytecode.CompileToBytecode(node), nil, DefaultExecutionPolicy(), tc.grants, strings.NewReader(""), &out, &stderr, 0)
			if tc.denied {
				if ev.RuntimeFailure == nil || ev.RuntimeFailure.Code != "CAPABILITY_DENIED" {
					t.Fatalf("expected denial: %+v", ev.RuntimeFailure)
				}
			} else if ev.RuntimeFailure != nil {
				t.Fatalf("unexpected failure: %+v", ev.RuntimeFailure)
			}
			stderr.Reset()
			code := Interpret(node, nil, tc.grants, strings.NewReader(""), &out, &stderr)
			if (code != 0) != tc.denied || tc.denied && !strings.Contains(stderr.String(), "capability denied: filesystem") {
				t.Fatalf("interpreter code %d: %s", code, &stderr)
			}
		})
	}
	for _, path := range []string{insideFile, outsideFile} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "original" {
			t.Fatalf("denied write changed %s: %q %v", path, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "new")); !os.IsNotExist(err) {
		t.Fatalf("denied mkdir created directory: %v", err)
	}
}

func TestFileStoreFilesystemScopes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	read, _ := capability.ParseGrant("filesystem:read=" + root)
	write, _ := capability.ParseGrant("filesystem:write=" + root)
	otherWrite, _ := capability.ParseGrant("filesystem:write=" + outside)
	otherRead, _ := capability.ParseGrant("filesystem:read=" + outside)
	for _, tc := range []struct {
		name   string
		grants []capability.Capability
		action string
		denied bool
	}{
		{"put inside", []capability.Capability{read, write}, `(store_put db "k" (dict ("v" 1)))`, false},
		{"put outside write root", []capability.Capability{read, otherWrite}, `(store_put db "k" (dict ("v" 2)))`, true},
		{"delete outside write root", []capability.Capability{read, otherWrite}, `(store_delete db "k")`, true},
		{"get read only", []capability.Capability{read}, `(store_get db "k")`, false},
		{"keys read only", []capability.Capability{read}, `(store_keys db)`, false},
		{"open outside read root", []capability.Capability{otherRead, write}, `(store_keys db)`, true},
		{"open write only", []capability.Capability{write}, `(store_keys db)`, true},
		{"delete inside", []capability.Capability{read, write}, `(store_delete db "k")`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, "store.json")
			before, _ := os.ReadFile(path)
			source := fmt.Sprintf("(cli_app (store_open db %q) %s)", "file://"+path, tc.action)
			node := parser.NewParser(lexer.NewLexer(source), "store-scope.howl").ParseExpression()
			caps := append([]capability.Capability{capability.Database}, tc.grants...)
			var out, stderr bytes.Buffer
			ev := RunBytecodeWithEvidence(bytecode.CompileToBytecode(node), nil, DefaultExecutionPolicy(), caps, nil, &out, &stderr, 0)
			if tc.denied {
				if ev.RuntimeFailure == nil || ev.RuntimeFailure.Code != "CAPABILITY_DENIED" {
					t.Fatalf("expected denial: %+v", ev.RuntimeFailure)
				}
				after, _ := os.ReadFile(path)
				if !bytes.Equal(before, after) {
					t.Fatal("denial changed store")
				}
			} else if ev.RuntimeFailure != nil {
				t.Fatalf("unexpected failure: %+v", ev.RuntimeFailure)
			}
		})
	}
}

func TestFilesystemScopeReceiptDecision(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	read, _ := capability.ParseGrant("filesystem:read=" + root)
	write, _ := capability.ParseGrant("filesystem:write=" + root)
	for _, tc := range []struct {
		source, op string
		denied     bool
	}{
		{fmt.Sprintf("(cli_app (read_file %q))", filepath.Join(outside, "missing")), "READ_FILE", true},
		{fmt.Sprintf("(cli_app (mkdir %q))", filepath.Join(root, "new")), "MKDIR", false},
		{fmt.Sprintf(`(cli_app (store_open db %q) (store_put db "k" (dict ("v" 1))))`, "file://"+filepath.Join(root, "store.json")), "STORE_PUT", true},
	} {
		caps := []capability.Capability{capability.Database, read}
		if tc.op == "MKDIR" {
			caps = append(caps, write)
		}
		receipt, _, _ := receiptRun(t, tc.source, DefaultExecutionPolicy(), caps...)
		found := false
		for _, effect := range receipt.Effects {
			if effect.Op == tc.op && effect.Capability == "filesystem" {
				found = true
				want := "allowed"
				if tc.denied {
					want = "denied"
				}
				if effect.Decision != want {
					t.Fatalf("%s recorded %s, want %s", tc.op, effect.Decision, want)
				}
			}
		}
		if !found {
			t.Fatalf("missing %s filesystem decision: %+v", tc.op, receipt)
		}
	}
}
