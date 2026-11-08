// Package agent exposes read-only, target-scoped compiler discovery.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/construct"
)

const Schema = "howlframe.agent/v1"
const ExampleSource = "(cli_app (print (+ 20 22)))\n"

type Diagnostic struct {
	Code           string `json:"code"`
	Message        string `json:"message"`
	Filename       string `json:"filename"`
	Line           int    `json:"line"`
	Column         int    `json:"column"`
	Classification string `json:"classification"`
}
type Arity struct {
	Min int `json:"min"`
	Max int `json:"max"`
}
type Entry struct {
	Name              string `json:"name"`
	Support           string `json:"support"`
	Note              string `json:"note"`
	Arity             *Arity `json:"arity"`
	ArityExemption    string `json:"arity_exemption,omitempty"`
	SignatureCoverage string `json:"signature_coverage"`
	Requirement       string `json:"requirement"`
}
type Example struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Stdout string `json:"stdout"`
}
type Response struct {
	Schema        string           `json:"schema"`
	Command       string           `json:"command"`
	Target        string           `json:"target"`
	Coverage      string           `json:"coverage"`
	OK            bool             `json:"ok"`
	ActualGrants  []string         `json:"actual_grants"`
	Limitations   []string         `json:"limitations"`
	Constructs    []Entry          `json:"constructs"`
	Examples      []Example        `json:"examples"`
	Diagnostics   []Diagnostic     `json:"diagnostics"`
	CompilerNotes []string         `json:"compiler_notes"`
	Requirements  *bytecode.Report `json:"requirements,omitempty"`
	Explanation   string           `json:"explanation,omitempty"`
}

func entries() []Entry {
	out := []Entry{}
	exemptions := construct.ArityExemptions()
	for _, e := range construct.Table() {
		item := Entry{Name: e.Name, Support: e.Support.String(), Note: e.Note, SignatureCoverage: "argument-count-only; parameter and return types unknown", Requirement: "no requirement recorded by construct metadata"}
		if a, ok := construct.LookupArity(e.Name); ok {
			item.Arity = &Arity{a.Min, a.Max}
		} else {
			item.ArityExemption = exemptions[e.Name]
		}
		if c := capability.ForConstruct(e.Name); c != capability.None {
			item.Requirement = string(c)
		}
		switch e.Name {
		case "fetch", "http_request":
			item.Requirement = "network:fetch=host[:port]; URL-dependent scoped grant; coarse network is insufficient"
		case "exec":
			item.Requirement = "process:exec=<absolute program path>[ <pattern>...]; exact canonical executable and argument pattern required; coarse process is insufficient"
		case "store_open":
			item.Requirement = "database; filesystem additionally required for file:// URI"
		}
		out = append(out, item)
	}
	return out
}

var explanations = map[string]string{
	"AGENT_ARGUMENT":          "Malformed agent command arguments; use the documented command grammar.",
	"AGENT_UNKNOWN_TARGET":    "The target name is unrecognized; no source was read.",
	"AGENT_COVERAGE_UNKNOWN":  "This interface has no authoritative catalog/check coverage for this recognized backend.",
	"AGENT_UNKNOWN_CONSTRUCT": "The name is absent from the standalone bytecode construct registry; this is not a claim about other backends.",
	"AGENT_UNKNOWN_CODE":      "No explanation is registered for this diagnostic code.",
	"AGENT_SOURCE_IO":         "The requested source could not be read by the compiler.",
	"AGENT_CHECK_FAILED":      "Legacy compiler output could not be normalized to a coded diagnostic; message retains compiler evidence.",
	"AGENT_CHECK_INTERNAL":    "The compiler subprocess or its temporary artifact could not be inspected.",
	"AGENT_CHECK_TIMEOUT":     "The read-only compilation exceeded the interface's 30-second wall-clock limit.",
	"ARITY_ERROR":             "A construct received an argument count outside the authoritative arity range; describe the construct for its range.",
	"HFIR_TARGET_INFEASIBLE":  "The production HFIR gate found behavior the selected target cannot implement.",
}

