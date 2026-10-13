package hfir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/vm"
)

func blockerDAG(depth int, shared bool) candidateTransport {
	t := candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program"}
	t.Nodes = append(t.Nodes, testNode("n0", "const", "1", "INT", nil))
	for i := 1; i <= depth; i++ {
		edges := []transportEdge{{Role: "left", NodeID: NodeID(fmt.Sprintf("n%d", i-1))}, {Role: "right", NodeID: "n0"}}
		if shared {
			edges[1].NodeID = edges[0].NodeID
		}
		t.Nodes = append(t.Nodes, testNode(NodeID(fmt.Sprintf("n%d", i)), "binary", "+", "", edges))
	}
	t.Nodes = append(t.Nodes, testNode("print", "print", "", "", []transportEdge{{Role: "value", NodeID: NodeID(fmt.Sprintf("n%d", depth))}}), testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "print"}}))
	return t
}

func TestAdapterSharedDepth22Bounded(t *testing.T) {
	data, _ := json.Marshal(blockerDAG(22, true))
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	candidate, ds := DecodeCandidate(data)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	p, ds := CompileCandidate(candidate)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	runtime.ReadMemStats(&after)
	t.Logf("depth-22 lowering: %s, %d allocated bytes", time.Since(start), after.TotalAlloc-before.TotalAlloc)
	if time.Since(start) > time.Second || after.TotalAlloc-before.TotalAlloc > 32<<20 {
		t.Fatalf("unbounded lowering: %s, %d bytes", time.Since(start), after.TotalAlloc-before.TotalAlloc)
	}
	count := len(p.Main)
	for _, f := range p.Functions {
		count += len(f.Instructions)
	}
	if count > 150 {
		t.Fatalf("expanded shared DAG: %d instructions", count)
	}
	// A small shared DAG proves the outlined calls preserve the arithmetic result.
	c := mustCandidate(t, blockerDAG(4, true))
	p, ds = CompileCandidate(c)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	var out, errout bytes.Buffer
	ev := vm.RunBytecodeWithEvidence(p, nil, vm.DefaultExecutionPolicy(), nil, strings.NewReader(""), &out, &errout, 0)
	if ev.RuntimeFailure != nil || out.String() != "16\n" {
		t.Fatalf("semantics: %#v %q", ev, out.String())
	}
}

func TestAdapterDepthCapRejects4095Chain(t *testing.T) {
	for _, depth := range []int{2048, 4092, 4093} {
		data, _ := json.Marshal(blockerDAG(depth, false))
		start := time.Now()
		c, ds := DecodeCandidate(data)
		if c.Graph != nil || len(ds) != 1 || ds[0].Code != "HFIR_TRANSPORT_TOO_DEEP" {
			t.Fatalf("depth %d: %#v", depth, ds)
		}
		if time.Since(start) > time.Second {
			t.Fatal("depth rejection too slow")
		}
	}
}

func TestAdapterExpansionCap(t *testing.T) {
	// Shared allocation expressions are deliberately not outlined.
	tr := blockerDAG(17, true)
	for i := 1; i < len(tr.Nodes)-2; i++ {
		tr.Nodes[i].Kind = "list"
		tr.Nodes[i].Value = ""
		for j := range tr.Nodes[i].Inputs {
			tr.Nodes[i].Inputs[j].Role = "item"
		}
	}
	data, _ := json.Marshal(tr)
	c, ds := DecodeCandidate(data)
	if c.Graph != nil || len(ds) != 1 || ds[0].Code != "HFIR_TRANSPORT_TOO_LARGE" {
		t.Fatalf("expansion: %#v", ds)
	}
}

