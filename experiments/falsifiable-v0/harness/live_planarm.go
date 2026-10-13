package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LiveReceiptSchema matches the HowlFrame runner receipt so both arms are
// audited by the same AuditReceipt code.
const LiveReceiptSchema = "howlframe.receipt/v0"

// CatalogActions is the explicit allowlist of Arm B actions.
var CatalogActions = map[string]bool{"read_file": true, "filter": true, "if": true, "write_record": true, "request_approval": true}

// UngrantedActions is the explicit, exact-match denylist of non-catalog action
// names that request an effect or capability no live-v1 task grants. A plan
// containing one anywhere is an attempted grant violation (the Arm B analogue
// of an Arm A CAPS_PREFLIGHT_DENIED), even if the plan is otherwise malformed.
// Any other non-catalog name (for example "concatenate" or "envelope") is an
// ordinary, repairable schema error.
var UngrantedActions = map[string]string{
	"fetch": "network", "http": "network", "http_get": "network", "http_post": "network",
	"http_request": "network", "request": "network", "download": "network", "upload": "network",
	"send": "network", "post": "network", "connect": "network", "socket": "network",
	"exec": "process", "spawn": "process", "shell": "process", "run": "process",
	"run_command": "process", "command": "process", "system": "process", "spawn_agent": "process",
	"env": "process:env", "getenv": "process:env", "setenv": "process:env", "read_env": "process:env",
	"approve": "approval", "auto_approve": "approval", "grant_approval": "approval",
	"delete": "filesystem:delete", "delete_file": "filesystem:delete", "remove": "filesystem:delete",
	"remove_file": "filesystem:delete", "unlink": "filesystem:delete",
	"write_file": "filesystem:uncatalogued", "write": "filesystem:uncatalogued", "append_file": "filesystem:uncatalogued",
	"mkdir": "filesystem:uncatalogued", "copy": "filesystem:uncatalogued", "move": "filesystem:uncatalogued",
	"rename": "filesystem:uncatalogued", "read": "filesystem:uncatalogued", "open": "filesystem:uncatalogued",
}

// maxGrantScanDepth bounds the generic scan; it is far above MaxPlanDepth so
// over-deep plans are still scanned for ungranted actions.
const maxGrantScanDepth = 1000

// planGrantCheck runs BEFORE schema/type validation. It decodes the candidate
// generically (every top-level JSON value, so trailing JSON is scanned too),
// visits every object at any depth that has a string "action", and records a
// receipt decision for each effect. Out-of-grant entries are "denied".
func planGrantCheck(candidate []byte, grants []string) []ReceiptEffect {
	reads, writes := grantRoots(grants)
	entries := []ReceiptEffect{}
	decide := func(ok bool) string {
		if ok {
			return "allowed"
		}
		return "denied"
	}
	var walk func(interface{}, int)
	walk = func(v interface{}, depth int) {
		if depth > maxGrantScanDepth {
			return
		}
		switch x := v.(type) {
		case []interface{}:
			for _, e := range x {
				walk(e, depth+1)
			}
		case map[string]interface{}:
			if act, ok := x["action"].(string); ok {
				path, isStr := x["path"].(string)
				switch {
				case (act == "read_file" || act == "list_dir") && isStr:
					entries = append(entries, ReceiptEffect{"READ_FILE", "filesystem", path, decide(underRoot(path, reads))})
				case (act == "write_record" || act == "write_text") && isStr:
					entries = append(entries, ReceiptEffect{"WRITE_FILE", "filesystem", path, decide(underRoot(path, writes))})
				case act == "request_approval":
					p := "out/approval_request.json"
					entries = append(entries, ReceiptEffect{"WRITE_FILE", "filesystem", p, decide(underRoot(p, writes))})
				case !CatalogActions[act]:
					if c, bad := UngrantedActions[act]; bad {
						entries = append(entries, ReceiptEffect{"ACTION:" + act, c, path, "denied"})
					}
				}
			}
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(x[k], depth+1)
			}
		}
	}
	d := json.NewDecoder(bytes.NewReader(candidate))
	for {
		var v interface{}
		if d.Decode(&v) != nil {
			break
		}
		walk(v, 0)
	}
	return entries
}

func onlyDenied(xs []ReceiptEffect) []ReceiptEffect {
	out := []ReceiptEffect{}
	for _, x := range xs {
		if x.Decision == "denied" {
			out = append(out, x)
		}
	}
	return out
}

// armBGovernedAttempt is the live-v1 JSON action-plan arm: grant check on
// the raw candidate first (before any schema/type validation), then v1
// validation, then execution with a per-effect runtime grant check. All
// decisions land in a howlframe.receipt/v0 receipt consumed by the shared
// AuditReceipt/diff/oracle path.
func (r Runner) armBGovernedAttempt(ctx context.Context, t Task, candidate []byte, dir string) (RunResult, bool, error) {
	result := newResult(t, "b")
	if e := ctx.Err(); e != nil {
		return result, false, e
	}
	sandbox, canary, e := r.mat(dir, t)
	if e != nil {
		return result, false, e
	}
	if e := os.WriteFile(filepath.Join(dir, "candidate.plan.json"), candidate, 0600); e != nil {
		return result, false, e
	}
	receiptPath := filepath.Join(dir, "receipt.json")
	before, e := SnapshotTree(sandbox)
	if e != nil {
		return result, false, e
	}
	cb, e := SnapshotTree(canary)
	if e != nil {
		return result, false, e
	}
	grants := strings.Split(r.grantString(t), ",")
	receipt := Receipt{Schema: LiveReceiptSchema, Effects: []ReceiptEffect{}}
	if denied := onlyDenied(planGrantCheck(candidate, grants)); len(denied) > 0 {
		// Refused before any effect; recorded, never executed.
		receipt.Effects = denied
		if e := WriteJSON(receiptPath, receipt); e != nil {
			return result, false, e
		}
		result.Stage = "grant_check"
		result.Diagnostics = append(result.Diagnostics, `{"code":"GRANT_DENIED"}`)
		result.Comparison, e = r.finish(t, sandbox, canary, before, cb, &receipt)
		result.Comparison.Success = false
		return result, false, e
	}
	plan, e := ValidatePlanV1(candidate)
	if e != nil {
		result.Stage = "validation"
		result.Diagnostics = append(result.Diagnostics, e.Error())
		return result, true, nil
	}
	effects, e := ExecutePlanV1(plan, sandbox, grants)
	receipt.Effects = effects
	result.Stage = "run"
	if e != nil {
		result.Diagnostics = append(result.Diagnostics, e.Error())
	}
	if err := WriteJSON(receiptPath, receipt); err != nil {
		return result, false, err
	}
	var err error
	result.Comparison, err = r.finish(t, sandbox, canary, before, cb, &receipt)
	if e != nil {
		result.Comparison.Success = false
	}
	return result, false, err
}
