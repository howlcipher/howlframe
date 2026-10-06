package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (r Runner) RunArmA(ctx context.Context, t Task, candidate []byte, out string) (RunResult, error) {
	return r.run(ctx, t, candidate, out, "a")
}
func (r Runner) armAAttempt(ctx context.Context, t Task, candidate []byte, dir string) (RunResult, bool, error) {
	result := newResult(t, "a")
	sandbox, canary, e := materialize(r.Root, dir, t, r.attackerURL())
	if e != nil {
		return result, false, e
	}
	before, e := SnapshotTree(sandbox)
	if e != nil {
		return result, false, e
	}
	cb, e := SnapshotTree(canary)
	if e != nil {
		return result, false, e
	}
	source := filepath.Join(dir, "candidate.howl")
	artifact := filepath.Join(dir, "prog.hfbc")
	receiptPath := filepath.Join(dir, "receipt.json")
	candidate = []byte(strings.ReplaceAll(string(candidate), "{{ATTACKER_URL}}", r.attackerURL()))
	if e := os.WriteFile(source, candidate, 0600); e != nil {
		return result, false, e
	}
	deadline := r.Deadline
	if deadline <= 0 {
		deadline = 5 * time.Second
	}
	budget := r.MaxInstructions
	if budget <= 0 {
		budget = 100000
	}
	command := func(args ...string) (string, string, error) {
		cmdctx, cancel := context.WithTimeout(ctx, deadline+time.Second)
		defer cancel()
		cmd := exec.CommandContext(cmdctx, r.HowlFrame, args...)
		cmd.Dir = sandbox
		cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + dir, "LANG=C", "TZ=UTC"}
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		e := cmd.Run()
		return stdout.String(), stderr.String(), e
	}
	for _, stage := range []string{"check", "build"} {
		args := []string{stage, "--profile", "governed", source}
		if stage == "build" {
			args = append(args, "-o", artifact)
		}
		stdout, stderr, e := command(args...)
		if e != nil {
			result.Stage = stage
			result.Stdout = stdout
			result.Stderr = stderr
			result.Diagnostics = append(result.Diagnostics, stderr, stdout, e.Error())
			return result, true, nil
		}
	}
	stdout, stderr, e := command("inspect", "--caps", artifact)
	if e != nil {
		result.Stage = "caps_preflight"
		result.Diagnostics = append(result.Diagnostics, stderr, e.Error())
		return result, true, nil
	}
	var report struct {
		Capabilities []string `json:"capabilities"`
	}
	if e := json.Unmarshal([]byte(stdout), &report); e != nil {
		return result, false, e
	}
	result.RequiredCaps = append([]string{}, report.Capabilities...)
	if err := os.WriteFile(filepath.Join(dir, "caps.json"), []byte(stdout), 0600); err != nil {
		return result, false, err
	}
	for _, cap := range report.Capabilities {
		if !has(t.AllowedCaps, cap) {
			result.Stage = "caps_preflight"
			result.Diagnostics = append(result.Diagnostics, `{"code":"CAPS_PREFLIGHT_DENIED","capability":"`+cap+`"}`)
			return result, true, nil
		}
	}
	result.Stdout, result.Stderr, e = command("-run-bc", "--allow-caps", strings.Join(t.AllowedCaps, ","), "--max-instructions", strconv.Itoa(budget), "--deadline", deadline.String(), "--receipt", receiptPath, artifact)
	result.Stage = "run"
	if e != nil {
		result.Diagnostics = append(result.Diagnostics, result.Stderr, e.Error())
	}
	receipt, err := ReadReceipt(receiptPath)
	if err != nil {
		result.Comparison, err = r.finish(t, sandbox, canary, before, cb, nil)
		result.Comparison.Success = false
		result.Comparison.Status = "receipt_missing"
		if err != nil {
			return result, false, err
		}
		result.Diagnostics = append(result.Diagnostics, "missing/invalid receipt")
		return result, false, nil
	}
	result.Comparison, err = r.finish(t, sandbox, canary, before, cb, &receipt)
	if e != nil {
		result.Comparison.Success = false
	}
	return result, false, err
}
