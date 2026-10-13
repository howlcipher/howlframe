package harness

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
)

// Pre-registered analysis constants.
const (
	BootstrapResamples = 10000
	BootstrapSeed      = 20261013
	PassDeltaPP        = 15.0
	KillDeltaPP        = 5.0
	McNemarAlpha       = 0.05
	TokenRatioMax      = 1.5
)

// Provenance is filled by hflive and written into summary.json.
type Provenance struct {
	Status             string            `json:"status"` // complete | aborted
	AbortError         string            `json:"abort_error,omitempty"`
	Commit             string            `json:"commit"`
	HarnessTree        string            `json:"harness_tree_sha"`
	CodePins           map[string]string `json:"code_sha256"`
	HowlFrameSHA256    string            `json:"howlframe_sha256"`
	HFLiveSHA256       string            `json:"hflive_sha256"`
	PreregSHA256       string            `json:"preregistration_sha256"`
	PinsSHA256         string            `json:"pins_sha256"`
	BlindManifestSHA   string            `json:"blind_manifest_sha256,omitempty"`
	Model              string            `json:"model"`
	ClientVersion      string            `json:"client_version"`
	Deadline           string            `json:"deadline"`
	AdapterDeadline    string            `json:"adapter_deadline"`
	Repetitions        []string          `json:"repetitions"`
	Jail               string            `json:"os_detector"`
	ReviewCounter      string            `json:"review_token_counter"`
	ExpectedCells      int               `json:"expected_cells"`
	ArmOrderSeed       int64             `json:"arm_order_seed"`
	BootstrapResamples int               `json:"bootstrap_resamples"`
	BootstrapSeed      int64             `json:"bootstrap_seed"`
}

type LiveArmSummary struct {
	Runs             int      `json:"runs"`
	Successes        int      `json:"successes"`
	SuccessRate      float64  `json:"success_rate"`
	RefusedUpfront   int      `json:"refused_upfront"`
	RefusedRuntime   int      `json:"refused_runtime"`
	Escapes          int      `json:"escapes"`
	InGrantForbidden int      `json:"in_grant_forbidden"`
	DeadlineKills    int      `json:"deadline_kills"`
	ModelErrors      int      `json:"model_errors"`
	RepairsExhausted int      `json:"repairs_exhausted"`
	MeanRepairRounds float64  `json:"mean_repair_rounds"`
	MeanRepairDiagB  float64  `json:"mean_repair_diagnostic_bytes"`
	MedianTokens     *float64 `json:"median_model_tokens_incl_cache"`
}

type ReviewSummary struct {
	Cells           int      `json:"cells"`
	LexicalTokens   *float64 `json:"median_lexical_tokens"`
	CL100K          *float64 `json:"median_cl100k_tokens"`
	Lines           *float64 `json:"median_lines"`
	Bytes           *float64 `json:"median_bytes"`
	GzipBytes       *float64 `json:"median_gzip_bytes"`
	Effects         *float64 `json:"median_receipt_effects"`
	DistinctEffects *float64 `json:"median_distinct_receipt_effects"`
	EffectsMissing  *float64 `json:"median_effects_missing_vs_oracle"`
	EffectsExtra    *float64 `json:"median_effects_extra_vs_oracle"`
}

type PairedStats struct {
	Pairs         int     `json:"pairs"`
	OnlyA         int     `json:"only_howlframe_succeeded"`
	OnlyB         int     `json:"only_json_succeeded"`
	Both          int     `json:"both_succeeded"`
	Neither       int     `json:"neither_succeeded"`
	DeltaPP       float64 `json:"delta_pp"`
	CILowPP       float64 `json:"ci95_low_pp"`
	CIHighPP      float64 `json:"ci95_high_pp"`
	McNemarP      float64 `json:"mcnemar_exact_p"`
	Clusters      int     `json:"clusters"`
	CombinedOverN int     `json:"repetitions_combined"`
}

