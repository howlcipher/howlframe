package harness

import (
	"context"
	"os"
	"path/filepath"
)

func (r Runner) RunArmB(ctx context.Context, t Task, candidate []byte, out string) (RunResult, error) {
	return r.run(ctx, t, candidate, out, "b")
}
func (r Runner) armBAttempt(ctx context.Context, t Task, candidate []byte, dir string) (RunResult, bool, error) {
	result := newResult(t, "b")
	if e := ctx.Err(); e != nil {
		return result, false, e
	}
	sandbox, canary, e := materialize(r.Root, dir, t, r.attackerURL())
	if e != nil {
		return result, false, e
	}
	if e := os.WriteFile(filepath.Join(dir, "candidate.plan.json"), candidate, 0600); e != nil {
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
	p, e := DecodePlan(candidate)
	if e == nil {
		e = ValidatePlan(p, sandbox, t)
	}
	if e != nil {
		result.Stage = "validation"
		result.Diagnostics = append(result.Diagnostics, e.Error())
		return result, true, nil
	}
	broker, e := ExecutePlan(p, sandbox, t)
	result.Broker = &broker
	result.Stage = "run"
	if e != nil {
		result.Diagnostics = append(result.Diagnostics, e.Error())
	}
	var err error
	result.Comparison, err = r.finish(t, sandbox, canary, before, cb, nil)
	if e != nil {
		result.Comparison.Success = false
	}
	return result, false, err
}
