package verification

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestReportFinalizeStatusAndDeterminism(t *testing.T) {
	makeReport := func() Report {
		r := Report{
			Target:  "bytecode",
			Profile: "governed",
			Versions: Versions{
				Compiler:          "0.3.0",
				HFBCFormat:        "2",
				ConstructRegistry: 42,
				HFIRVerifier:      "v1",
			},
			HFIRBlockingCodes: []string{"HFIR_INVALID_REF", "PROFILE_FORBIDDEN_CONSTRUCT", "HFIR_INVALID_REF"},
			Checks: []Check{
				{Name: "parser", Status: StatusPass, Summary: "parsed cleanly"},
				{Name: "hfir_gate", Status: StatusPass, Version: "v1", Codes: []string{"B", "A", "B"}, Summary: "no blocking codes"},
				{Name: "artifact_validation", Status: StatusSkipped, Summary: "not built"},
			},
			Limitations: []string{"Static verification report only; does not execute the program, prove runtime correctness, or grant execution authority.", "Extra caveat"},
		}
		r.Finalize()
		return r
	}

	first, second := makeReport(), makeReport()
	b1, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("marshal first: %v", err)
	}
	b2, err := json.Marshal(second)
	if err != nil {
		t.Fatalf("marshal second: %v", err)
	}
	if string(b1) != string(b2) {
		t.Fatalf("non-deterministic output:\n%s\nvs\n%s", b1, b2)
	}
	if first.Schema != ReportSchema || !first.Passed || !first.Partial {
		t.Fatalf("unexpected report header/status: %+v", first)
	}
	wantBlocking := []string{"HFIR_INVALID_REF", "PROFILE_FORBIDDEN_CONSTRUCT"}
	if !slices.Equal(first.HFIRBlockingCodes, wantBlocking) {
		t.Fatalf("HFIRBlockingCodes = %v, want %v", first.HFIRBlockingCodes, wantBlocking)
	}
	if !slices.Equal(first.Checks[1].Codes, []string{"A", "B"}) {
		t.Fatalf("check codes = %v, want [A B]", first.Checks[1].Codes)
	}
	if len(first.Limitations) != 3 {
		t.Fatalf("expected deduplicated 3 limitations, got %v", first.Limitations)
	}
}

func TestReportFailAndEmptyNeverPass(t *testing.T) {
	empty := Report{}
	empty.Finalize()
	if empty.Passed || !empty.Partial || empty.Checks == nil || empty.HFIRBlockingCodes == nil {
		t.Fatalf("empty report must not pass and must initialize slices: %+v", empty)
	}

	failed := Report{
		Checks: []Check{
			{Name: "parser", Status: StatusPass, Summary: "ok"},
			{Name: "checker", Status: StatusFail, Summary: "type mismatch"},
			{Name: "hfir_gate", Status: StatusSkipped, Summary: "skipped after checker fail"},
		},
	}
	failed.Finalize()
	if failed.Passed || !failed.Partial {
		t.Fatalf("failed report must set Passed=false, Partial=true: %+v", failed)
	}

	unknown := Report{
		Checks: []Check{{Name: "mystery", Status: "ok", Summary: "invalid status"}},
	}
	unknown.Finalize()
	if unknown.Passed {
		t.Fatalf("unknown check status must fail closed: %+v", unknown)
	}

	if code := ExtractErrorCode("ARITY_ERROR: wrong args"); code != "ARITY_ERROR" {
		t.Fatalf("ExtractErrorCode = %q, want ARITY_ERROR", code)
	}
	if code := ExtractErrorCode("plain message"); code != "" {
		t.Fatalf("ExtractErrorCode unexpected code %q", code)
	}
	filled := AppendRemainingSkipped([]Check{{Name: "parser", Status: StatusFail, Summary: "bad syntax"}}, StandardPipelineChecks, "skipped after prior failure")
	if len(filled) != len(StandardPipelineChecks) || filled[1].Name != "construct_registry" || filled[1].Status != StatusSkipped {
		t.Fatalf("AppendRemainingSkipped = %+v", filled)
	}
}
