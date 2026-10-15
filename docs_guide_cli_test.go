package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Milestone 2 gate H, documentation: docs/guides/http_api.md must satisfy the
// frozen rule in milestone2/ticketdesk/dx/docs_rule.md. The test compiles every
// code block, so a guide example can never rot.
func TestHTTPAPIGuideRule(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	raw, err := os.ReadFile("docs/guides/http_api.md")
	if err != nil {
		t.Fatalf("rule 1: docs/guides/http_api.md must exist: %v", err)
	}
	guide := string(raw)

	for _, h := range []string{"Routing", "JSON bodies", "Authentication", "Validation", "Errors", "Limits", "Logging", "Running standalone"} {
		if !regexp.MustCompile(`(?m)^## ` + regexp.QuoteMeta(h) + `$`).MatchString(guide) {
			t.Errorf("rule 2: missing heading %q", h)
		}
	}

	fence := regexp.MustCompile("(?s)```([^\n]*)\n(.*?)```")
	dir := t.TempDir()
	checks, hasPathParam, hasReqBody := 0, false, false
	for i, m := range fence.FindAllStringSubmatch(guide, -1) {
		info, body := strings.TrimSpace(m[1]), m[2]
		switch info {
		case "text", "bash", "json":
			continue
		case "lisp check", "lisp fragment":
		default:
			t.Errorf("rule 3: block %d has info string %q", i, info)
			continue
		}
		src := body
		if info == "lisp fragment" {
			trimmed := strings.TrimSpace(body)
			if !strings.HasPrefix(trimmed, "(http_server") && !strings.HasPrefix(trimmed, "(cli_app") && !strings.HasPrefix(trimmed, "(web_app") && !strings.HasPrefix(trimmed, "(wasm_app") {
				src = "(cli_app\n" + body + "\n)"
			}
		}
		file := filepath.Join(dir, "block.howl")
		os.WriteFile(file, []byte(src), 0o600)
		if out, err := exec.Command(binary, "check", file).CombinedOutput(); err != nil {
			t.Errorf("rule 3: block %d (%s) fails check: %s", i, info, out)
		}
		if info == "lisp check" {
			checks++
			if out, err := exec.Command(binary, "build", file, "-o", filepath.Join(dir, "block.hfbc")).CombinedOutput(); err != nil {
				t.Errorf("rule 3: block %d fails build: %s", i, out)
			}
			if strings.Contains(body, "(http_server") && strings.Contains(body, "{id}") {
				hasPathParam = true
			}
			if strings.Contains(body, "(parse_json Any req.body)") {
				hasReqBody = true
			}
		}
	}
	if checks < 3 || !hasPathParam || !hasReqBody {
		t.Errorf("rule 4: need >=3 'lisp check' blocks incl. an http_server with a path parameter (%v) and parse_json over req.body (%v); have %d", hasPathParam, hasReqBody, checks)
	}

	help, _ := exec.Command(binary, "run", "--help").CombinedOutput()
	cli, _ := os.ReadFile("docs/cli.md")
	for _, flagName := range regexp.MustCompile(`--[a-z][a-z-]+`).FindAllString(guide, -1) {
		if !strings.Contains(string(help), flagName) && !strings.Contains(string(cli), flagName) {
			t.Errorf("rule 5: flag %s is in neither `howlframe run --help` nor docs/cli.md", flagName)
		}
	}
	reqs, err := os.ReadFile("milestone2/ticketdesk/requirements/REQUIREMENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	grants := regexp.MustCompile("(?m)^network,database,environment,filesystem:read=<DIR>,filesystem:write=<DIR>$").FindString(string(reqs))
	if grants == "" || !strings.Contains(guide, "--allow-caps "+grants) {
		t.Errorf("rule 5: the guide must run the example with exactly the R2 grants %q", grants)
	}

	if !strings.Contains(guide, "milestone2/ticketdesk/") {
		t.Errorf("rule 6: the guide must link to milestone2/ticketdesk/")
	}
	if regexp.MustCompile(`td_[0-9a-f]{64}|adm-[0-9a-f]{32}`).MatchString(guide) {
		t.Errorf("rule 6: the guide contains something that looks like a real token")
	}
}
