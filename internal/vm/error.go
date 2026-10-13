package vm

import (
	"encoding/json"
	"fmt"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"time"
)

type VMError struct {
	Phase       string `json:"phase"`
	Code        string `json:"code"`
	Function    string `json:"function"`
	Instruction int    `json:"instruction"`
	Opcode      string `json:"opcode,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
	Message     string `json:"message"`

	// ExecExitCode and ExecOutput describe a child that ran and exited
	// non-zero. They are never part of Error() (receipts, evidence and server
	// logs stay unchanged); only a try_let/catch binding sees them, through
	// CatchString, so a program can recover the exit status and the output.
	ExecExitCode int    `json:"-"`
	ExecOutput   []byte `json:"-"`
}

// execArgv turns the evaluated trailing arguments of exec into the argument
// vector. Each value is printed as text. As a special case, exactly one
// trailing argument that is a list is the whole argument vector, so a program
// can run a command whose argument count is only known at run time. The
// process:exec allow-list is checked against the final vector.
func execArgv(raw []any) []string {
	if len(raw) == 1 {
		if list, ok := raw[0].([]any); ok {
			raw = list
		}
	}
	args := make([]string, len(raw))
	for i, v := range raw {
		args[i] = fmt.Sprint(v)
	}
	return args
}

type vmErrorWithExec struct {
	VMError
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

// CatchString is the error value a try_let catch binding receives. It equals
// Error() unless the error is a non-zero child exit, which adds "exit_code"
// and "output" (the child's combined stdout and stderr).
func (e *VMError) CatchString() string {
	if e.ExecExitCode == 0 {
		return e.Error()
	}
	b, _ := json.Marshal(vmErrorWithExec{VMError: *e, ExitCode: e.ExecExitCode, Output: string(e.ExecOutput)})
	return string(b)
}

func (e *VMError) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func NewRuntimeError(code string, fn string, ip int, op bytecode.Opcode, msg string, args ...any) *VMError {
	opName := ""
	if spec, ok := bytecode.Registry[op]; ok {
		opName = spec.Name
	}
	return &VMError{
		Phase:       "runtime",
		Code:        code,
		Function:    fn,
		Instruction: ip,
		Opcode:      opName,
		Message:     fmt.Sprintf(msg, args...),
	}
}

type VMLimits struct {
	MaxInstructions   int
	MaxMemoryBytes    int
	MaxCallDepth      int
	MaxFetchBodyBytes int
	// MaxRequestBodyBytes bounds a route's req.body read (parse_json); <=0 means the default.
	MaxRequestBodyBytes int
	MaxExecOutputBytes  int
}

// ExecutionPolicy is authority supplied by the trusted bytecode runner. It
// may constrain an application, but application code cannot inspect or widen
// it. The zero value is fail-closed and permits no instruction execution.
type ExecutionPolicy struct {
	Limits VMLimits
	// Deadline is an optional wall-clock duration; zero means no deadline.
	Deadline time.Duration
	// AdapterDeadline is the runner-owned wall-clock budget for programs
	// produced by hfir.CompileCandidate (#175). Zero means
	// DefaultAdapterDeadline; values above MaxAdapterDeadline are clamped to
	// it. Model-authored candidates cannot set or widen it. A shorter
	// Deadline still applies.
	AdapterDeadline time.Duration
}

// DefaultAdapterDeadline is the model-adapter budget when the runner sets none.
const DefaultAdapterDeadline = time.Second

// MaxAdapterDeadline is the hard cap on a runner-owned adapter budget (#175).
const MaxAdapterDeadline = 30 * time.Second

// ParseAdapterDeadline validates a runner flag value such as "5s". It refuses
// zero, negative and over-cap durations instead of clamping them, so a runner
// typo cannot silently change the budget.
func ParseAdapterDeadline(raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("-adapter-deadline invalid duration: %w", err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("-adapter-deadline must be greater than zero")
	}
	if d > MaxAdapterDeadline {
		return 0, fmt.Errorf("-adapter-deadline %s exceeds the %s hard cap", d, MaxAdapterDeadline)
	}
	return d, nil
}

// AdapterDeadlineFlag is a flag.Value for a runner's -adapter-deadline flag.
// It writes the validated budget into the runner's ExecutionPolicy.
type AdapterDeadlineFlag struct{ Policy *ExecutionPolicy }

func (f AdapterDeadlineFlag) String() string {
	if f.Policy == nil || f.Policy.AdapterDeadline == 0 {
		return DefaultAdapterDeadline.String()
	}
	return f.Policy.AdapterDeadline.String()
}

func (f AdapterDeadlineFlag) Set(raw string) error {
	d, err := ParseAdapterDeadline(raw)
	if err != nil {
		return err
	}
	f.Policy.AdapterDeadline = d
	return nil
}

var DefaultLimits = VMLimits{
	MaxInstructions:     100000,
	MaxMemoryBytes:      67108864,
	MaxCallDepth:        1000,
	MaxFetchBodyBytes:   10 * 1024 * 1024,
	MaxRequestBodyBytes: 10 * 1024 * 1024,
	MaxExecOutputBytes:  10 * 1024 * 1024,
}

// DefaultExecutionPolicy returns a copy of the standalone VM's safe defaults.
func DefaultExecutionPolicy() ExecutionPolicy {
	return ExecutionPolicy{Limits: DefaultLimits}
}
