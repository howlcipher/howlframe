package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// EvidenceDesk's ledger module is consumed outside EvidenceDesk (HowlBoard's
// ledger import). parse must keep the frozen 262144-byte ceiling, and
// parse_within must apply the caller's ceiling with otherwise identical
// validation.
func TestEvidenceDeskLedgerParseWithin(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	module, err := os.ReadFile(filepath.Join("milestone3", "evidencedesk", "app", "ledger.howl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.howl"), module, 0o600); err != nil {
		t.Fatal(err)
	}
	program := `(cli_app
  (use "ledger.howl" as ledger)
  (try_let (raw (read_file (cli_args 0)))
    (catch e (exit 3))
    (let (text (bytes_to_string raw) limit (to_int (cli_args 1)) ceiling (cli_args 2))
      (if (= ceiling "")
        (print (encode_json (call ledger/parse text limit)))
        (print (encode_json (call ledger/parse_within text limit (to_int ceiling))))))))`
	source, artifact := filepath.Join(dir, "probe.howl"), filepath.Join(dir, "probe.hfbc")
	if err := os.WriteFile(source, []byte(program), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(binary, "build", source, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("compile: %v: %s", err, out)
	}
	line := func(id string, pad int) string {
		return `{"schema":"ai.evidence_entry/v1","entry_id":"` + id + `","task_id":"T","agent_id":"a","action":"x","pad":"` +
			strings.Repeat("p", pad) + `"}` + "\n"
	}
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	type result struct {
		Events []map[string]any `json:"events"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Line    int    `json:"line"`
		} `json:"error"`
	}
	run := func(path, limit, ceiling string) result {
		t.Helper()
		args := []string{"run", "--allow-caps", "filesystem:read=" + dir, "--max-instructions", "50000000", artifact, path, limit}
		if ceiling != "" {
			args = append(args, ceiling)
		}
		out, err := exec.Command(binary, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("run %v: %v: %s", args, err, out)
		}
		var r result
		if err := json.Unmarshal(out, &r); err != nil {
			t.Fatalf("decode %q: %v", out, err)
		}
		return r
	}

	// 300 KB of valid records: over the frozen ceiling, under a caller ceiling.
	var big strings.Builder
	records := 0
	for ; big.Len() <= 300000; records++ {
		big.WriteString(line(fmt.Sprintf("e%05d", records), 900))
	}
	bigPath := write("big.jsonl", big.String())
	if r := run(bigPath, "1000", ""); r.Error == nil || r.Error.Code != "payload_too_large" || r.Error.Message != "Ledger exceeds 262144 bytes" {
		t.Fatalf("parse lost its frozen ceiling: %+v", r.Error)
	}
	if r := run(bigPath, "1000", "4194304"); r.Error != nil || len(r.Events) != records {
		t.Fatalf("parse_within rejected input under its ceiling: %+v (%d events)", r.Error, len(r.Events))
	}
	small := write("small.jsonl", line("a", 10)+line("b", 10))
	if r := run(small, "10", "100"); r.Error == nil || r.Error.Message != "Ledger exceeds 100 bytes" {
		t.Fatalf("parse_within ignored a smaller ceiling: %+v", r.Error)
	}
	// Validation other than the ceiling is unchanged: duplicate entry_id at line 2.
	dup := write("dup.jsonl", line("a", 1)+line("a", 1))
	for _, ceiling := range []string{"", "4194304"} {
		if r := run(dup, "10", ceiling); r.Error == nil || r.Error.Message != "Duplicate entry_id" || r.Error.Line != 2 {
			t.Fatalf("ceiling %q: duplicate not rejected at line 2: %+v", ceiling, r.Error)
		}
	}
}