// Run returns a process exit status and writes exactly one versioned JSON document.
// executable must be the current compiler binary, never a source-selected executable.
func Run(args []string, executable string, out io.Writer) int {
	r := Response{Schema: Schema, CompilerNotes: []string{}, ActualGrants: []string{}, Limitations: []string{"bytecode registry only; other backend coverage unknown", "arity is not a full typed signature", "inspection grants no execution authority", "check compiles but never runs source; includes and modules may be read", "diagnostic explanation coverage is partial", "construct authority metadata is partial; absent metadata is not proof of purity"}, Constructs: []Entry{}, Examples: []Example{}, Diagnostics: []Diagnostic{}, Target: "bytecode", Coverage: "known"}
	fail := func(code, msg string) int {
		r.Diagnostics = append(r.Diagnostics, Diagnostic{Code: code, Message: msg, Classification: "interface"})
		json.NewEncoder(out).Encode(r)
		return 1
	}
	if len(args) == 0 {
		return fail("AGENT_ARGUMENT", "expected capabilities, describe, check, or explain")
	}
	r.Command = args[0]
	positional := []string{}
	jsonFlag := false
	ended := false
	targetSeen := false
	for i := 1; i < len(args); i++ {
		a := args[i]
		if !ended && a == "--" {
			ended = true
			continue
		}
		if !ended && a == "--json" {
			if jsonFlag {
				return fail("AGENT_ARGUMENT", "duplicate --json")
			}
			jsonFlag = true
			continue
		}
		if !ended && (a == "--target" || strings.HasPrefix(a, "--target=")) {
			if targetSeen {
				return fail("AGENT_ARGUMENT", "duplicate --target")
			}
			targetSeen = true
			if a == "--target" {
				i++
				if i >= len(args) {
					return fail("AGENT_ARGUMENT", "missing target")
				}
				r.Target = args[i]
			} else {
				r.Target = strings.TrimPrefix(a, "--target=")
			}
			continue
		}
		if !ended && strings.HasPrefix(a, "-") {
			return fail("AGENT_ARGUMENT", "unknown option: "+a)
		}
		positional = append(positional, a)
	}
	switch r.Target {
	case "bytecode", "bc":
		r.Target = "bytecode"
	case "go", "js", "javascript", "wasm", "wasmgc", "interpreter":
		r.Coverage = "unknown"
	default:
		r.Coverage = "unknown"
		return fail("AGENT_UNKNOWN_TARGET", "unrecognized target: "+r.Target)
	}
	switch r.Command {
	case "capabilities":
		if len(positional) != 0 || jsonFlag {
			return fail("AGENT_ARGUMENT", "usage: agent capabilities [--target TARGET]")
		}
	case "describe":
		if len(positional) != 1 || jsonFlag {
			return fail("AGENT_ARGUMENT", "usage: agent describe NAME [--target TARGET]")
		}
	case "check":
		if len(positional) != 1 || !jsonFlag {
			return fail("AGENT_ARGUMENT", "usage: agent check --json [--target TARGET] [--] SOURCE")
		}
	case "explain":
		if len(positional) != 1 || jsonFlag || targetSeen {
			return fail("AGENT_ARGUMENT", "usage: agent explain CODE")
		}
	default:
		return fail("AGENT_ARGUMENT", "unknown command: "+r.Command)
	}
	if r.Command == "explain" {
		text, ok := explanations[positional[0]]
		if !ok {
			return fail("AGENT_UNKNOWN_CODE", "unknown diagnostic code: "+positional[0])
		}
		r.Explanation = text
	} else if r.Coverage == "unknown" {
		if r.Command != "capabilities" {
			return fail("AGENT_COVERAGE_UNKNOWN", "coverage unknown for target: "+r.Target)
		}
	} else {
		switch r.Command {
		case "capabilities":
			r.Constructs = entries()
			r.Examples = []Example{{"pure-add", ExampleSource, "bytecode", "42\n"}}
		case "describe":
			for _, e := range entries() {
				if e.Name == positional[0] {
					r.Constructs = append(r.Constructs, e)
				}
			}
			if len(r.Constructs) == 0 {
				return fail("AGENT_UNKNOWN_CONSTRUCT", "unknown construct: "+positional[0])
			}
		case "check":
			ds, requirements, notes := check(executable, positional[0])
			r.CompilerNotes = notes
			r.Diagnostics = ds
			r.Requirements = requirements
			if len(ds) > 0 {
				json.NewEncoder(out).Encode(r)
				return 1
			}
		}
	}
	r.OK = true
	json.NewEncoder(out).Encode(r)
	return 0
}

