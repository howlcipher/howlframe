package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/construct"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

// HFIR model adapter coverage. The adapter (hfir-model-adapter/v1) must be
// able to carry every program the experimental -compile-hfir-bc path can
// lower, and every arity-table form must either be covered by such a program
// or carry an explicit, checked exemption. A program "is covered" when
// -compile-hfir-bc -hfir-adapter-roundtrip (encode the graph as adapter JSON,
// strictly decode it, CompileCandidate) writes a byte-identical artifact to
// plain -compile-hfir-bc.

// adapterFormSamples covers arity-table forms that no corpus program uses.
var adapterFormSamples = map[string]string{
	"dec":      `(do (let (x 3) (do (dec x) (print x))))`,
	"ltrim":    `(do (print (ltrim "  a")))`,
	"max_by":   `(do (print (max_by x (list 1 3 2) x)))`,
	"min_by":   `(do (print (min_by x (list 1 3 2) x)))`,
	"merge":    `(do (print (encode_json (merge (rec a 1) (rec b 2)))))`,
	"pluck":    `(do (print (encode_json (pluck (list (rec a 1)) "a"))))`,
	"put":      `(do (let (m (rec)) (do (put m "a" 1) (print (encode_json m)))))`,
	"reduce":   `(do (print (reduce acc 0 x (list 1 2 3) (+ acc x))))`,
	"select":   `(do (print (encode_json (select (list (rec a 1 b 2)) a))))`,
	"group_by": `(do (print (encode_json (group_by x (list 1 2 3) (% x 2)))))`,
	"switch":   `(do (let (v 2) (switch v (1 (print "one")) (2 (print "two")) (else (print "other")))))`,
	// module/export are resolved by ast.ResolveModules before HFIR; the
	// resolved program round-trips.
	"module":      `(cli_app (module m (export (defun f () (return "1")))) (print 1))`,
	"export":      `(cli_app (module m (export (defun f () (return "1")))) (print 1))`,
	"ver_ge":      `(do (print (ver_ge "1.2.3" "1.2.0")))`,
	"ver_gt":      `(do (print (ver_gt "1.2.3" "1.2.0")))`,
	"unix_now":    `(do (print (unix_now)))`,
	"http_body":   `(do (let (r (dict ("body" "x"))) (print (http_body r))))`,
	"http_status": `(do (let (r (dict ("status" 200))) (print (http_status r))))`,
	"req_query":   `(do (let (r (dict ("query" (dict ("x" "1"))))) (print (req_query r "x"))))`,
	"req_header":  `(do (let (r (dict ("headers" (dict ("x" "1"))))) (print (req_header r "x"))))`,
	"req_path":    `(do (let (r (dict ("path" "/a"))) (print (req_path r "x"))))`,
}

