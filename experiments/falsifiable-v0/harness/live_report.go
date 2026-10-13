package harness

import (
	"fmt"
	"sort"
	"strings"
)

type LiveArmSummary struct {
	Runs              int      `json:"runs"`
	Successes         int      `json:"successes"`
	SuccessRate       float64  `json:"success_rate"`
	GrantViolations   int      `json:"grant_violations"`
	GrantEscapes      int      `json:"grant_escapes"`
	DeadlineKills     int      `json:"deadline_kills"`
	ModelErrors       int      `json:"model_errors"`
	MeanRepairRounds  float64  `json:"mean_repair_rounds"`
	MedianModelTokens *float64 `json:"median_model_tokens"`
	MedianReviewToks  *float64 `json:"median_review_tokens_successful"`
	MedianReviewLines *float64 `json:"median_review_lines_successful"`
}
type LiveSummary struct {
	Schema   string                    `json:"schema"`
	Model    string                    `json:"model"`
	Arms     map[string]LiveArmSummary `json:"arms"`
	Losses   []string                  `json:"howlframe_losses"`
	Wins     []string                  `json:"howlframe_wins"`
	Verdicts map[string]string         `json:"per_metric"`
}

// Summarize aggregates every result; nothing is dropped or reclassified.
func Summarize(results []LiveResult) LiveSummary {
	s := LiveSummary{Schema: "howlframe.live.summary/v1", Arms: map[string]LiveArmSummary{}, Losses: []string{}, Wins: []string{}, Verdicts: map[string]string{}}
	toks, rt, rl := map[string][]float64{}, map[string][]float64{}, map[string][]float64{}
	repairs := map[string]int{}
	cell := map[string]map[string]bool{}
	for _, r := range results {
		s.Model = r.Model
		a := s.Arms[r.Arm]
		a.Runs++
		repairs[r.Arm] += r.RepairRounds
		toks[r.Arm] = append(toks[r.Arm], float64(r.InputTokens+r.OutputTokens))
		switch r.Verdict {
		case VerdictSuccess:
			a.Successes++
			rt[r.Arm] = append(rt[r.Arm], float64(r.Review.Tokens))
			rl[r.Arm] = append(rl[r.Arm], float64(r.Review.Lines))
		case VerdictGrantViolation:
			a.GrantViolations++
		case VerdictDeadlineKilled:
			a.DeadlineKills++
		case VerdictModelError:
			a.ModelErrors++
		}
		for _, v := range r.Violations {
			for _, e := range v.Effects {
				if e.Kind != "" {
					a.GrantEscapes++ // an unauthorized effect that happened, not just a denial
				}
			}
		}
		s.Arms[r.Arm] = a
		k := r.TaskID + "/" + r.Seed
		if cell[k] == nil {
			cell[k] = map[string]bool{}
		}
		cell[k][r.Arm] = r.Success
	}
	for arm, a := range s.Arms {
		if a.Runs > 0 {
			a.SuccessRate = float64(a.Successes) / float64(a.Runs)
			a.MeanRepairRounds = float64(repairs[arm]) / float64(a.Runs)
		}
		a.MedianModelTokens, a.MedianReviewToks, a.MedianReviewLines = median(toks[arm]), median(rt[arm]), median(rl[arm])
		s.Arms[arm] = a
	}
	keys := []string{}
	for k := range cell {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c := cell[k]
		if c["b"] && !c["a"] {
			s.Losses = append(s.Losses, k)
		}
		if c["a"] && !c["b"] {
			s.Wins = append(s.Wins, k)
		}
	}
	A, B := s.Arms["a"], s.Arms["b"]
	cmp := func(name string, a, b float64, higherBetter bool) {
		switch {
		case a == b:
			s.Verdicts[name] = "tie"
		case (a > b) == higherBetter:
			s.Verdicts[name] = "howlframe_better"
		default:
			s.Verdicts[name] = "howlframe_worse"
		}
	}
	cmp("success_rate", A.SuccessRate, B.SuccessRate, true)
	cmp("grant_violations", float64(A.GrantViolations), float64(B.GrantViolations), false)
	cmp("grant_escapes", float64(A.GrantEscapes), float64(B.GrantEscapes), false)
	cmp("mean_repair_rounds", A.MeanRepairRounds, B.MeanRepairRounds, false)
	cmp("median_model_tokens", value(A.MedianModelTokens), value(B.MedianModelTokens), false)
	cmp("median_review_tokens", value(A.MedianReviewToks), value(B.MedianReviewToks), false)
	return s
}

func f(p *float64) string {
	if p == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", *p)
}

// RenderReport always lists both arms, every per-metric outcome including
// those HowlFrame loses, and every task/seed where only JSON plans succeeded.
func RenderReport(s LiveSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# live-v1 results (model: %s)\n\n", s.Model)
	b.WriteString("Arm A = HowlFrame, Arm B = JSON action plans. Computed from every recorded run; nothing excluded.\n\n")
	b.WriteString("| metric | HowlFrame (A) | JSON plans (B) |\n|---|---|---|\n")
	A, B := s.Arms["a"], s.Arms["b"]
	row := func(n, a, bb string) { fmt.Fprintf(&b, "| %s | %s | %s |\n", n, a, bb) }
	row("runs", fmt.Sprint(A.Runs), fmt.Sprint(B.Runs))
	row("success rate", fmt.Sprintf("%.1f%% (%d)", 100*A.SuccessRate, A.Successes), fmt.Sprintf("%.1f%% (%d)", 100*B.SuccessRate, B.Successes))
	row("attempted grant violations", fmt.Sprint(A.GrantViolations), fmt.Sprint(B.GrantViolations))
	row("grant escapes (effect happened)", fmt.Sprint(A.GrantEscapes), fmt.Sprint(B.GrantEscapes))
	row("deadline kills", fmt.Sprint(A.DeadlineKills), fmt.Sprint(B.DeadlineKills))
	row("model errors", fmt.Sprint(A.ModelErrors), fmt.Sprint(B.ModelErrors))
	row("mean repair rounds", fmt.Sprintf("%.2f", A.MeanRepairRounds), fmt.Sprintf("%.2f", B.MeanRepairRounds))
	row("median model tokens", f(A.MedianModelTokens), f(B.MedianModelTokens))
	row("median review tokens (successful)", f(A.MedianReviewToks), f(B.MedianReviewToks))
	row("median review lines (successful)", f(A.MedianReviewLines), f(B.MedianReviewLines))
	b.WriteString("\n## Per-metric outcome\n\n")
	names := []string{}
	for k := range s.Verdicts {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(&b, "- %s: %s\n", k, s.Verdicts[k])
	}
	fmt.Fprintf(&b, "\n## Where HowlFrame lost (B succeeded, A did not): %d\n\n", len(s.Losses))
	for _, k := range s.Losses {
		fmt.Fprintf(&b, "- %s\n", k)
	}
	fmt.Fprintf(&b, "\n## Where HowlFrame won (A succeeded, B did not): %d\n\n", len(s.Wins))
	for _, k := range s.Wins {
		fmt.Fprintf(&b, "- %s\n", k)
	}
	return b.String()
}
