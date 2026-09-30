package main

import (
	"encoding/json"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/hfir"
	"github.com/howlcipher/howlframe/internal/testutil"
)

type conformanceFile struct {
	ABI   string            `json:"abi"`
	Cases []conformanceCase `json:"cases"`
}

type conformanceCase struct {
	Name           string   `json:"name"`
	Fixture        string   `json:"fixture"`
	AllowCaps      *string  `json:"allow_caps"`
	DenyAll        bool     `json:"deny_all"`
	Targets        []Target `json:"targets"`
	JavaScriptRoot string   `json:"javascript_root"`
	Expect         string   `json:"expect"`
	Stdout         string   `json:"stdout"`
	StdoutKeys     []string `json:"stdout_keys"`
	Forbid         string   `json:"forbid"`
}

func loadConformance(t *testing.T) (string, conformanceFile) {
	t.Helper()
	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	path := filepath.Join(root, "tests", "conformance", "lowered_hfir_abi_v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read conformance manifest: %v", err)
	}
	var file conformanceFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatalf("parse conformance manifest: %v", err)
	}
	return root, file
}

func (c conformanceCase) runOptions() RunOptions {
	return RunOptions{
		AllowCaps: c.AllowCaps,
		DenyAll:   c.DenyAll,
		JSRoot:    c.JavaScriptRoot,
	}
}