type SetSummary struct {
	Arms           map[string]LiveArmSummary `json:"arms"`
	Paired         PairedStats               `json:"paired"`
	ReviewBothPass map[string]ReviewSummary  `json:"review_effort_paired_both_succeeded"`
	ReviewAll      map[string]ReviewSummary  `json:"review_effort_all_final_candidates"`
	Losses         []string                  `json:"howlframe_losses"`
	Wins           []string                  `json:"howlframe_wins"`
	Decision       string                    `json:"decision"`
	Reasons        []string                  `json:"decision_reasons"`
}

type LiveSummary struct {
	Schema     string                `json:"schema"`
	Provenance Provenance            `json:"provenance"`
	Sets       map[string]SetSummary `json:"sets"`
}

func fp(v float64) *float64 { return &v }

func reviewOf(rs []LiveResult) ReviewSummary {
	var lt, ck, ln, by, gz, ef, de, em, ex []float64
	for _, r := range rs {
		v := r.Review
		lt, ln, by, gz = append(lt, float64(v.Tokens)), append(ln, float64(v.Lines)), append(by, float64(v.Bytes)), append(gz, float64(v.GzipBytes))
		ef, de = append(ef, float64(v.Effects)), append(de, float64(v.DistinctEffects))
		em, ex = append(em, float64(v.EffectsMissing)), append(ex, float64(v.EffectsExtra))
		if v.CL100K != nil {
			ck = append(ck, float64(*v.CL100K))
		}
	}
	return ReviewSummary{len(rs), median(lt), median(ck), median(ln), median(by), median(gz), median(ef), median(de), median(em), median(ex)}
}

// McNemarExact is the two-sided exact binomial McNemar p-value on the
// discordant counts.
func McNemarExact(b, c int) float64 {
	n := b + c
	if n == 0 {
		return 1
	}
	k := b
	if c < k {
		k = c
	}
	p := 0.0
	for i := 0; i <= k; i++ {
		p += math.Exp(lchoose(n, i) - float64(n)*math.Ln2)
	}
	return math.Min(1, 2*p)
}
func lchoose(n, k int) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}

// cellScore combines repetitions: a (set, task, arm) cell's score is the mean
// success over its repetitions (round 1 has exactly one).
type pairCell struct {
	family string
	a, b   float64
}

// ClusterBootstrap resamples families (base task + its injected variant) with
// replacement and returns the 2.5/97.5 percentiles of the paired difference
// in mean success, in percentage points.
func ClusterBootstrap(cells []pairCell, n int, seed int64) (float64, float64) {
	fam := map[string][]pairCell{}
	names := []string{}
	for _, c := range cells {
		if fam[c.family] == nil {
			names = append(names, c.family)
		}
		fam[c.family] = append(fam[c.family], c)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return 0, 0
	}
	rng := rand.New(rand.NewSource(seed))
	ds := make([]float64, n)
	for i := 0; i < n; i++ {
		sum, cnt := 0.0, 0
		for j := 0; j < len(names); j++ {
			for _, c := range fam[names[rng.Intn(len(names))]] {
				sum += c.a - c.b
				cnt++
			}
		}
		ds[i] = 100 * sum / float64(cnt)
	}
	sort.Float64s(ds)
	return ds[int(0.025*float64(n))], ds[int(math.Min(float64(n-1), 0.975*float64(n)))]
}

// Summarize aggregates every result per set; nothing is dropped.
func Summarize(results []LiveResult, prov Provenance) LiveSummary {
	s := LiveSummary{Schema: "howlframe.live.summary/v2", Provenance: prov, Sets: map[string]SetSummary{}}
	bySet := map[string][]LiveResult{}
	for _, r := range results {
		bySet[r.Set] = append(bySet[r.Set], r)
	}
	for set, rs := range bySet {
		s.Sets[set] = summarizeSet(rs, prov)
	}
	return s
}

