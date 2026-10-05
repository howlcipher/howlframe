package vm

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// The model-call constructs below send requests to the local model host at
// 127.0.0.1:11434 (hard-coded in the VM and the interpreter). They are network
// effects and must sit behind the network grant on both -run-bc and -run.

// modelHostProbe binds the hard-coded model host so a test can count
// requests that reach it. When the port is already taken, probing is
// disabled and the test still checks the structured denial.
type modelHostProbe struct {
	hits    atomic.Int64
	enabled bool
}

func startModelHostProbe(t *testing.T) *modelHostProbe {
	t.Helper()
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	probe := &modelHostProbe{}
	ln, err := net.Listen("tcp", "127.0.0.1:11434")
	if err != nil {
		t.Logf("model host port unavailable (%v); request counting disabled", err)
		return probe
	}
	probe.enabled = true
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probe.hits.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"response":"0.5"}`)
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return probe
}

func (p *modelHostProbe) assertNoRequests(t *testing.T, label string) {
	t.Helper()
	if p.enabled && p.hits.Load() != 0 {
		t.Fatalf("%s: denied run sent %d request(s) to the model host", label, p.hits.Load())
	}
}

// grantsWithoutNetwork is every other capability, so a denial proves the
// network grant specifically is required.
func grantsWithoutNetwork() []capability.Capability {
	var caps []capability.Capability
	for _, c := range capability.All() {
		if c != capability.Network {
			caps = append(caps, c)
		}
	}
	return caps
}

var modelCallSources = []struct {
	name   string
	source string
	opcode bytecode.Opcode
}{
	{"confidence", `(cli_app (print (confidence "the sky is blue")))`, bytecode.OpConfidence},
	{"neural_circuit", `(cli_app (print (neural_circuit ("x") "classify")))`, bytecode.OpNeuralCircuit},
	{"ephemeral_circuit", `(cli_app (print (ephemeral_circuit ("x") "classify")))`, bytecode.OpEphemeralCircuit},
}

func TestModelCallOpcodesDeclareNetwork(t *testing.T) {
	for _, tc := range modelCallSources {
		if got := bytecode.Registry[tc.opcode].Capability; got != capability.Network {
			t.Errorf("%s opcode capability = %q, want network", tc.name, got)
		}
		if got := capability.ForConstruct(tc.name); got != capability.Network {
			t.Errorf("ForConstruct(%s) = %q, want network", tc.name, got)
		}
	}
}

func TestBytecodeModelCallsDeniedWithoutNetwork(t *testing.T) {
	probe := startModelHostProbe(t)
	for _, tc := range modelCallSources {
		t.Run(tc.name, func(t *testing.T) {
			_, program := parseAndCompile(t, tc.source)
			var artifact bytes.Buffer
			if err := bytecode.WriteArtifact(&artifact, program); err != nil {
				t.Fatalf("WriteArtifact() error = %v", err)
			}
			decoded, err := bytecode.ReadArtifact(&artifact)
			if err != nil {
				t.Fatalf("ReadArtifact() error = %v", err)
			}
			for label, caps := range map[string][]capability.Capability{
				"empty grant":             nil,
				"every grant but network": grantsWithoutNetwork(),
			} {
				outcome := runBytecodeOutcome(decoded, "", caps)
				if outcome.vmError == nil || outcome.vmError.Code != "CAPABILITY_DENIED" {
					t.Fatalf("%s: outcome = %+v, want CAPABILITY_DENIED", label, outcome)
				}
				if !strings.Contains(outcome.vmError.Message, "network") {
					t.Fatalf("%s: denial message = %q, want network", label, outcome.vmError.Message)
				}
				if outcome.stdout != "" {
					t.Fatalf("%s: denied run printed %q", label, outcome.stdout)
				}
				probe.assertNoRequests(t, label)
			}
		})
	}
}

func TestBytecodeConfidenceRequestsModelWhenNetworkGranted(t *testing.T) {
	probe := startModelHostProbe(t)
	if !probe.enabled {
		t.Skip("model host port unavailable; cannot observe the granted request")
	}
	_, program := parseAndCompile(t, modelCallSources[0].source)
	outcome := runBytecodeOutcome(program, "", []capability.Capability{capability.Network})
	if outcome.vmError != nil || outcome.panicVal != nil {
		t.Fatalf("granted confidence failed: %+v", outcome)
	}
	if probe.hits.Load() != 1 {
		t.Fatalf("granted confidence sent %d request(s), want 1", probe.hits.Load())
	}
	if strings.TrimSpace(outcome.stdout) != "0.5" {
		t.Fatalf("stdout = %q, want 0.5", outcome.stdout)
	}
}

func TestInterpretModelCallsDeniedWithoutNetwork(t *testing.T) {
	probe := startModelHostProbe(t)
	sources := map[string]string{
		"confidence":        modelCallSources[0].source,
		"neural_circuit":    modelCallSources[1].source,
		"ephemeral_circuit": modelCallSources[2].source,
		"lazy_synthesize":   `(cli_app (lazy_synthesize my_add (a b) "Returns the sum of a and b") (print (call my_add 1 2)))`,
	}
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			node, _ := parseAndCompile(t, source)
			for label, caps := range map[string][]capability.Capability{
				"empty grant":             nil,
				"every grant but network": grantsWithoutNetwork(),
			} {
				var stdout, stderr bytes.Buffer
				exitCode := Interpret(node, nil, caps, strings.NewReader(""), &stdout, &stderr)
				if exitCode == 0 {
					t.Fatalf("%s: expected denial, got exit 0; stdout=%q", label, stdout.String())
				}
				if !strings.Contains(stderr.String(), "capability denied: network") {
					t.Fatalf("%s: stderr = %q, want capability denied: network", label, stderr.String())
				}
				if stdout.String() != "" {
					t.Fatalf("%s: denied run printed %q", label, stdout.String())
				}
				probe.assertNoRequests(t, label)
			}
		})
	}
}
