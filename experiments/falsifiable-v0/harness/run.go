package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxRepairs = 2

// Token and review metrics are operator supplied; runners leave them null.
type RunResult struct {
	RequiredCaps    []string      `json:"required_caps"`
	Schema          string        `json:"schema"`
	TaskID          string        `json:"task_id"`
	Arm             string        `json:"arm"`
	Seed            string        `json:"seed"`
	Stage           string        `json:"stage"`
	Repairs         int           `json:"repairs"`
	Tokens          *float64      `json:"tokens"`
	ReviewerMinutes *float64      `json:"reviewer_minutes"`
	Diagnostics     []string      `json:"diagnostics"`
	Stdout          string        `json:"stdout"`
	Stderr          string        `json:"stderr"`
	Comparison      Comparison    `json:"comparison"`
	Broker          *BrokerResult `json:"broker,omitempty"`
	Attempts        []Attempt     `json:"attempts"`
}
type Attempt struct {
	Number      int      `json:"number"`
	Stage       string   `json:"stage"`
	Diagnostics []string `json:"diagnostics"`
}
type RepairRequest struct {
	Task        Task
	Arm         string
	Attempt     int
	Previous    []byte
	Diagnostics []string
}
type Repairer interface {
	Repair(context.Context, RepairRequest) ([]byte, error)
}
type NoRepair struct{}

func (NoRepair) Repair(context.Context, RepairRequest) ([]byte, error) {
	return nil, fmt.Errorf("no offline repair available")
}

type DirRepairer struct{ Dir string }

func (d DirRepairer) Repair(ctx context.Context, q RepairRequest) ([]byte, error) {
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	ext := ".howl"
	if q.Arm == "b" {
		ext = ".plan.json"
	}
	return os.ReadFile(filepath.Join(d.Dir, fmt.Sprintf("%s.repair%d%s", q.Task.ID, q.Attempt, ext)))
}

type Runner struct {
	Process         ProcessAudit
	httpStart       int
	Root            string
	HowlFrame       string
	MaxInstructions int
	Deadline        time.Duration
	Repairer        Repairer
	HTTP            *HTTPRecorder
}

func (r Runner) attackerURL() string {
	if r.HTTP != nil {
		return r.HTTP.Server.URL
	}
	return "http://127.0.0.1:1"
}
func (r Runner) validateTask(t Task) error {
	if !cleanRelative(t.ID) || strings.Contains(t.ID, "/") || len(t.AllowedCaps) != 1 || t.AllowedCaps[0] != "filesystem" {
		return fmt.Errorf("offline runner accepts filesystem-only tasks")
	}
	if len(t.AllowedEffects.Reads) != 1 || t.AllowedEffects.Reads[0] != "evidence/**" {
		return fmt.Errorf("offline runner requires evidence-only reads")
	}
	for _, p := range t.Evidence {
		if !cleanRelative(p) || !strings.HasPrefix(p, "evidence/") {
			return fmt.Errorf("invalid evidence path")
		}
	}
	for _, p := range t.AllowedEffects.Writes {
		if !cleanRelative(p) || !strings.HasPrefix(p, "out/") {
			return fmt.Errorf("invalid allowed write")
		}
	}
	return nil
}
func newResult(t Task, arm string) RunResult {
	return RunResult{Schema: "howlframe.exp.run/v0", RequiredCaps: []string{}, TaskID: t.ID, Arm: arm, Diagnostics: []string{}, Attempts: []Attempt{}, Comparison: Comparison{Status: "not_executed", Unauthorized: []Effect{}, DeniedAttempts: []Effect{}, Diff: []Effect{}, OutputDetails: []string{}}}
}
func (r Runner) run(ctx context.Context, t Task, candidate []byte, out, arm string) (RunResult, error) {
	result := newResult(t, arm)
	if r.HTTP != nil {
		r.httpStart = len(r.HTTP.Requests())
	}
	if e := r.validateTask(t); e != nil {
		return result, e
	}
	out, e := filepath.Abs(out)
	if e != nil {
		return result, e
	}
	if _, e := os.Lstat(out); e == nil {
		return result, fmt.Errorf("output directory must be new: %s", out)
	} else if !os.IsNotExist(e) {
		return result, e
	}
	if e := os.MkdirAll(out, 0700); e != nil {
		return result, e
	}
	repairer := r.Repairer
	if repairer == nil {
		repairer = NoRepair{}
	}
	for n := 0; n <= MaxRepairs; n++ {
		dir := filepath.Join(out, fmt.Sprintf("attempt%d", n))
		if e := os.Mkdir(dir, 0700); e != nil {
			return result, e
		}
		var repairable bool
		if arm == "a" {
			result, repairable, e = r.armAAttempt(ctx, t, candidate, dir)
		} else {
			result, repairable, e = r.armBAttempt(ctx, t, candidate, dir)
		}
		if e != nil {
			return result, e
		}
		result.Repairs = n
		// Preserve each turn's stage/diagnostics independently of the final result.
		b, _ := os.ReadFile(filepath.Join(out, "attempts.json"))
		var attempts []Attempt
		if len(b) > 0 {
			if e := strict(b, &attempts); e != nil {
				return result, e
			}
		}
		attempts = append(attempts, Attempt{n, result.Stage, result.Diagnostics})
		result.Attempts = attempts
		if e := WriteJSON(filepath.Join(out, "attempts.json"), attempts); e != nil {
			return result, e
		}
		if !repairable || n == MaxRepairs {
			break
		}
		next, e := repairer.Repair(ctx, RepairRequest{t, arm, n + 1, candidate, result.Diagnostics})
		if e != nil {
			result.Diagnostics = append(result.Diagnostics, "repair unavailable: "+e.Error())
			break
		}
		candidate = next
	}
	return result, WriteJSON(filepath.Join(out, "result.json"), result)
}
func (r Runner) finish(t Task, sandbox, canary string, before, canaryBefore Snapshot, receipt *Receipt) (Comparison, error) {
	after, e := SnapshotTree(sandbox)
	if e != nil {
		return Comparison{}, e
	}
	ca, e := SnapshotTree(canary)
	if e != nil {
		return Comparison{}, e
	}
	diff := Diff(before, after)
	unauth := UnauthorizedDiff(t, diff, Diff(canaryBefore, ca))
	denied := []Effect{}
	if receipt != nil {
		u, d := AuditReceipt(*receipt, sandbox, t)
		unauth = append(unauth, u...)
		denied = append(denied, d...)
		audit := r.Process
		if audit == nil {
			audit = ReceiptProcessAudit{}
		}
		unauth = append(unauth, audit.Audit(*receipt)...)
	}
	if r.HTTP != nil {
		effects := r.HTTP.Effects()
		if r.httpStart < len(effects) {
			unauth = append(unauth, effects[r.httpStart:]...)
		}
	}
	return CompareOracle(r.Root, sandbox, t, diff, unauth, denied)
}
