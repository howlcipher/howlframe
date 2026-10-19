package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// StubModel is the dry-run model. It never contacts a provider. It returns the
// handwritten reference for tasks that have one, an invalid candidate followed
// by the reference on repair for RepairIDs, an ungranted fetch for ViolateIDs,
// and an empty answer otherwise.
type StubModel struct {
	Root       string
	RepairIDs  map[string]bool
	ViolateIDs map[string]bool
}

func (StubModel) Name() string { return "stub/dry-run" }
func (s StubModel) Generate(ctx context.Context, p Prompt) (Completion, error) {
	ext, dir := ".howl", "arm_a"
	if p.Arm == "b" {
		ext, dir = ".plan.json", "arm_b"
	}
	c := Completion{ModelIDs: []string{"stub"}, InputTokens: len(strings.Fields(p.Text)), OutputTokens: 0}
	text := ""
	switch {
	case s.ViolateIDs[p.TaskID]:
		b, e := os.ReadFile(filepath.Join(s.Root, "fixtures", "fetch"+ext))
		if e != nil {
			return c, e
		}
		text = string(b)
	case s.RepairIDs[p.TaskID] && p.Round == 0:
		text = "not a valid candidate"
	default:
		b, e := os.ReadFile(filepath.Join(s.Root, "reference", dir, p.TaskID+ext))
		if e == nil {
			text = string(b)
		} else {
			text = "no reference"
		}
	}
	c.Text = "```\n" + text + "```\n"
	c.OutputTokens = len(strings.Fields(text))
	return c, nil
}

// StubModelIDs is the allowed id set for dry runs.
var StubModelIDs = map[string]bool{"stub": true}

// ClaudeModel runs `claude -p --output-format json` with the prompt on stdin in
// an empty directory with tools disabled. Every modelUsage key is recorded and
// checked against Allowed; a mismatch aborts the run. Not exercised in CI.
// codex is not in v1: its CLI does not report the serving model id.
type ClaudeModel struct {
	Work    string
	Allowed map[string]bool
	Bin     string // CLI path; "" means "claude" on PATH (tests use a fake)
}

func (ClaudeModel) Name() string { return "claude-code/" + LiveClaudeModel }
func (m ClaudeModel) Argv() []string {
	bin := m.Bin
	if bin == "" {
		bin = "claude"
	}
	return []string{bin, "-p", "--model", LiveClaudeModel, "--output-format", "json", "--tools", ""}
}
func (m ClaudeModel) Generate(ctx context.Context, p Prompt) (Completion, error) {
	var c Completion
	dir, e := os.MkdirTemp(m.Work, "gen-")
	if e != nil {
		return c, ErrInfra{e}
	}
	argv := m.Argv()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(p.Text)
	var out, er bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &er
	if e := cmd.Run(); e != nil {
		// A failed client is infrastructure, never a model_error cell.
		return c, ErrInfra{fmt.Errorf("claude: %v: %s", e, er.String())}
	}
	return ParseClaudeJSON(out.Bytes(), m.Allowed)
}

// ParseClaudeJSON extracts text, all token classes and every model id.
func ParseClaudeJSON(b []byte, allowed map[string]bool) (Completion, error) {
	var c Completion
	var r struct {
		Result string `json:"result"`
		Usage  struct {
			In          int `json:"input_tokens"`
			Out         int `json:"output_tokens"`
			CacheRead   int `json:"cache_read_input_tokens"`
			CacheCreate int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
		ModelUsage map[string]json.RawMessage `json:"modelUsage"`
	}
	if e := json.Unmarshal(b, &r); e != nil {
		return c, ErrInfra{fmt.Errorf("claude output unparseable: %v", e)}
	}
	c.Text, c.InputTokens, c.OutputTokens = r.Result, r.Usage.In, r.Usage.Out
	c.CacheReadTokens, c.CacheCreateTokens = r.Usage.CacheRead, r.Usage.CacheCreate
	c.ModelIDs = []string{}
	for k := range r.ModelUsage {
		c.ModelIDs = append(c.ModelIDs, k)
	}
	sort.Strings(c.ModelIDs)
	return c, CheckModelIDs(c.ModelIDs, allowed)
}

// TiktokenCounter counts cl100k_base tokens with an external tiktoken
// install (the same counter as the token-efficiency benchmarks).
type TiktokenCounter struct{ Python string }

const tiktokenScript = `import sys,json,tiktoken
e=tiktoken.get_encoding("cl100k_base")
if sys.argv[1:]==["version"]:
    print("tiktoken "+tiktoken.__version__+" cl100k_base")
else:
    print(json.dumps([len(e.encode(t)) for t in json.load(sys.stdin)]))`

func (t TiktokenCounter) Version() (string, error) {
	b, e := exec.Command(t.Python, "-c", tiktokenScript, "version").Output()
	return strings.TrimSpace(string(b)), e
}
func (t TiktokenCounter) Count(texts []string) ([]int, error) {
	in, _ := json.Marshal(texts)
	c := exec.Command(t.Python, "-c", tiktokenScript)
	c.Stdin = bytes.NewReader(in)
	b, e := c.Output()
	if e != nil {
		return nil, e
	}
	var out []int
	if e := json.Unmarshal(b, &out); e != nil || len(out) != len(texts) {
		return nil, fmt.Errorf("tiktoken count failed")
	}
	return out, nil
}