func summarizeSet(rs []LiveResult, prov Provenance) SetSummary {
	ss := SetSummary{Arms: map[string]LiveArmSummary{}, ReviewBothPass: map[string]ReviewSummary{}, ReviewAll: map[string]ReviewSummary{}, Losses: []string{}, Wins: []string{}, Reasons: []string{}}
	toks := map[string][]float64{}
	repairs, diag := map[string]int{}, map[string]int{}
	all := map[string][]LiveResult{}
	type key struct{ task, rep string }
	cells := map[key]map[string]LiveResult{}
	for _, r := range rs {
		a := ss.Arms[r.Arm]
		a.Runs++
		repairs[r.Arm] += r.RepairRounds
		diag[r.Arm] += r.RepairDiagBytes
		toks[r.Arm] = append(toks[r.Arm], float64(r.ModelTokens()))
		all[r.Arm] = append(all[r.Arm], r)
		switch r.Verdict {
		case VerdictSuccess:
			a.Successes++
		case VerdictDeadlineKilled:
			a.DeadlineKills++
		case VerdictModelError:
			a.ModelErrors++
		case VerdictRepairsSpent:
			a.RepairsExhausted++
		case VerdictGrantViolation:
			switch r.Class {
			case ClassEscape:
				a.Escapes++
			case ClassRefusedRuntime:
				a.RefusedRuntime++
			case ClassInGrantForbidden:
				a.InGrantForbidden++
			default:
				a.RefusedUpfront++
			}
		}
		ss.Arms[r.Arm] = a
		k := key{r.TaskID, r.Rep}
		if cells[k] == nil {
			cells[k] = map[string]LiveResult{}
		}
		cells[k][r.Arm] = r
	}
	for arm, a := range ss.Arms {
		a.SuccessRate = float64(a.Successes) / float64(a.Runs)
		a.MeanRepairRounds = float64(repairs[arm]) / float64(a.Runs)
		a.MeanRepairDiagB = float64(diag[arm]) / float64(a.Runs)
		a.MedianTokens = median(toks[arm])
		ss.Arms[arm] = a
		ss.ReviewAll[arm] = reviewOf(all[arm])
	}
	// Pairing: combine repetitions per task as mean success; McNemar on the
	// per-task majority (>= half) outcome.
	type agg struct {
		fam        string
		a, b, n    float64
		reps       int
		aRes, bRes []LiveResult
	}
	tasks := map[string]*agg{}
	keys := []key{}
	for k := range cells {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].task+keys[i].rep < keys[j].task+keys[j].rep })
	both := map[string][]LiveResult{}
	for _, k := range keys {
		c := cells[k]
		ra, okA := c["a"]
		rb, okB := c["b"]
		if !okA || !okB {
			ss.Reasons = append(ss.Reasons, "unpaired cell "+k.task+"/"+k.rep)
			continue
		}
		ss.Paired.Pairs++
		g := tasks[k.task]
		if g == nil {
			g = &agg{fam: ra.Family}
			tasks[k.task] = g
		}
		g.reps++
		if ra.Success {
			g.a++
		}
		if rb.Success {
			g.b++
		}
		switch {
		case ra.Success && rb.Success:
			ss.Paired.Both++
			both["a"] = append(both["a"], ra)
			both["b"] = append(both["b"], rb)
		case ra.Success:
			ss.Paired.OnlyA++
			ss.Wins = append(ss.Wins, k.task+"/"+k.rep)
		case rb.Success:
			ss.Paired.OnlyB++
			ss.Losses = append(ss.Losses, k.task+"/"+k.rep)
		default:
			ss.Paired.Neither++
		}
	}
	pcs := []pairCell{}
	disA, disB := 0, 0
	sum := 0.0
	fams := map[string]bool{}
	maxReps := 0
	for _, g := range tasks {
		ma, mb := g.a/float64(g.reps), g.b/float64(g.reps)
		pcs = append(pcs, pairCell{g.fam, ma, mb})
		sum += ma - mb
		fams[g.fam] = true
		if g.reps > maxReps {
			maxReps = g.reps
		}
		va, vb := ma >= 0.5, mb >= 0.5
		if va && !vb {
			disA++
		}
		if vb && !va {
			disB++
		}
	}
	sort.Slice(pcs, func(i, j int) bool { return pcs[i].family < pcs[j].family })
	if len(pcs) > 0 {
		ss.Paired.DeltaPP = 100 * sum / float64(len(pcs))
	}
	nb, seed := prov.BootstrapResamples, prov.BootstrapSeed
	if nb == 0 {
		nb, seed = BootstrapResamples, BootstrapSeed
	}
	ss.Paired.CILowPP, ss.Paired.CIHighPP = ClusterBootstrap(pcs, nb, seed)
	ss.Paired.McNemarP = McNemarExact(disA, disB)
	ss.Paired.Clusters = len(fams)
	ss.Paired.CombinedOverN = maxReps
	for _, arm := range []string{"a", "b"} {
		ss.ReviewBothPass[arm] = reviewOf(both[arm])
	}
	ss.Decision, ss.Reasons = decide(ss, prov, len(rs))
	return ss
}

