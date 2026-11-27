package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --replay drives a terminal_app from a recorded event file (improvements.md
// #118): one howlframe.terminal_replay/v1 record per handled event, effects
// recorded and never executed, so no grant is needed.

type replayRecord struct {
	Schema  string           `json:"schema"`
	Seq     int              `json:"seq"`
	Source  string           `json:"source"`
	Event   map[string]any   `json:"event"`
	OK      bool             `json:"ok"`
	State   map[string]any   `json:"state"`
	View    []any            `json:"view"`
	Prompt  string           `json:"prompt"`
	Effects []map[string]any `json:"effects"`
	Error   map[string]any   `json:"error"`
	Quit    *int             `json:"quit"`
}

func runReplay(t *testing.T, source string, events []string, flags ...string) (int, []replayRecord, string) {
	t.Helper()
	binary := buildHowlFrameBinaryForTest(t)
	file := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(file, []byte(strings.Join(events, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runSource(t, binary, source, append([]string{"--replay", file}, flags...)...)
	var records []replayRecord
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var rec replayRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("not a replay record: %q: %v", line, err)
		}
		records = append(records, rec)
	}
	return code, records, errOut
}

func TestTerminalReplayRecordsEffectsWithoutRunningThem(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	sh := canonicalProgram(t, "sh")
	start := `{"type":"start","tty":false,"cols":80,"rows":24}`
	launch, _ := json.Marshal(map[string]string{"type": "line", "text": `[{"kind":"exec_start","id":"p","argv":["` + sh + `","-c","touch ` + marker + `"]},{"kind":"nope"}]`})
	exit := `{"type":"process_exit","id":"p","exit_code":0,"stdout":"hi\n","stderr":"","error":null}`
	quit := `{"type":"line","text":"[{\"kind\":\"quit\",\"code\":4}]"}`
	code, records, errOut := runReplay(t, terminalEchoApp, []string{start, string(launch), exit, quit, `{"type":"line","text":"[]"}`})
	if code != 4 {
		t.Fatalf("exit %d, want the quit code 4; stderr %q", code, errOut)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("replay executed an exec_start effect: %v", err)
	}
	if len(records) != 5 {
		t.Fatalf("records = %d, want 5 (the line after quit is never handled): %+v", len(records), records)
	}
	sources := []string{}
	for i, rec := range records {
		if rec.Schema != "howlframe.terminal_replay/v1" || rec.Seq != i+1 || !rec.OK {
			t.Fatalf("record %d = %+v", i, rec)
		}
		sources = append(sources, rec.Source)
	}
	if strings.Join(sources, ",") != "replay,replay,runtime,replay,replay" {
		t.Fatalf("sources = %v", sources)
	}
	if got := records[1].Effects; len(got) != 2 || got[0]["kind"] != "exec_start" || got[0]["id"] != "p" {
		t.Fatalf("recorded effects = %v", got)
	}
	if ev := records[2].Event; ev["type"] != "effect_failed" || ev["kind"] != "nope" {
		t.Fatalf("an unknown kind is refused as live: %v", ev)
	}
	if records[3].Event["stdout"] != "hi\n" || records[3].State["seen"] != float64(4) {
		t.Fatalf("file events reach the handler verbatim: %+v", records[3])
	}
	if records[4].Quit == nil || *records[4].Quit != 4 {
		t.Fatalf("quit not recorded: %+v", records[4])
	}
}

func TestTerminalReplayKeepsStateWhenTheHandlerFails(t *testing.T) {
	events := []string{`{"type":"start"}`, `{"type":"line","text":"[]"}`, `{"type":"line","text":"spin"}`, `{"type":"line","text":"[]"}`}
	code, records, errOut := runReplay(t, terminalEchoApp, events, "--max-event-instructions", "2000")
	if code != 0 || len(records) != 5 {
		t.Fatalf("exit %d, %d records; stderr %q", code, len(records), errOut)
	}
	failed, notice := records[2], records[3]
	if failed.OK || failed.Error["code"] != "LIMIT_EXCEEDED" || failed.State["seen"] != float64(2) {
		t.Fatalf("failed event = %+v", failed)
	}
	if notice.Source != "runtime" || notice.Event["type"] != "handler_failed" || notice.Event["event_type"] != "line" {
		t.Fatalf("handler_failed = %+v", notice)
	}
	if records[4].State["seen"] != float64(4) {
		t.Fatalf("the session continues from the kept state: %+v", records[4])
	}
}

func TestTerminalReplayTerminateStopsLikeLive(t *testing.T) {
	code, records, _ := runReplay(t, terminalEchoApp, []string{`{"type":"start"}`, `{"type":"terminate"}`, `{"type":"line","text":"[]"}`})
	if code != 143 || len(records) != 2 {
		t.Fatalf("exit %d, %d records", code, len(records))
	}
}

func TestTerminalReplayRecordsStyledEntriesAsData(t *testing.T) {
	code, records, errOut := runReplay(t, terminalStyledApp, []string{`{"type":"start"}`})
	if code != 0 || len(records) != 1 {
		t.Fatalf("exit %d, %d records; stderr %q", code, len(records), errOut)
	}
	view := records[0].View
	first, _ := view[0].(map[string]any)
	if len(view) != 5 || first["text"] != "FAILED" || first["style"] != "error" || view[4] != "plain" {
		t.Fatalf("view = %v", view)
	}
}

// A handler usually builds its view and effects with list literals, which
// the VM holds as persistent lists rather than []any. The records must carry
// them all the same (the apps above build theirs with parse_json).
const terminalListLiteralApp = `(cli_app
  (defun on_event (state event)
    (type_hints (state dict) (event dict) (return dict))
    (return (dict ("state" state)
                  ("view" (list (dict ("text" "first line") ("style" "plain")) (dict ("text" "styled") ("style" "ok"))))
                  ("prompt" "> ")
                  ("effects" (list (dict ("kind" "timer") ("id" "t") ("ms" 5))
                                   (dict ("kind" "quit") ("code" 3)))))))
  (exit (terminal_app "on_event" (dict))))`

func TestTerminalReplayRecordsListLiteralViewsAndEffects(t *testing.T) {
	code, records, errOut := runReplay(t, terminalListLiteralApp, []string{`{"type":"start"}`})
	if code != 3 || len(records) != 1 {
		t.Fatalf("exit %d, %d records; stderr %q", code, len(records), errOut)
	}
	rec := records[0]
	if len(rec.View) != 2 {
		t.Fatalf("view = %v", rec.View)
	}
	first, _ := rec.View[0].(map[string]any)
	styled, _ := rec.View[1].(map[string]any)
	if first["text"] != "first line" || styled["style"] != "ok" {
		t.Fatalf("view = %v", rec.View)
	}
	if len(rec.Effects) != 2 || rec.Effects[0]["kind"] != "timer" || rec.Effects[1]["kind"] != "quit" {
		t.Fatalf("effects = %v", rec.Effects)
	}
	if rec.Quit == nil || *rec.Quit != 3 {
		t.Fatalf("quit not recorded: %+v", rec)
	}
}

func TestTerminalReplayRefusesMalformedEvents(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	file := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(file, []byte("{\"type\":\"start\"}\n[1,2]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := runSource(t, binary, terminalEchoApp, "--replay", file)
	if code == 0 || !strings.Contains(errOut, "line 2 is not an event object") {
		t.Fatalf("exit %d stderr %q", code, errOut)
	}
	code, _, errOut = runSource(t, binary, terminalEchoApp, "--replay", filepath.Join(t.TempDir(), "absent.jsonl"))
	if code == 0 || !strings.Contains(errOut, "IO_ERROR") {
		t.Fatalf("missing file: exit %d stderr %q", code, errOut)
	}
}