// adapterExemptForms are arity-table forms the adapter does not carry,
// because -compile-hfir-bc itself does not lower them. Each exemption is
// checked: its sample must be rejected by -compile-hfir-bc and by the adapter
// round trip, and no corpus program that compiles may use it, so a form that
// becomes lowerable fails this test until the adapter covers it.
var adapterExemptForms = map[string]struct{ reason, sample string }{
	// On the promote-blocker fence (docs/reference/lowered_hfir_prod_flip_criteria.md):
	// they have a bytecode opcode but -compile-hfir-bc does not lower them, and
	// widening that lowering changes the locked prod-flip criteria.
	"read_line":          {"promote-blocker fence; -compile-hfir-bc does not lower READ_LINE", `(do (print (read_line)))`},
	"read_stdin":         {"promote-blocker fence; -compile-hfir-bc does not lower READ_STDIN", `(do (print (read_stdin)))`},
	"sha256_hex":         {"promote-blocker fence; -compile-hfir-bc does not lower SHA256_HEX", `(do (print (sha256_hex "a")))`},
	"req_id":             {"promote-blocker fence; -compile-hfir-bc does not lower HTTP_REQ_ID", `(do (print (req_id 1)))`},
	"rand_hex":           {"promote-blocker fence; -compile-hfir-bc does not lower RAND_HEX", `(do (print (rand_hex 4)))`},
	"secure_equal":       {"promote-blocker fence; -compile-hfir-bc does not lower SECURE_EQUAL", `(do (print (secure_equal "a" "b")))`},
	"confidence":         {"promote-blocker fence; model call (network grant); not lowered by HFIR", `(do (print (confidence 1)))`},
	"http_request":       {"promote-blocker fence; not lowered by HFIR (fetch is the lowered network form)", `(do (print (http_request "http://127.0.0.1:9/" "GET")))`},
	"req_method":         {"promote-blocker fence; not lowered by HFIR (req_query/req_header/req_path are)", `(do (let (r (dict ("method" "GET"))) (print (req_method r))))`},
	"achieve":            {"opaque goal form (operands are stringified, not compiled); not lowered by HFIR", `(do (achieve "x" "y"))`},
	"spawn":              {"child-process authority form; not lowered by HFIR, kept out of model reach", `(do (spawn (lambda () (print 1))))`},
	"spawn_agent":        {"child-agent authority form; not lowered by HFIR, kept out of model reach", `(do (spawn_agent "a" "b"))`},
	"task":               {"agent task form; not lowered by HFIR", `(do (task "t" (print 1)))`},
	"grants":             {"only valid as the grant list of spawn_agent/spawn; rejected standalone", `(do (grants) (print 1))`},
	"llm_generate":       {"model call; not lowered by HFIR", `(do (print (llm_generate "hi")))`},
	"lazy_synthesize":    {"model synthesis form; not lowered by HFIR", `(cli_app (lazy_synthesize my_add (a b) "sum") (print (call my_add "1" "2")))`},
	"neural_circuit":     {"model circuit form; not lowered by HFIR", `(cli_app (let (r (neural_circuit (2 3) "multiply")) (print r)))`},
	"ephemeral_circuit":  {"model circuit form; not lowered by HFIR", `(cli_app (let (r (ephemeral_circuit (2 3) "multiply")) (print r)))`},
	"optimize_block":     {"optimizer directive; not lowered by HFIR", `(cli_app (optimize_block "latency" 10 (print 1)))`},
	"optimize_signature": {"optimizer directive; not lowered by HFIR", `(cli_app (optimize_signature p (metric "accuracy") (test "t") (candidate "a" "x") (print 1)))`},
	"schema_bridge":      {"schema bridge; not lowered by HFIR", `(cli_app (struct User (name string)) (let (s (schema_bridge User "{}")) (print s)))`},
	"db_connect":         {"database form; not lowered by HFIR", `(cli_app (db_connect db "sqlite3" "t.db"))`},
	"sql_query":          {"database form; not lowered by HFIR", `(cli_app (defun f (db) (sql_query db "SELECT 1")))`},
	"store_open":         {"store form; not lowered by HFIR", `(cli_app (store_open s "memory://x"))`},
	"store_get":          {"store form; not lowered by HFIR", `(cli_app (defun f (s) (print (store_get s "k")) (return "")))`},
	"store_put":          {"store form; not lowered by HFIR", `(cli_app (defun f (s) (store_put s "k" "v")))`},
	"store_delete":       {"store form; not lowered by HFIR", `(cli_app (defun f (s) (store_delete s "k")))`},
	"store_keys":         {"store form; not lowered by HFIR", `(cli_app (defun f (s) (print (store_keys s)) (return "")))`},
	"sget":               {"store sugar (store_get); not lowered by HFIR", `(cli_app (defun f (s) (print (sget s "k")) (return "")))`},
	"sput":               {"store sugar (store_put); not lowered by HFIR", `(cli_app (defun f (s) (sput s "k" "v")))`},
	"sinc":               {"store sugar (store_get/store_put); not lowered by HFIR", `(cli_app (defun f (s) (sinc s "k")))`},
	"http_server":        {"server root; routes are scoped by #165 first, not lowered by HFIR", `(http_server 8080 (route "/" (lambda (req) (res 200 "text/plain" "x"))))`},
	"res":                {"route response; only inside http_server routes", `(cli_app (res 200 "text/plain" "x"))`},
	"res_json":           {"route response; only inside http_server routes", `(cli_app (res_json 200 (dict ("a" 1))))`},
	"res_header":         {"route response; only inside http_server routes", `(cli_app (res_header "X-A" "1"))`},
	"http_res":           {"desugars to res; only inside http_server routes", `(cli_app (let (r (dict ("status" 200) ("body" "x"))) (http_res r)))`},
	"web_app":            {"JavaScript-target root; never compiled by HFIR", `(web_app (print 1))`},
	"wasm_app":           {"Wasm-target root; never compiled by HFIR", `(wasm_app (print 1))`},
}

func adapterCorpus(t *testing.T) []string {
	var files []string
	for _, root := range []string{"experiments", "tests", "examples", "apps"} {
		filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".howl") {
				files = append(files, p)
			}
			return nil
		})
	}
	sort.Strings(files)
	return files
}