// boundedBuffer prevents malformed compiler diagnostics from growing memory without bound.
type boundedBuffer struct {
	bytes.Buffer
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := (1 << 20) - b.Len()
	if len(p) > remaining {
		b.truncated = true
		p = p[:remaining]
	}
	b.Buffer.Write(p)
	return n, nil
}

var codedReason = regexp.MustCompile(`^([A-Z][A-Z0-9_]+):`)

func check(executable, source string) ([]Diagnostic, *bytecode.Report, []string) {
	ds := []Diagnostic{}
	failure := func(code, msg string) ([]Diagnostic, *bytecode.Report, []string) {
		return []Diagnostic{{Code: code, Message: msg, Filename: source, Classification: "interface"}}, nil, []string{}
	}
	absolute, err := filepath.Abs(source)
	if err != nil {
		return failure("AGENT_SOURCE_IO", err.Error())
	}
	if _, err = os.Stat(absolute); err != nil {
		return failure("AGENT_SOURCE_IO", err.Error())
	}
	dir, err := os.MkdirTemp("", "howlframe-agent-check-")
	if err != nil {
		return failure("AGENT_CHECK_INTERNAL", err.Error())
	}
	defer os.RemoveAll(dir)
	artifact := filepath.Join(dir, "checked.hfbc")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "build", "--target", "bytecode", "-o", artifact, "--", absolute)
	var stdout, stderr boundedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if ctx.Err() != nil {
		return failure("AGENT_CHECK_TIMEOUT", "compilation exceeded 30 seconds")
	}
	if err != nil {
		if !stdout.truncated && !stderr.truncated && strings.TrimSpace(stderr.String()) == "" {
			ds = normalize(stdout.Bytes(), source)
		}
		if len(ds) == 0 {
			msg := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			msg = strings.ReplaceAll(msg, dir, "<temporary>")
			return failure("AGENT_CHECK_FAILED", msg)
		}
		return ds, nil, []string{}
	}
	if stdout.truncated || stderr.truncated {
		return failure("AGENT_CHECK_FAILED", "compiler output exceeded 1 MiB capture limit")
	}
	notes := []string{}
	for _, stream := range []string{stdout.String(), stderr.String()} {
		for _, line := range strings.Split(strings.TrimSuffix(stream, "\n"), "\n") {
			if line != "" && line != "Built "+artifact {
				notes = append(notes, strings.ReplaceAll(line, dir, "<temporary>"))
			}
		}
	}
	f, err := os.Open(artifact)
	if err != nil {
		return failure("AGENT_CHECK_INTERNAL", err.Error())
	}
	defer f.Close()
	program, err := bytecode.ReadArtifact(f)
	if err != nil {
		return failure("AGENT_CHECK_INTERNAL", err.Error())
	}
	report := bytecode.RequiredCapabilities(program)
	return ds, &report, notes
}
func normalize(data []byte, source string) []Diagnostic {
	var legacy struct {
		Reason string `json:"reason"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
	}
	if json.Unmarshal(data, &legacy) == nil && legacy.Reason != "" {
		code := "AGENT_CHECK_FAILED"
		class := "legacy-uncoded"
		if match := codedReason.FindStringSubmatch(legacy.Reason); match != nil {
			code = match[1]
			class = "compiler"
		}
		return []Diagnostic{{code, legacy.Reason, "", legacy.Line, legacy.Column, class}}
	}
	var raw []struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Location struct {
			Filename string `json:"filename"`
			Line     int    `json:"line"`
			Column   int    `json:"column"`
		} `json:"location"`
	}
	if json.Unmarshal(data, &raw) != nil || len(raw) == 0 {
		return nil
	}
	out := []Diagnostic{}
	for _, d := range raw {
		if d.Code == "" || d.Message == "" {
			return nil
		}
		out = append(out, Diagnostic{d.Code, d.Message, d.Location.Filename, d.Location.Line, d.Location.Column, "compiler"})
	}
	return out
}
