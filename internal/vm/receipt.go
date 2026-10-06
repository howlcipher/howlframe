package vm

import (
	"encoding/json"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
)

// ExecutionReceipt is host-owned reporting, independent of sealed ExecutionEvidence.
// ArtifactSHA256 and CompilerVersion are supplied by the trusted artifact runner.
type ExecutionReceipt struct {
	Schema           string          `json:"schema"`
	ArtifactSHA256   string          `json:"artifact_sha256"`
	CompilerVersion  string          `json:"compiler_version"`
	Grant            []string        `json:"grant"`
	Limits           ReceiptLimits   `json:"limits"`
	InstructionsUsed int             `json:"instructions_used"`
	Effects          []ReceiptEffect `json:"effects"`
	EffectsTruncated bool            `json:"effects_truncated"`
	Exit             ReceiptExit     `json:"exit"`
}
type ReceiptLimits struct {
	MaxInstructions    int   `json:"max_instructions"`
	MaxCallDepth       int   `json:"max_call_depth"`
	MaxMemoryBytes     int   `json:"max_memory_bytes"`
	MaxFetchBodyBytes  int   `json:"max_fetch_body_bytes"`
	MaxExecOutputBytes int   `json:"max_exec_output_bytes"`
	DeadlineMS         int64 `json:"deadline_ms"`
}
type ReceiptEffect struct {
	Op            string `json:"op"`
	Capability    string `json:"capability"`
	TargetSummary string `json:"target_summary"`
	Decision      string `json:"decision"`
}
type ReceiptExit struct {
	Status string        `json:"status"`
	Code   int           `json:"code"`
	Error  *ReceiptError `json:"error"`
}
type ReceiptError struct {
	Code        string `json:"code"`
	Opcode      string `json:"opcode"`
	Instruction int    `json:"instruction"`
	Message     string `json:"message"`
}

func (r ExecutionReceipt) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

const maxReceiptEffects = 10000

type receiptRecorder struct {
	mu           sync.Mutex
	effects      []ReceiptEffect
	truncated    bool
	finalized    bool
	instructions int
}
type effectDecision struct {
	summary string
	seen    []capability.Capability
}

func (r *receiptRecorder) finish(instructions int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.instructions = instructions
	r.finalized = true
}
func (vm *BCVM) recordEffect(cap capability.Capability, op bytecode.Opcode, decision string) {
	if vm.receipt == nil {
		return
	}
	d := vm.receiptDecision
	if d == nil {
		return
	}
	for _, seen := range d.seen {
		if seen == cap {
			return
		}
	}
	d.seen = append(d.seen, cap)
	r := vm.receipt
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finalized {
		return
	}
	if len(r.effects) >= maxReceiptEffects {
		r.truncated = true
		return
	}
	r.effects = append(r.effects, ReceiptEffect{bytecode.Registry[op].Name, string(cap), d.summary, decision})
}

// RunBytecodeWithReceipt returns normally on every VM outcome. Diagnostics match
// RunBytecodeWithPolicy; the caller must persist the receipt before process exit.
func RunBytecodeWithReceipt(prog *bytecode.BCProgram, args []string, policy ExecutionPolicy, caps []capability.Capability, in io.Reader, out, errOut io.Writer, artifactSHA256, compilerVersion string) (int, ExecutionReceipt) {
	recorder := &receiptRecorder{effects: []ReceiptEffect{}}
	ev := runBytecodeEvidence(prog, args, policy, caps, in, out, errOut, 0, recorder)
	grant := []string{}
	for _, cap := range caps {
		grant = append(grant, string(cap))
	}
	sort.Strings(grant)
	grant = compactGrant(grant)
	l := policy.Limits
	var deadlineMS int64
	if policy.Deadline > 0 {
		// Round up so zero always denotes an absent deadline, without overflow.
		deadlineMS = int64((policy.Deadline-1)/time.Millisecond) + 1
	}
	r := ExecutionReceipt{Schema: "howlframe.receipt/v0", ArtifactSHA256: artifactSHA256, CompilerVersion: compilerVersion, Grant: grant,
		Limits:           ReceiptLimits{l.MaxInstructions, l.MaxCallDepth, l.MaxMemoryBytes, l.MaxFetchBodyBytes, l.MaxExecOutputBytes, deadlineMS},
		InstructionsUsed: recorder.instructions, Effects: recorder.effects, EffectsTruncated: recorder.truncated, Exit: ReceiptExit{Status: "ok", Code: ev.ExitCode}}
	if ev.RuntimeFailure != nil {
		writeRuntimeFailure(errOut, ev.RuntimeFailure)
		f := ev.RuntimeFailure
		// Host errors can embed URLs, SQL, arguments, or prompts. Keep those only in
		// the existing stderr diagnostic, never in a receipt.
		message := "runtime failure: " + f.Code
		if f.Code == "LIMIT_EXCEEDED" || f.Code == "CAPABILITY_DENIED" {
			message = f.Message
		}
		r.Exit = ReceiptExit{Status: "error", Code: 1, Error: &ReceiptError{f.Code, f.Opcode, f.Instruction, message}}
	} else if ev.ExitCode != 0 {
		r.Exit.Status = "error"
	}
	return r.Exit.Code, r
}
func compactGrant(grant []string) []string {
	n := 0
	for _, cap := range grant {
		if n == 0 || grant[n-1] != cap {
			grant[n] = cap
			n++
		}
	}
	return grant[:n]
}

func (vm *BCVM) effectSummary(inst bytecode.BCInstruction, env *BcEnv) string {
	peek := func(depth int) string {
		i := len(vm.stack) - 1 - depth
		if i < 0 || i >= len(vm.stack) {
			return ""
		}
		s, _ := vm.stack[i].(string)
		return s
	}
	summary := ""
	switch inst.Op {
	case bytecode.OpFetch:
		u, err := url.Parse(peek(1))
		if err != nil || u.Scheme == "" || u.Host == "" {
			summary = "<invalid-url>"
		} else {
			summary = u.Scheme + "://" + u.Host
		}
	case bytecode.OpEnv:
		summary = peek(0)
	case bytecode.OpReadFile, bytecode.OpMkdir:
		summary = peek(0)
	case bytecode.OpWriteFile:
		summary = peek(1)
	case bytecode.OpExec:
		if inst.IntOperand >= 0 && inst.IntOperand < int64(len(vm.stack)) {
			summary = peek(int(inst.IntOperand))
			if summary != "" {
				summary = filepath.Base(summary)
			}
		}
	case bytecode.OpDbConnect:
		summary = inst.StringOperand2
	case bytecode.OpSqlQuery, bytecode.OpStorePut, bytecode.OpStoreGet, bytecode.OpStoreDelete, bytecode.OpStoreKeys:
		summary = inst.StringOperand
	case bytecode.OpStoreOpen:
		summary = inst.StringOperand2
	case bytecode.OpHttpServerStart, bytecode.OpHttpRoute:
		summary = inst.StringOperand
	case bytecode.OpHttpServerServe:
		v, _ := env.get("__http_port")
		summary, _ = v.(string)
	}
	if len(summary) > 256 {
		summary = summary[:256]
		for !utf8.ValidString(summary) {
			summary = summary[:len(summary)-1]
		}
	}
	return summary
}
