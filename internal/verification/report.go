package verification

import (
	"regexp"
	"slices"
	"strings"
)

// ReportSchema identifies the structured P1.3 verification report format.
const ReportSchema = "howlframe.verification_report/v0"

const (
	StatusPass    = "pass"
	StatusFail    = "fail"
	StatusSkipped = "skipped"
	StatusPartial = "partial"
)

var codedPrefixRe = regexp.MustCompile(`^([A-Z][A-Z0-9_]+):`)

// StandardPipelineChecks names all static verification stages emitted by check reports.
var StandardPipelineChecks = []string{
	"parser",
	"construct_registry",
	"profile_policy",
	"checker",
	"hfir_gate",
	"artifact_validation",
}

// Versions records compiler and verifier component versions used to produce a report.
type Versions struct {
	Compiler          string `json:"compiler,omitempty"`
	HFBCFormat        string `json:"hfbc_format"`
	ConstructRegistry int    `json:"construct_registry_entries"`
	HFIRVerifier      string `json:"hfir_verifier"`
}

// Check records one named verification stage rather than collapsing all passes
// into a single opaque boolean.
type Check struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Version     string   `json:"version,omitempty"`
	Codes       []string `json:"codes,omitempty"`
	Summary     string   `json:"summary"`
	Diagnostics []string `json:"diagnostics,omitempty"`
}

// Report is the structured machine-readable verification evidence emitted by
// `howlframe check --json`, `howlframe inspect --report`, and `agent check --json`.
type Report struct {
	Schema            string   `json:"schema"`
	Passed            bool     `json:"passed"`
	Partial           bool     `json:"partial"`
	Target            string   `json:"target,omitempty"`
	Profile           string   `json:"profile,omitempty"`
	ArtifactSHA256    string   `json:"artifact_sha256,omitempty"`
	Versions          Versions `json:"versions"`
	HFIRBlockingCodes []string `json:"hfir_blocking_codes"`
	Checks            []Check  `json:"checks"`
	Limitations       []string `json:"limitations"`
}

// DefaultLimitations documents explicit non-claims for static verification reports.
func DefaultLimitations() []string {
	return []string{
		"Static verification report only; does not execute the program, prove runtime correctness, or grant execution authority.",
		"HFIR target feasibility rules reject infeasible constructs only for the wasm target; other targets validate graph structure, construct support, and effect inference.",
	}
}

// Finalize canonicalizes fields deterministically and derives Passed and Partial
// strictly from named Check outcomes without mutating timestamps or host paths.
func (r *Report) Finalize() {
	if r == nil {
		return
	}
	r.Schema = ReportSchema
	r.HFIRBlockingCodes = dedupeSortedStrings(r.HFIRBlockingCodes)
	r.Limitations = dedupeNonEmptyStrings(append(DefaultLimitations(), r.Limitations...))

	hasPass := false
	hasFail := false
	hasPartialOrSkip := false
	for i := range r.Checks {
		r.Checks[i].Codes = dedupeSortedStrings(r.Checks[i].Codes)
		r.Checks[i].Diagnostics = dedupeNonEmptyStrings(r.Checks[i].Diagnostics)
		switch r.Checks[i].Status {
		case StatusPass:
			hasPass = true
		case StatusFail:
			hasFail = true
		case StatusSkipped:
			hasPartialOrSkip = true
		case StatusPartial:
			hasPartialOrSkip = true
			hasPass = true
		default:
			hasFail = true
		}
	}
	if r.Checks == nil {
		r.Checks = []Check{}
	}
	r.Partial = len(r.Checks) == 0 || hasPartialOrSkip
	r.Passed = hasPass && !hasFail
}

func dedupeSortedStrings(in []string) []string {
	out := dedupeNonEmptyStrings(in)
	slices.Sort(out)
	return out
}

func dedupeNonEmptyStrings(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

// ExtractErrorCode extracts a leading ALL_CAPS diagnostic prefix such as
// ARITY_ERROR or TYPE_ERROR when present.
func ExtractErrorCode(msg string) string {
	m := codedPrefixRe.FindStringSubmatch(strings.TrimSpace(msg))
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// AppendRemainingSkipped appends skipped check entries for any stage in pipeline
// that has not yet been recorded in checks.
func AppendRemainingSkipped(checks []Check, pipeline []string, summary string) []Check {
	recorded := make(map[string]bool, len(checks))
	for _, c := range checks {
		recorded[c.Name] = true
	}
	for _, name := range pipeline {
		if recorded[name] {
			continue
		}
		checks = append(checks, Check{
			Name:    name,
			Status:  StatusSkipped,
			Summary: summary,
		})
	}
	return checks
}