func sourceHeads(src []byte) map[string]bool {
	heads := map[string]bool{}
	defer func() { recover() }()
	p := parser.NewParser(lexer.NewLexer(string(src)), "x.howl")
	// Binding lists, parameter lists and dict entries are structural: their
	// first element is a name or key, not a form.
	var walk func(n *ast.Node, structural bool)
	walk = func(n *ast.Node, structural bool) {
		if n == nil {
			return
		}
		head := ""
		if n.Type == "List" && len(n.Children) > 0 && n.Children[0].Type == "SYMBOL" {
			head = n.Children[0].Value
			if !structural {
				heads[head] = true
			}
		}
		for i, c := range n.Children {
			walk(c, (head == "let" || head == "try_let") && i == 1 || head == "defun" && i == 2 || head == "dict" && i > 0 || head == "catch" && i == 1)
		}
	}
	walk(p.ParseExpression(), false)
	return heads
}

// adapterRoundTrip reports whether -compile-hfir-bc accepts src, and if so
// whether the adapter round trip reproduces the artifact byte for byte.
func adapterRoundTrip(t *testing.T, bin, src string) (lowered bool, detail string) {
	dir := t.TempDir()
	direct, viaAdapter := filepath.Join(dir, "a.bin"), filepath.Join(dir, "b.bin")
	if err := exec.Command(bin, "-compile-hfir-bc", src, "-o", direct).Run(); err != nil {
		if out, err := exec.Command(bin, "-compile-hfir-bc", "-hfir-adapter-roundtrip", src, "-o", viaAdapter).CombinedOutput(); err == nil {
			return false, "adapter accepted a program -compile-hfir-bc rejects: " + string(out)
		}
		return false, ""
	}
	if out, err := exec.Command(bin, "-compile-hfir-bc", "-hfir-adapter-roundtrip", src, "-o", viaAdapter).CombinedOutput(); err != nil {
		return true, "adapter rejected: " + string(out)
	}
	a, _ := os.ReadFile(direct)
	b, _ := os.ReadFile(viaAdapter)
	if !bytes.Equal(a, b) {
		return true, "adapter artifact differs from -compile-hfir-bc"
	}
	return true, ""
}

func TestModelAdapterCoversArityTable(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles every corpus program twice")
	}
	bin := c1c2Binary(t)
	files := adapterCorpus(t)

	var mu sync.Mutex
	covered := map[string]bool{}
	loweredCount := 0
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for _, f := range files {
		f := f
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			lowered, detail := adapterRoundTrip(t, bin, f)
			if detail != "" {
				t.Errorf("%s: %s", f, detail)
			}
			if !lowered {
				return
			}
			src, _ := os.ReadFile(f)
			heads := sourceHeads(src)
			mu.Lock()
			loweredCount++
			for h := range heads {
				covered[h] = true
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if loweredCount < 380 {
		t.Fatalf("only %d corpus programs lower on -compile-hfir-bc; want the full benchmark and fixture corpus", loweredCount)
	}

	sampleDir := t.TempDir()
	for form, sample := range adapterFormSamples {
		src := filepath.Join(sampleDir, form+".howl")
		os.WriteFile(src, []byte(sample), 0o644)
		lowered, detail := adapterRoundTrip(t, bin, src)
		if !lowered || detail != "" {
			t.Errorf("sample for %s: lowered=%v %s", form, lowered, detail)
			continue
		}
		if !sourceHeads([]byte(sample))[form] {
			t.Errorf("sample for %s does not use %s", form, form)
		}
		covered[form] = true
	}

	table := construct.ArityTable()
	for form, ex := range adapterExemptForms {
		if _, ok := table[form]; !ok {
			t.Errorf("exempt form %s is not in the arity table", form)
		}
		if covered[form] {
			t.Errorf("exempt form %s (%s) is now lowered by -compile-hfir-bc and carried by the adapter; drop the exemption", form, ex.reason)
		}
		if ex.sample == "" {
			t.Errorf("exempt form %s has no sample", form)
			continue
		}
		if !sourceHeads([]byte(ex.sample))[form] {
			t.Errorf("exempt sample for %s does not use %s", form, form)
		}
		{
			src := filepath.Join(sampleDir, "exempt_"+form+".howl")
			os.WriteFile(src, []byte(ex.sample), 0o644)
			if lowered, detail := adapterRoundTrip(t, bin, src); lowered || detail != "" {
				t.Errorf("exempt form %s: -compile-hfir-bc lowered=%v %s; the adapter must cover it", form, lowered, detail)
			}
		}
	}
	for form := range adapterFormSamples {
		if _, ok := table[form]; !ok {
			t.Errorf("sample form %s is not in the arity table", form)
		}
	}
	var missing []string
	for form := range table {
		if !covered[form] {
			if _, exempt := adapterExemptForms[form]; !exempt {
				missing = append(missing, form)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("arity-table forms with no model-adapter coverage and no exemption: %s", strings.Join(missing, " "))
	}
	t.Logf("adapter coverage: %d corpus programs round-trip; %d of %d arity forms covered, %d exempt", loweredCount, len(table)-len(adapterExemptForms)-len(missing), len(table), len(adapterExemptForms))
}