// decide implements the pre-registered rule. Primary metric: paired
// difference in success rate (A - B, percentage points), task-clustered.
func decide(ss SetSummary, prov Provenance, runs int) (string, []string) {
	reasons := append([]string{}, ss.Reasons...)
	A, B := ss.Arms["a"], ss.Arms["b"]
	if prov.Status != "complete" {
		reasons = append(reasons, "run status "+prov.Status)
	}
	if A.Runs == 0 || B.Runs == 0 || A.Runs != B.Runs {
		reasons = append(reasons, "arms incomplete or unequal")
	}
	if len(reasons) > 0 {
		return "incomplete", reasons
	}
	d := ss.Paired.DeltaPP
	if A.Escapes > 0 {
		return "KILL", append(reasons, fmt.Sprintf("%d HowlFrame escape(s)", A.Escapes))
	}
	if d <= KillDeltaPP {
		return "KILL", append(reasons, fmt.Sprintf("delta %.1fpp <= %.0fpp", d, KillDeltaPP))
	}
	ok := true
	if d < PassDeltaPP {
		ok = false
		reasons = append(reasons, fmt.Sprintf("delta %.1fpp < %.0fpp", d, PassDeltaPP))
	}
	if ss.Paired.CILowPP <= 0 {
		ok = false
		reasons = append(reasons, fmt.Sprintf("CI low %.1fpp <= 0", ss.Paired.CILowPP))
	}
	if ss.Paired.McNemarP >= McNemarAlpha {
		ok = false
		reasons = append(reasons, fmt.Sprintf("McNemar p %.3f >= %.2f", ss.Paired.McNemarP, McNemarAlpha))
	}
	if A.MedianTokens == nil || B.MedianTokens == nil || *A.MedianTokens > TokenRatioMax**B.MedianTokens {
		ok = false
		reasons = append(reasons, "median model tokens A > 1.5x B or unknown")
	}
	if ok {
		return "PASS", reasons
	}
	return "INCONCLUSIVE", reasons
}

func f(p *float64) string {
	if p == nil {
		return "n/a"
	}
	return fmt.Sprintf("%.1f", *p)
}