func TestAdapterRunDeadlineAndSleep(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		deadline     time.Duration
		fail         bool
	}{
		{"huge_sleep", `(cli_app (sleep 1000000000000000))`, 0, true},
		{"busy_loop", `(cli_app (while true (do)))`, 5 * time.Millisecond, true},
		{"caught_sleep", `(cli_app (try_let (x (sleep 1000000000000000)) (catch e (print "caught")) (print x)))`, 0, true},
		{"normal_sleep", `(cli_app (sleep 2) (print 7))`, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := parseAndLower(t, tc.source, "deadline.howl", false)
			data, err := EncodeCandidateTransport(g)
			if err != nil {
				t.Fatal(err)
			}
			c, ds := DecodeCandidate(data)
			if len(ds) > 0 {
				t.Fatal(ds)
			}
			p, ds := CompileCandidate(c)
			if len(ds) > 0 {
				t.Fatal(ds)
			}
			policy := vm.DefaultExecutionPolicy()
			policy.Deadline = tc.deadline
			if tc.name == "busy_loop" {
				policy.Limits.MaxInstructions = 1000000000
			}
			var out, stderr bytes.Buffer
			start := time.Now()
			ev, receipt := vm.RunBytecodeWithReceipt(p, nil, policy, nil, strings.NewReader(""), &out, &stderr, "test", "test")
			if tc.deadline == 0 && receipt.Limits.DeadlineMS != 1000 {
				t.Fatalf("receipt omits effective deadline: %#v", receipt.Limits)
			}
			_ = ev
			raw, _ := json.Marshal(receipt)
			if tc.fail {
				if receipt.Exit.Error == nil || receipt.Exit.Error.Code != "HFIR_DEADLINE_EXCEEDED" {
					t.Fatalf("receipt: %s", raw)
				}
			} else if out.String() != "7\n" {
				t.Fatalf("normal sleep: %q, receipt %s", out.String(), raw)
			}
			if time.Since(start) > time.Second {
				t.Fatal("deadline did not return promptly")
			}
		})
	}
}

func TestAdapterSharedEffectsKeepOrder(t *testing.T) {
	tr := candidateTransport{SchemaVersion: ModelAdapterSchemaVersion, GraphVersion: "v1", EntryNode: "program", Nodes: []transportNode{
		testNode("program", "program", "", "", []transportEdge{{Role: "body", NodeID: "seq"}, {Role: "body", NodeID: "seq"}}),
		testNode("seq", "sequence", "", "", []transportEdge{{Role: "body", NodeID: "p1"}, {Role: "body", NodeID: "p2"}}),
		testNode("p1", "print", "", "", []transportEdge{{Role: "value", NodeID: "one"}}),
		testNode("p2", "print", "", "", []transportEdge{{Role: "value", NodeID: "two"}}),
		testNode("one", "const", "1", "INT", nil), testNode("two", "const", "2", "INT", nil),
	}}
	c := mustCandidate(t, tr)
	p, ds := CompileCandidate(c)
	if len(ds) > 0 {
		t.Fatal(ds)
	}
	var out, stderr bytes.Buffer
	ev := vm.RunBytecodeWithEvidence(p, nil, vm.DefaultExecutionPolicy(), nil, nil, &out, &stderr, 0)
	if ev.RuntimeFailure != nil || out.String() != "1\n2\n1\n2\n" {
		t.Fatalf("effect order: %#v %q", ev, out.String())
	}
}

func TestAdapterDirectCandidateBounds(t *testing.T) {
	tr := blockerDAG(2048, false)
	g := NewGraph()
	g.EntryNode = tr.EntryNode
	for _, n := range tr.Nodes {
		g.AddNode(nodeFromTransport(n))
	}
	p, ds := CompileCandidate(Candidate{Graph: g})
	if p != nil || len(ds) != 1 || ds[0].Code != "HFIR_TRANSPORT_TOO_DEEP" {
		t.Fatalf("direct candidate bypassed transport bounds: %#v", ds)
	}
}

func TestAdapterAggregateFragmentCap(t *testing.T) {
	tr := blockerDAG(1700, false)
	second := blockerDAG(1700, false)
	for _, n := range second.Nodes {
		if n.ID == "n0" || n.ID == "program" {
			continue
		}
		n.ID += "b"
		for i := range n.Inputs {
			if n.Inputs[i].NodeID != "n0" {
				n.Inputs[i].NodeID += "b"
			}
		}
		tr.Nodes = append(tr.Nodes, n)
	}
	for i := range tr.Nodes {
		if tr.Nodes[i].ID == "program" {
			tr.Nodes[i].Inputs = append(tr.Nodes[i].Inputs, transportEdge{Role: "body", NodeID: "printb"})
		}
	}
	data, _ := json.Marshal(tr)
	c, ds := DecodeCandidate(data)
	if c.Graph != nil || len(ds) != 1 || ds[0].Code != "HFIR_TRANSPORT_TOO_LARGE" {
		t.Fatalf("aggregate fragments: %#v", ds)
	}
}