func (c conformanceCase) wantStdout() string {
	if len(c.StdoutKeys) == 0 {
		return c.Stdout
	}
	keys := append([]string(nil), c.StdoutKeys...)
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// TestLoweredHFIRABIConformance runs the v1 suite through the existing
// differential harness. Passing cases must share stdout. Rejections must
// share one error class and must not print a forbidden value.
func TestLoweredHFIRABIConformance(t *testing.T) {
	unlockFetch := testutil.LockABIFetchFixture(t)
	t.Cleanup(unlockFetch)
	t.Setenv("HOWLFRAME_ABI_SECRET", "phase1-token")
	// Loopback is the fixture. A proxy would hide a denied request or miss the
	// granted one. 08_fetch_capability.howl uses this exact address.
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	// 07_read_file_capability.howl reads this absolute path. Generated Go runs
	// in its own temp directory, so a relative fixture path would miss.
	const readMarkerPath = "/tmp/howlframe-abi-v1-phase2c.txt"
	if err := os.WriteFile(readMarkerPath, []byte("phase2c-read-marker"), 0o644); err != nil {
		t.Fatalf("write read_file marker: %v", err)
	}
	t.Cleanup(func() { os.Remove(readMarkerPath) })
	const writeMarkerPath = "/tmp/howlframe-abi-v1-phase2e-write.txt"
	const writeMarkerBody = "phase2e-write-marker"
	const mkdirMarkerPath = "/tmp/howlframe-abi-v1-phase2e-dir"
	const dogfoodPrefix = "/tmp/howlframe-abi-v1-dogfood"
	const hostReadPrefix = "/tmp/howlframe-abi-v1-hostread"
	hostReadTaken := map[string]string{
		hostReadPrefix + "-read.txt": "dogfood-read-marker",
		hostReadPrefix + "-miss.txt": "dogfood-miss-marker",
	}
	hostReadUntaken := []string{
		hostReadPrefix + "-skip.txt",
		hostReadPrefix + "-else.txt",
		hostReadPrefix + "-empty.txt",
		hostReadPrefix + "-dead.txt",
	}
	writeHostRead := func() {
		for path, body := range hostReadTaken {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatalf("write host-read marker %s: %v", path, err)
			}
		}
		for _, path := range hostReadUntaken {
			os.Remove(path)
		}
	}
	writeHostRead()
	dogfoodFiles := []string{
		dogfoodPrefix + "-write.txt",
		dogfoodPrefix + "-loop.txt",
		dogfoodPrefix + "-skip.txt",
		dogfoodPrefix + "-else.txt",
		dogfoodPrefix + "-empty.txt",
	}
	dogfoodDirs := []string{
		dogfoodPrefix + "-dir",
		dogfoodPrefix + "-miss",
		dogfoodPrefix + "-skip",
		dogfoodPrefix + "-dead",
	}
	cleanDogfood := func() {
		for _, path := range dogfoodFiles {
			os.Remove(path)
		}
		for _, path := range dogfoodDirs {
			os.RemoveAll(path)
		}
	}
	t.Cleanup(func() {
		os.Remove(writeMarkerPath)
		os.RemoveAll(mkdirMarkerPath)
		cleanDogfood()
		for path := range hostReadTaken {
			os.Remove(path)
		}
		for _, path := range hostReadUntaken {
			os.Remove(path)
		}
	})
	const fetchAddr = "127.0.0.1:47653"
	var fetchHits atomic.Int64
	ln, err := net.Listen("tcp", fetchAddr)
	if err != nil {
		t.Fatalf("listen fetch fixture: %v", err)
	}
	fetchSrv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/howlframe-abi-v1-phase2d" {
			http.NotFound(w, r)
			return
		}
		fetchHits.Add(1)
		_, _ = w.Write([]byte("phase2d-fetch-marker"))
	})}
	go func() { _ = fetchSrv.Serve(ln) }()
	t.Cleanup(func() { _ = fetchSrv.Close() })
	root, file := loadConformance(t)
	if file.ABI != hfir.LoweredABIV1 {
		t.Fatalf("manifest abi = %q, want %q", file.ABI, hfir.LoweredABIV1)
	}
	if len(file.Cases) == 0 {
		t.Fatal("conformance manifest has no cases")
	}

	for _, tc := range file.Cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if tc.DenyAll && tc.AllowCaps != nil {
				t.Fatal("case sets both deny_all and allow_caps")
			}
			fixture := filepath.Join(root, tc.Fixture)
			switch tc.Name {
			case "write_file_denied", "write_file_granted":
				os.Remove(writeMarkerPath)
			case "mkdir_denied", "mkdir_granted":
				os.RemoveAll(mkdirMarkerPath)
			case "nested_fs_write_denied", "nested_fs_write_granted":
				cleanDogfood()
			case "nested_env_read_denied", "nested_env_read_granted":
				writeHostRead()
			}
			hitsBefore := fetchHits.Load()
			report, err := VerifyParityWithOptions(fixture, tc.Targets, tc.runOptions())
			if err != nil {
				t.Fatalf("VerifyParityWithOptions: %v", err)
			}
			hits := fetchHits.Load() - hitsBefore
			if (tc.Name == "fetch_denied" || tc.Name == "nested_fetch_denied") && hits != 0 {
				t.Fatalf("%s performed %d HTTP request(s)", tc.Name, hits)
			}
			if (tc.Name == "fetch_granted" || tc.Name == "nested_fetch_granted") && hits == 0 {
				t.Fatalf("%s performed no HTTP request", tc.Name)
			}
			if report.OverallStatus != StatusPass {
				t.Fatalf("parity: %s", strings.Join(report.Discrepancies, "\n"))
			}

			wantOut := tc.wantStdout()
			if tc.Expect == "PASS" {
				if NormalizeOutput(report.CanonicalResult.Stdout) != NormalizeOutput(wantOut) {
					t.Fatalf("canonical stdout = %q, want %q", NormalizeOutput(report.CanonicalResult.Stdout), NormalizeOutput(wantOut))
				}
				if report.CanonicalResult.Status != StatusPass {
					t.Fatalf("canonical status = %s, want PASS (%s)", report.CanonicalResult.Status, report.CanonicalResult.ErrorMessage)
				}
			} else {
				if report.CanonicalResult.Status != StatusRuntimeFailure {
					t.Fatalf("canonical status = %s, want runtime failure (%s)", report.CanonicalResult.Status, report.CanonicalResult.ErrorMessage)
				}
				if report.CanonicalResult.ErrorClass != tc.Expect {
					t.Fatalf("canonical error class = %q, want %q\n%s", report.CanonicalResult.ErrorClass, tc.Expect, report.CanonicalResult.ErrorMessage)
				}
			}

			for _, tgt := range tc.Targets {
				res := report.TargetResults[tgt]
				if tgt == TargetJavaScript && res.Status == StatusBackendUnsupported && strings.Contains(res.ErrorMessage, "node runtime not available") {
					t.Log("javascript skipped: node runtime not available")
					continue
				}
				if tc.Expect != "PASS" {
					if res.ErrorClass != tc.Expect {
						t.Errorf("%s error class = %q, want %q\n%s", tgt, res.ErrorClass, tc.Expect, res.ErrorMessage)
					}
					if res.ExitCode == 0 {
						t.Errorf("%s exit = 0, want rejection", tgt)
					}
					if strings.TrimSpace(res.Stdout) != "" {
						t.Errorf("%s stdout = %q, want empty on rejection", tgt, res.Stdout)
					}
				}
				if tc.Forbid != "" && strings.Contains(res.Stdout+res.Stderr+res.ErrorMessage, tc.Forbid) {
					t.Errorf("%s leaked %q", tgt, tc.Forbid)
				}
			}
			switch tc.Name {
			case "write_file_denied":
				if _, err := os.Stat(writeMarkerPath); !os.IsNotExist(err) {
					t.Fatalf("write_file_denied created %s", writeMarkerPath)
				}
			case "write_file_granted":
				got, err := os.ReadFile(writeMarkerPath)
				if err != nil {
					t.Fatalf("write_file_granted did not write: %v", err)
				}
				if string(got) != writeMarkerBody {
					t.Fatalf("write_file_granted body = %q, want %s", got, writeMarkerBody)
				}
			case "mkdir_denied":
				if _, err := os.Stat(mkdirMarkerPath); !os.IsNotExist(err) {
					t.Fatalf("mkdir_denied created %s", mkdirMarkerPath)
				}
			case "mkdir_granted":
				info, err := os.Stat(mkdirMarkerPath)
				if err != nil || !info.IsDir() {
					t.Fatalf("mkdir_granted did not create a directory: %v", err)
				}
			case "nested_fs_write_denied":
				for _, path := range append(append([]string{}, dogfoodFiles...), dogfoodDirs...) {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("nested_fs_write_denied created %s", path)
					}
				}
			case "nested_fs_write_granted":
				got, err := os.ReadFile(dogfoodPrefix + "-write.txt")
				if err != nil {
					t.Fatalf("nested write did not land: %v", err)
				}
				if string(got) != "dogfood-write-marker" {
					t.Fatalf("nested write body = %q", got)
				}
				got, err = os.ReadFile(dogfoodPrefix + "-loop.txt")
				if err != nil {
					t.Fatalf("nested loop write did not land: %v", err)
				}
				if string(got) != "dogfood-loop-marker" {
					t.Fatalf("nested loop body = %q", got)
				}
				for _, path := range []string{dogfoodPrefix + "-dir", dogfoodPrefix + "-miss"} {
					info, err := os.Stat(path)
					if err != nil || !info.IsDir() {
						t.Fatalf("nested mkdir %s: %v", path, err)
					}
				}
				for _, path := range []string{
					dogfoodPrefix + "-skip.txt",
					dogfoodPrefix + "-else.txt",
					dogfoodPrefix + "-empty.txt",
					dogfoodPrefix + "-skip",
					dogfoodPrefix + "-dead",
				} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("untaken filesystem path was created: %s", path)
					}
				}
			case "nested_env_read_denied", "nested_env_read_granted":
				for _, tgt := range tc.Targets {
					res := report.TargetResults[tgt]
					blob := res.Stdout + res.Stderr + res.ErrorMessage
					if strings.Contains(blob, hostReadPrefix) {
						t.Errorf("%s leaked the host-read path", tgt)
					}
					if tc.Name == "nested_env_read_denied" && (strings.Contains(blob, "dogfood-read-marker") || strings.Contains(blob, "dogfood-miss-marker") || strings.Contains(blob, "phase1-token")) {
						t.Errorf("%s leaked a host-read marker or secret: %s", tgt, blob)
					}
				}
				for path, body := range hostReadTaken {
					got, err := os.ReadFile(path)
					if err != nil {
						t.Fatalf("host-read marker %s: %v", path, err)
					}
					if string(got) != body {
						t.Fatalf("host-read marker %s changed to %q", path, got)
					}
				}
				for _, path := range hostReadUntaken {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatalf("untaken host-read path was created: %s", path)
					}
				}
			}
		})
	}
}

