package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestWasmGCCorpusParity runs every cli_app fixture of the differential
// corpus (tests/*.howl, less the manifest's exemptions) and every cli_app
// example (examples/*.howl) on the bytecode VM, in Node and, when
// wasmtime is installed, as a WASI command in wasmtime
// and as WebAssembly. A fixture the backend builds must match the VM; one it
// does not support yet is counted, with the construct it stops at, so the log
// shows what the backend still needs.
func TestWasmGCCorpusParity(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs every corpus fixture")
	}
	root, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	var exempt map[string]string
	data, err := os.ReadFile(filepath.Join(root, "tools", "difftest", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &exempt); err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, dir := range []string{"tests", "examples"} {
		matches, err := filepath.Glob(filepath.Join(root, dir, "*.howl"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	construct := regexp.MustCompile(`WASM_UNSUPPORTED: "([^"]*)"`)
	stoppedAt := map[string]int{}
	engines := map[string]bool{"node": true}
	passed, unsupported := 0, 0
	for _, file := range files {
		base := filepath.Base(file)
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := exempt[base]; ok || !strings.Contains(string(src), "(cli_app") {
			continue
		}
		report, err := VerifyParity(file, nil, "", []Target{TargetWasmGC, TargetWasmGCWASI})
		if err != nil {
			t.Fatal(err)
		}
		res := report.TargetResults[TargetWasmGC]
		if wasi := report.TargetResults[TargetWasmGCWASI]; wasi.Status != StatusBackendUnsupported {
			engines["wasmtime"] = true
		}
		switch {
		case len(report.Discrepancies) > 0:
			t.Errorf("%s: %s", base, strings.Join(report.Discrepancies, "\n"))
		case res.Status == StatusBackendUnsupported:
			if strings.Contains(res.ErrorMessage, "runtime not available") {
				t.Skip("node not installed")
			}
			unsupported++
			what := res.ErrorMessage
			if m := construct.FindStringSubmatch(what); m != nil {
				what = m[1]
			}
			stoppedAt[what]++
		default:
			passed++
		}
	}
	var gaps []string
	for what, n := range stoppedAt {
		gaps = append(gaps, what+" ("+strconv.Itoa(n)+")")
	}
	sort.Strings(gaps)
	var ran []string
	for e := range engines {
		ran = append(ran, e)
	}
	sort.Strings(ran)
	t.Logf("wasmgc corpus (%s): %d match the VM, %d not supported yet; stopped at: %s", strings.Join(ran, ", "), passed, unsupported, strings.Join(gaps, ", "))
}
