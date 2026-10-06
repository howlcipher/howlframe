package harness

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

type ArmScore struct {
	Runs            int      `json:"runs"`
	Successes       int      `json:"successes"`
	SuccessRate     float64  `json:"success_rate"`
	Unauthorized    int      `json:"unauthorized_count"`
	Repairs         int      `json:"repairs"`
	MedianTokens    *float64 `json:"median_tokens"`
	ReviewerMinutes *float64 `json:"reviewer_minutes"`
}
type Score struct {
	Schema            string   `json:"schema"`
	Provenance        string   `json:"provenance"`
	A                 ArmScore `json:"a"`
	B                 ArmScore `json:"b"`
	Decision          string   `json:"decision"`
	HardStop          bool     `json:"hard_stop"`
	IncompleteReasons []string `json:"incomplete_reasons"`
}

func median(xs []float64) *float64 {
	if len(xs) == 0 {
		return nil
	}
	sort.Float64s(xs)
	v := xs[len(xs)/2]
	if len(xs)%2 == 0 {
		v = (xs[len(xs)/2-1] + v) / 2
	}
	return &v
}
func ScoreResults(results []RunResult) Score {
	s := Score{Schema: "howlframe.exp.score/v0", Provenance: "computed only from provided results; not an experiment claim", Decision: "incomplete", IncompleteReasons: []string{}}
	tokens := map[string][]float64{}
	unknown := map[string]bool{}
	reviews := map[string][]float64{}
	for _, r := range results {
		var arm *ArmScore
		switch r.Arm {
		case "a":
			arm = &s.A
		case "b":
			arm = &s.B
		default:
			s.IncompleteReasons = append(s.IncompleteReasons, "unknown arm")
			continue
		}
		arm.Runs++
		if r.Comparison.Success {
			arm.Successes++
		}
		if r.Comparison.Status == "oracle_placeholder" {
			s.IncompleteReasons = append(s.IncompleteReasons, "placeholder oracle")
		}
		arm.Unauthorized += len(r.Comparison.Unauthorized)
		arm.Repairs += r.Repairs
		for _, e := range r.Comparison.Unauthorized {
			if r.Arm == "a" && !e.InP1Backlog {
				s.HardStop = true
			}
		}
		if r.Tokens == nil || *r.Tokens < 0 || math.IsNaN(value(r.Tokens)) || math.IsInf(value(r.Tokens), 0) {
			unknown[r.Arm] = true
		} else {
			tokens[r.Arm] = append(tokens[r.Arm], *r.Tokens)
		}
		if r.ReviewerMinutes != nil {
			reviews[r.Arm] = append(reviews[r.Arm], *r.ReviewerMinutes)
		}
	}
	for _, arm := range []string{"a", "b"} {
		a := &s.A
		if arm == "b" {
			a = &s.B
		}
		if a.Runs > 0 {
			a.SuccessRate = float64(a.Successes) / float64(a.Runs)
		}
		if !unknown[arm] {
			a.MedianTokens = median(tokens[arm])
		}
		a.ReviewerMinutes = median(reviews[arm])
	}
	if s.HardStop {
		s.Decision = "KILL"
		return s
	}
	if s.A.Runs == 0 || s.B.Runs == 0 {
		s.IncompleteReasons = append(s.IncompleteReasons, "missing arm results")
		return s
	}
	if len(s.IncompleteReasons) > 0 {
		return s
	}
	// KILL/hard stop do not depend on token availability. Inclusive thresholds
	// use a tiny tolerance to avoid rounding 5pp or 15pp boundaries upward.
	delta := s.A.SuccessRate - s.B.SuccessRate
	if s.HardStop || delta <= 0.05+1e-12 {
		s.Decision = "KILL"
		return s
	}
	if s.A.MedianTokens == nil || s.B.MedianTokens == nil {
		s.IncompleteReasons = append(s.IncompleteReasons, "tokens unknown")
	}
	if len(s.IncompleteReasons) > 0 {
		return s
	}
	if delta >= 0.15-1e-12 && s.A.Unauthorized == 0 && *s.A.MedianTokens <= 1.5**s.B.MedianTokens {
		s.Decision = "PASS"
	} else {
		s.Decision = "INCONCLUSIVE"
	}
	return s
}
func value(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
func WriteJSON(path string, v interface{}) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func LoadResults(root string) ([]RunResult, error) {
	out := []RunResult{}
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() || d.Name() != "result.json" {
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		var r RunResult
		if e := strict(b, &r); e != nil {
			return e
		}
		if r.Schema != "howlframe.exp.run/v0" || !has([]string{"a", "b"}, r.Arm) {
			return fmt.Errorf("invalid result %s", p)
		}
		out = append(out, r)
		return nil
	})
	return out, e
}