func TestCompareToCanonicalDetectsDrift(t *testing.T) {
	canonical := ExecutionResult{Target: TargetBytecode, Status: StatusPass, Stdout: "14\n", ExitCode: 0}
	drifted := canonical
	drifted.Stdout = "15\n"
	if diffs := compareToCanonical(canonical, drifted, TargetGo); len(diffs) == 0 {
		t.Fatal("stdout drift was not detected")
	}

	denied := ExecutionResult{Status: StatusRuntimeFailure, ErrorClass: "CAPABILITY_DENIED", ExitCode: 1}
	other := ExecutionResult{Status: StatusRuntimeFailure, ErrorClass: "TYPE_ERROR", ExitCode: 1}
	if diffs := compareToCanonical(denied, other, TargetInterpreter); len(diffs) == 0 {
		t.Fatal("error-class drift was not detected")
	}

	same := denied
	if diffs := compareToCanonical(denied, same, TargetInterpreter); len(diffs) != 0 {
		t.Fatalf("matching rejection reported drift: %v", diffs)
	}
}

func TestABIPropertyArithmetic(t *testing.T) {
	rng := rand.New(rand.NewSource(90))
	targets := []Target{TargetHFIRBytecode, TargetBytecode, TargetInterpreter, TargetGo, TargetJavaScript}
	opts := RunOptions{DenyAll: true, JSRoot: "web_app"}
	var firstStdout string
	var firstSource string

	for i := 0; i < 8; i++ {
		node := genArith(rng, 2)
		expr := formatArith(node)
		want := strconv.Itoa(evalArith(node))
		dir := t.TempDir()
		path := filepath.Join(dir, "arith.howl")
		source := "(cli_app (print " + expr + "))\n"
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		report, err := VerifyParityWithOptions(path, targets, opts)
		if err != nil {
			t.Fatalf("case %d %s: %v", i, expr, err)
		}
		if report.OverallStatus != StatusPass {
			t.Fatalf("case %d %s parity:\n%s", i, expr, strings.Join(report.Discrepancies, "\n"))
		}
		if got := NormalizeOutput(report.CanonicalResult.Stdout); got != want {
			t.Fatalf("case %d %s stdout = %q, want %q", i, expr, got, want)
		}
		if i == 0 {
			firstStdout = report.CanonicalResult.Stdout
			firstSource = source
			again, err := VerifyParityWithOptions(path, []Target{TargetBytecode}, opts)
			if err != nil {
				t.Fatal(err)
			}
			if again.CanonicalResult.Stdout != firstStdout {
				t.Fatalf("repeat of %s stdout = %q, want %q", firstSource, again.CanonicalResult.Stdout, firstStdout)
			}
		}
	}
}

type arithExpr struct {
	op    string
	value int
	left  *arithExpr
	right *arithExpr
}

func genArith(rng *rand.Rand, depth int) *arithExpr {
	if depth == 0 || rng.Intn(4) == 0 {
		// Literals stay non-negative. The lexer has no negative integer token.
		return &arithExpr{value: rng.Intn(13)}
	}
	ops := []string{"+", "-", "*"}
	return &arithExpr{
		op:    ops[rng.Intn(len(ops))],
		left:  genArith(rng, depth-1),
		right: genArith(rng, depth-1),
	}
}

func evalArith(node *arithExpr) int {
	switch node.op {
	case "":
		return node.value
	case "+":
		return evalArith(node.left) + evalArith(node.right)
	case "-":
		return evalArith(node.left) - evalArith(node.right)
	case "*":
		return evalArith(node.left) * evalArith(node.right)
	default:
		panic("unknown op " + node.op)
	}
}

func formatArith(node *arithExpr) string {
	if node.op == "" {
		return strconv.Itoa(node.value)
	}
	return "(" + node.op + " " + formatArith(node.left) + " " + formatArith(node.right) + ")"
}
