package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	c := Completion{ModelID: "stub", InputTokens: len(strings.Fields(p.Text)), OutputTokens: 0}
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

// CLIModel runs a model CLI with the prompt on stdin in an empty directory.
// Format "claude-json" parses `claude -p --output-format json`; "codex" reads
// the file passed to `codex exec -o`. Not exercised in CI.
type CLIModel struct {
	Label  string
	Argv   []string
	Format string
	Work   string
}

func (m CLIModel) Name() string { return m.Label }
func (m CLIModel) Generate(ctx context.Context, p Prompt) (Completion, error) {
	var c Completion
	dir, e := os.MkdirTemp(m.Work, "gen-")
	if e != nil {
		return c, e
	}
	argv := append([]string{}, m.Argv...)
	last := filepath.Join(dir, "last.txt")
	for i := range argv {
		argv[i] = strings.ReplaceAll(argv[i], "{{LAST}}", last)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(p.Text)
	var out, er bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &er
	if e := cmd.Run(); e != nil {
		return c, fmt.Errorf("%s: %v: %s", m.Label, e, er.String())
	}
	switch m.Format {
	case "claude-json":
		var r struct {
			Result string `json:"result"`
			Usage  struct {
				In  int `json:"input_tokens"`
				Out int `json:"output_tokens"`
			} `json:"usage"`
			ModelUsage map[string]json.RawMessage `json:"modelUsage"`
		}
		if e := json.Unmarshal(out.Bytes(), &r); e != nil {
			return c, e
		}
		c.Text, c.InputTokens, c.OutputTokens = r.Result, r.Usage.In, r.Usage.Out
		for k := range r.ModelUsage {
			c.ModelID = k
		}
	case "codex":
		b, e := os.ReadFile(last)
		if e != nil {
			return c, e
		}
		c.Text = string(b)
	default:
		c.Text = out.String()
	}
	return c, nil
}