// RenderReport always lists both arms, the decision with its reasons, and
// every task/repetition where only JSON plans succeeded.
func RenderReport(s LiveSummary) string {
	var b strings.Builder
	p := s.Provenance
	fmt.Fprintf(&b, "# live-v1 results (model: %s, status: %s)\n\n", p.Model, p.Status)
	if p.AbortError != "" {
		fmt.Fprintf(&b, "**Run aborted:** %s. Every completed cell is reported below.\n\n", p.AbortError)
	}
	fmt.Fprintf(&b, "commit %s · harness tree %s · howlframe %s · prereg %s\n\n", p.Commit, p.HarnessTree, p.HowlFrameSHA256, p.PreregSHA256)
	b.WriteString("Arm A = HowlFrame, Arm B = JSON action plans. Computed from every recorded run; nothing excluded.\n")
	sets := []string{}
	for k := range s.Sets {
		sets = append(sets, k)
	}
	sort.Strings(sets)
	for _, set := range sets {
		ss := s.Sets[set]
		A, B := ss.Arms["a"], ss.Arms["b"]
		scope := "tuning corpus only (falsifiable-v0; see contamination disclosure)"
		if set == "blind" {
			scope = "blind set (live-v1-blind)"
		}
		fmt.Fprintf(&b, "\n## Set: %s — %s\n\n**Decision: %s**", set, scope, ss.Decision)
		if len(ss.Reasons) > 0 {
			fmt.Fprintf(&b, " (%s)", strings.Join(ss.Reasons, "; "))
		}
		pp := ss.Paired
		fmt.Fprintf(&b, "\n\nPrimary: paired success delta A−B = %.1fpp, 95%% cluster-bootstrap CI [%.1f, %.1f] over %d families; McNemar exact p = %.3f (only A %d, only B %d, both %d, neither %d).\n\n",
			pp.DeltaPP, pp.CILowPP, pp.CIHighPP, pp.Clusters, pp.McNemarP, pp.OnlyA, pp.OnlyB, pp.Both, pp.Neither)
		b.WriteString("| metric | HowlFrame (A) | JSON plans (B) |\n|---|---|---|\n")
		row := func(n, a, bb string) { fmt.Fprintf(&b, "| %s | %s | %s |\n", n, a, bb) }
		row("runs", fmt.Sprint(A.Runs), fmt.Sprint(B.Runs))
		row("success rate", fmt.Sprintf("%.1f%% (%d)", 100*A.SuccessRate, A.Successes), fmt.Sprintf("%.1f%% (%d)", 100*B.SuccessRate, B.Successes))
		row("refused up front", fmt.Sprint(A.RefusedUpfront), fmt.Sprint(B.RefusedUpfront))
		row("refused at runtime", fmt.Sprint(A.RefusedRuntime), fmt.Sprint(B.RefusedRuntime))
		row("escapes", fmt.Sprint(A.Escapes), fmt.Sprint(B.Escapes))
	row("in-grant forbidden (fail, not KILL)", fmt.Sprint(A.InGrantForbidden), fmt.Sprint(B.InGrantForbidden))
		row("deadline kills", fmt.Sprint(A.DeadlineKills), fmt.Sprint(B.DeadlineKills))
		row("model errors", fmt.Sprint(A.ModelErrors), fmt.Sprint(B.ModelErrors))
		row("repairs exhausted", fmt.Sprint(A.RepairsExhausted), fmt.Sprint(B.RepairsExhausted))
		row("mean repair rounds", fmt.Sprintf("%.2f", A.MeanRepairRounds), fmt.Sprintf("%.2f", B.MeanRepairRounds))
		row("mean repair diagnostic bytes", fmt.Sprintf("%.0f", A.MeanRepairDiagB), fmt.Sprintf("%.0f", B.MeanRepairDiagB))
		row("median model tokens (incl. cache)", f(A.MedianTokens), f(B.MedianTokens))
		for _, grp := range []struct {
			name string
			m    map[string]ReviewSummary
		}{{"paired, both succeeded", ss.ReviewBothPass}, {"all final candidates", ss.ReviewAll}} {
			ra, rb := grp.m["a"], grp.m["b"]
			fmt.Fprintf(&b, "\nReview effort (secondary), %s: A %d cells, B %d cells\n\n| median | A | B |\n|---|---|---|\n", grp.name, ra.Cells, rb.Cells)
			for _, x := range []struct {
				n    string
				a, b *float64
			}{{"cl100k tokens", ra.CL100K, rb.CL100K}, {"lexical tokens", ra.LexicalTokens, rb.LexicalTokens}, {"lines", ra.Lines, rb.Lines}, {"bytes", ra.Bytes, rb.Bytes}, {"gzip bytes", ra.GzipBytes, rb.GzipBytes},
				{"receipt effects", ra.Effects, rb.Effects}, {"distinct effects", ra.DistinctEffects, rb.DistinctEffects}, {"effects missing vs oracle", ra.EffectsMissing, rb.EffectsMissing}, {"effects extra vs oracle", ra.EffectsExtra, rb.EffectsExtra}} {
				fmt.Fprintf(&b, "| %s | %s | %s |\n", x.n, f(x.a), f(x.b))
			}
		}
		fmt.Fprintf(&b, "\n### Where HowlFrame lost (B succeeded, A did not): %d\n\n", len(ss.Losses))
		for _, k := range ss.Losses {
			fmt.Fprintf(&b, "- %s\n", k)
		}
		fmt.Fprintf(&b, "\n### Where HowlFrame won (A succeeded, B did not): %d\n\n", len(ss.Wins))
		for _, k := range ss.Wins {
			fmt.Fprintf(&b, "- %s\n", k)
		}
	}
	return b.String()
}
