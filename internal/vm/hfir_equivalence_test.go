package vm

import (
	"bytes"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/checker"
	"github.com/howlcipher/howlframe/internal/hfir"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

type bytecodeOutcome struct {
	stdout   string
	stderr   string
	exitCode int
	vmError  *VMError
	panicVal any
}

func TestHFIRBytecodeEquivalence(t *testing.T) {
	tests := []struct {
		name   string
		source string
		caps   []capability.Capability
		stdin  string
	}{
		{
			name: "scalar control",
			source: `(cli_app
  (let (count 3)
    (do
      (set count (+ count 2))
      (if (and (> count 4) (= "ok" "ok"))
        (print "count:" (to_string count))
        (stderr "unexpected")))))`,
		},
		{
			name: "collections",
			source: `(cli_app
  (let (items (list "a" "b"))
    (let (record (dict ("status" "open") ("owner" "team")))
      (do
        (map_set record "status" "closed")
        (map_delete record "missing")
        (print (list_get items 1) (map_get record "status") (list_len items))))))`,
		},
		{
			name:   "string transforms",
			source: `(cli_app (let (parts (str_split "alpha,beta" ",")) (print (str_join parts "|"))))`,
		},
		{
			name:   "stderr and exit",
			source: `(cli_app (stderr "halt\n") (exit 7) (print "unreachable"))`,
		},
		{
			name:   "capability guarded environment",
			source: `(cli_app (print (env "HFIR_EQ_TEST_VALUE")))`,
			caps:   []capability.Capability{capability.Environment},
		},
	}

	t.Setenv("HFIR_EQ_TEST_VALUE", "expected")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, graph := checkedHFIRGraph(t, tt.source)
			legacy := bytecode.CompileToBytecode(root)
			direct, diagnostics := hfir.LowerToBytecode(graph)
			if len(diagnostics) != 0 {
				t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
			}
			legacy = roundTripArtifact(t, legacy)
			direct = roundTripArtifact(t, direct)

			legacyOutcome := runBytecodeOutcome(legacy, tt.stdin, tt.caps)
			directOutcome := runBytecodeOutcome(direct, tt.stdin, tt.caps)
			if !reflect.DeepEqual(legacyOutcome, directOutcome) {
				t.Fatalf("AST bytecode outcome = %#v\nHFIR bytecode outcome = %#v", legacyOutcome, directOutcome)
			}
		})
	}
}

func TestHFIRBytecodeCapabilityConsistencyAndDenial(t *testing.T) {
	t.Setenv("HFIR_EQ_TEST_VALUE", "expected")
	root, graph := checkedHFIRGraph(t, `(cli_app (print (env "HFIR_EQ_TEST_VALUE")))`)
	legacy := bytecode.CompileToBytecode(root)
	direct, diagnostics := hfir.LowerToBytecode(graph)
	if len(diagnostics) != 0 {
		t.Fatalf("LowerToBytecode() diagnostics = %#v", diagnostics)
	}
	if got, want := graphCapabilities(graph), programCapabilities(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("HFIR effects = %v, emitted bytecode capabilities = %v", got, want)
	}
	if got, want := programCapabilities(legacy), programCapabilities(direct); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy capabilities = %v, HFIR capabilities = %v", got, want)
	}

	legacyOutcome := runBytecodeOutcome(legacy, "", nil)
	directOutcome := runBytecodeOutcome(direct, "", nil)
	if !reflect.DeepEqual(legacyOutcome, directOutcome) {
		t.Fatalf("AST denial = %#v\nHFIR denial = %#v", legacyOutcome, directOutcome)
	}
	if legacyOutcome.vmError == nil || legacyOutcome.vmError.Code != "CAPABILITY_DENIED" {
		t.Fatalf("denial = %#v, want CAPABILITY_DENIED", legacyOutcome)
	}
}

func checkedHFIRGraph(t *testing.T, source string) (*ast.Node, *hfir.Graph) {
	t.Helper()
	parsed := parser.NewParser(lexer.NewLexer(source), "hfir_equivalence.howl")
	root := parsed.ParseExpression()
	if parsed.Cur.Type != lexer.TokenEOF {
		t.Fatal("parser did not consume source")
	}
	ast.ApplyPatches(root)
	root = ast.ApplyWithContext(root, nil)
	root = ast.ApplyWithContext(root, nil)
	checker.Check(root)
	graph, err := hfir.LowerAST(root, "hfir_equivalence.howl")
	if err != nil {
		t.Fatalf("LowerAST() error = %v", err)
	}
	if diagnostics := hfir.NewVerifier(graph, hfir.TargetBytecode).Verify(); len(diagnostics) != 0 {
		t.Fatalf("Verify() diagnostics = %#v", diagnostics)
	}
	return root, graph
}

func roundTripArtifact(t *testing.T, program *bytecode.BCProgram) *bytecode.BCProgram {
	t.Helper()
	if err := bytecode.ValidateProgram(program); err != nil {
		t.Fatalf("ValidateProgram() error = %v", err)
	}
	var artifact bytes.Buffer
	if err := bytecode.WriteArtifact(&artifact, program); err != nil {
		t.Fatalf("WriteArtifact() error = %v", err)
	}
	decoded, err := bytecode.ReadArtifact(&artifact)
	if err != nil {
		t.Fatalf("ReadArtifact() error = %v", err)
	}
	return decoded
}

func runBytecodeOutcome(program *bytecode.BCProgram, stdin string, caps []capability.Capability) (outcome bytecodeOutcome) {
	var stdout, stderr bytes.Buffer
	machine := &BCVM{
		prog:        program,
		env:         NewBcEnv(nil),
		insts:       program.Main,
		stores:      newBCStoreRegistry(),
		Limits:      DefaultLimits,
		AllowedCaps: caps,
		In:          strings.NewReader(stdin),
		Out:         &stdout,
		ErrOut:      &stderr,
	}
	defer func() {
		outcome.stdout = stdout.String()
		outcome.stderr = stderr.String()
		if recovered := recover(); recovered != nil {
			switch value := recovered.(type) {
			case VmExit:
				outcome.exitCode = value.code
			case *VMError:
				copy := *value
				outcome.vmError = &copy
			default:
				outcome.panicVal = value
			}
		}
	}()
	machine.run(machine.insts, machine.env)
	return outcome
}

func graphCapabilities(graph *hfir.Graph) []capability.Capability {
	seen := make(map[capability.Capability]bool)
	for _, node := range graph.Nodes {
		for _, effect := range node.Effects {
			if effect.Type == "capability" && effect.Capability != "" {
				seen[capability.Capability(effect.Capability)] = true
			}
		}
	}
	return sortedCapabilities(seen)
}

func programCapabilities(program *bytecode.BCProgram) []capability.Capability {
	seen := make(map[capability.Capability]bool)
	for _, inst := range program.Main {
		if spec, ok := bytecode.Registry[inst.Op]; ok && spec.Capability != capability.None {
			seen[spec.Capability] = true
		}
	}
	return sortedCapabilities(seen)
}

func sortedCapabilities(seen map[capability.Capability]bool) []capability.Capability {
	result := make([]capability.Capability, 0, len(seen))
	for cap := range seen {
		result = append(result, cap)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
