package agent

import (
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
)

// SessionSchema versions authenticated local recovery reports.
const SessionSchema = "howlframe.agent.session/v1"
const sessionSchema = SessionSchema
const sessionMaxFile = 16 << 20
const sessionMaxTotal = 256 << 20
const sessionMaxFiles = 20000

type sessionInput struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type sessionManifest struct {
	IndexSHA256       string         `json:"index_sha256"`
	CheckArgv         []string       `json:"check_argv"`
	CheckExitCode     int            `json:"check_exit_code"`
	BuildArgvTemplate []string       `json:"build_argv_template"`
	ChildEnvironment  []string       `json:"child_environment"`
	Schema            string         `json:"schema"`
	Task              string         `json:"task"`
	Root              string         `json:"root"`
	Head              string         `json:"head"`
	Source            string         `json:"source"`
	Compiler          sessionInput   `json:"compiler"`
	CompilerRevision  string         `json:"compiler_revision"`
	EnvironmentSHA256 string         `json:"environment_sha256"`
	Inputs            []sessionInput `json:"inputs"`
	Claims            []string       `json:"claims"`
	Next              []string       `json:"next_actions"`
	Observed          sessionInput   `json:"observed"`
	Producer          string         `json:"producer"`
}
type sessionResponse struct {
	Schema      string           `json:"schema"`
	Command     string           `json:"command"`
	OK          bool             `json:"ok"`
	Verified    bool             `json:"verified"`
	Code        string           `json:"code,omitempty"`
	Message     string           `json:"message,omitempty"`
	Manifest    *sessionManifest `json:"manifest,omitempty"`
	Limitations []string         `json:"limitations"`
}
type sessionOptions struct {
	task, root, bundle, state, source string
	claims, next                      []string
	verified                          bool
}
type sessionChecker func(executable, root, source string) ([]byte, error)

func runSession(args []string, executable string, out io.Writer, checker sessionChecker) int {
	r := sessionResponse{Schema: sessionSchema, Command: args[0], Limitations: []string{
		"verified means authenticated compiler-check evidence for the recorded inputs, not task completion or behavioral test success",
		"all tracked and nonignored untracked regular files are covered; ignored, external, Git metadata and installed dependencies are not covered",
		"sources containing include/use substrings are rejected, even inside comments, strings or identifiers; ignored or external dependency coverage is unsupported",
		"child environment is fixed; only ambient HOWLFRAME_ALLOW_CAPS is conservatively fingerprinted; other ambient variables are not inputs",
		"local trust protects against bundle editing without the separate key; same-user compromise and input changes restored during a check are outside this guarantee",
		"claims and next actions are owner-supplied text, never observed results; no efficiency gain is measured",
	}}
	fail := func(code string, e error) int {
		r.OK = false
		r.Verified = false
		r.Code = code
		r.Message = e.Error()
		json.NewEncoder(out).Encode(r)
		return 1
	}
	o, e := parseSession(args)
	if e != nil {
		return fail("AGENT_SESSION_ARGUMENT", e)
	}
	o.root, e = secureDirectory(o.root)
	if e != nil {
		return fail("AGENT_SESSION_INPUT", e)
	}
	// Evidence and trust must be outside the input tree and independent of each other.
	o.bundle, e = secureLocation(o.bundle)
	if e != nil {
		return fail("AGENT_SESSION_INPUT", e)
	}
	o.state, e = secureLocation(o.state)
	if e != nil {
		return fail("AGENT_SESSION_TRUST", e)
	}
	if contained(o.root, o.bundle) || contained(o.root, o.state) || contained(o.bundle, o.state) || contained(o.state, o.bundle) {
		return fail("AGENT_SESSION_ARGUMENT", errors.New("bundle and state must be separate directories outside the project"))
	}
	if args[0] == "snapshot" {
		if e = os.Mkdir(o.bundle, 0700); e != nil {
			return fail("AGENT_SESSION_PUBLICATION", e)
		}
		// A failed/interrupted run deliberately leaves an incomplete fresh bundle.
		key, e := sessionKey(o.state, true)
		if e != nil {
			return fail("AGENT_SESSION_TRUST", e)
		}
		before, e := captureSession(o, executable)
		if e != nil {
			return fail("AGENT_SESSION_INPUT", e)
		}
		evidence, e := checker(executable, o.root, filepath.Join(o.root, o.source))
		if e != nil {
			return fail("AGENT_SESSION_CHECK", e)
		}
		if len(evidence) > sessionMaxFile {
			return fail("AGENT_SESSION_CHECK", errors.New("check evidence exceeds size limit"))
		}
		var checked struct {
			OK bool `json:"ok"`
		}
		if json.Unmarshal(evidence, &checked) != nil || !checked.OK {
			return fail("AGENT_SESSION_CHECK", errors.New("compiler check did not pass"))
		}
		after, e := captureSession(o, executable)
		if e != nil {
			return fail("AGENT_SESSION_INPUT", e)
		}
		if !reflect.DeepEqual(before, after) {
			return fail("AGENT_SESSION_STALE", errors.New("inputs changed during compilation"))
		}
		before.Observed = sessionInput{"check.json", digest(evidence)}
		manifest, e := json.Marshal(before)
		if e != nil {
			return fail("AGENT_SESSION_PUBLICATION", e)
		}
		if e = atomicSessionFile(o.bundle, "check.json", evidence); e != nil {
			return fail("AGENT_SESSION_PUBLICATION", e)
		}
		if e = atomicSessionFile(o.bundle, "manifest.json", manifest); e != nil {
			return fail("AGENT_SESSION_PUBLICATION", e)
		}
		auth, _ := json.Marshal(struct {
			Schema string `json:"schema"`
			MAC    string `json:"mac"`
		}{sessionSchema, sessionMAC(key, manifest)})
		if e = atomicSessionFile(o.bundle, "authentication.json", auth); e != nil {
			return fail("AGENT_SESSION_PUBLICATION", e)
		}
		r.Manifest = &before
	} else {
		key, e := sessionKey(o.state, false)
		if e != nil {
			return fail("AGENT_SESSION_TRUST", e)
		}
		manifest, e := readSessionFile(o.bundle, "manifest.json", sessionMaxFile)
		if e != nil {
			return fail("AGENT_SESSION_EVIDENCE", e)
		}
		auth, e := readSessionFile(o.bundle, "authentication.json", 1024)
		if e != nil {
			return fail("AGENT_SESSION_EVIDENCE", e)
		}
		var a struct {
			Schema string `json:"schema"`
			MAC    string `json:"mac"`
		}
		if strictSessionJSON(auth, &a) != nil || a.Schema != sessionSchema || !hmac.Equal([]byte(a.MAC), []byte(sessionMAC(key, manifest))) {
			return fail("AGENT_SESSION_AUTHENTICATION", errors.New("manifest authentication failed"))
		}
		var m sessionManifest
		if e = strictSessionJSON(manifest, &m); e != nil || m.Schema != sessionSchema {
			return fail("AGENT_SESSION_EVIDENCE", errors.New("invalid manifest schema"))
		}
		if m.Task != o.task || m.Root != o.root {
			return fail("AGENT_SESSION_STALE", errors.New("task or project does not match"))
		}
		if m.Observed.Path != "check.json" {
			return fail("AGENT_SESSION_EVIDENCE", errors.New("unexpected evidence path"))
		}
		evidence, e := readSessionFile(o.bundle, m.Observed.Path, sessionMaxFile)
		if e != nil || digest(evidence) != m.Observed.SHA256 {
			return fail("AGENT_SESSION_EVIDENCE", errors.New("missing or changed check evidence"))
		}
		o.source = m.Source
		o.claims = m.Claims
		o.next = m.Next
		current, e := captureSession(o, executable)
		if e != nil {
			return fail("AGENT_SESSION_INPUT", e)
		}
		current.Observed = m.Observed
		if !reflect.DeepEqual(current, m) {
			return fail("AGENT_SESSION_STALE", errors.New("source, compiler, environment, dependency, test or configuration input changed; take a new snapshot"))
		}
		r.Manifest = &m
	}
	r.OK = true
	r.Verified = true
	json.NewEncoder(out).Encode(r)
	return 0
}
func parseSession(args []string) (sessionOptions, error) {
	o := sessionOptions{claims: []string{}, next: []string{}}
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--verified" {
			if seen[a] {
				return o, errors.New("duplicate --verified")
			}
			seen[a] = true
			o.verified = true
			continue
		}
		if a != "--task" && a != "--root" && a != "--bundle" && a != "--state-dir" && a != "--source" && a != "--claim" && a != "--next" {
			return o, fmt.Errorf("unknown option %q", a)
		}
		i++
		if i >= len(args) || args[i] == "" || len(args[i]) > 4096 {
			return o, errors.New("missing or oversized option value")
		}
		if seen[a] && a != "--claim" && a != "--next" {
			return o, fmt.Errorf("duplicate %s", a)
		}
		seen[a] = true
		switch a {
		case "--task":
			o.task = args[i]
		case "--root":
			o.root = args[i]
		case "--bundle":
			o.bundle = args[i]
		case "--state-dir":
			o.state = args[i]
		case "--source":
			o.source = args[i]
		case "--claim":
			o.claims = append(o.claims, args[i])
		case "--next":
			o.next = append(o.next, args[i])
		}
	}
	if o.task == "" || o.root == "" || o.bundle == "" || o.state == "" || len(o.claims)+len(o.next) > 100 {
		return o, errors.New("require --task ID --root PROJECT --bundle DIRECTORY --state-dir TRUST")
	}
	if args[0] == "snapshot" {
		if o.verified || !filepath.IsLocal(o.source) {
			return o, errors.New("snapshot requires a project-relative --source and no --verified")
		}
	} else if o.source != "" || len(o.claims)+len(o.next) > 0 || o.verified != (args[0] == "status") {
		return o, errors.New("status requires --verified; context accepts neither --verified nor snapshot-only options")
	}
	return o, nil
}
func compileSessionCheck(executable, root, source string) ([]byte, error) {
	ds, requirements, notes := checkInEnvironment(executable, source, root, sessionEnvironment())
	r := Response{Schema: Schema, Command: "check", Target: "bytecode", Coverage: "known", OK: len(ds) == 0, ActualGrants: []string{}, Limitations: []string{"compilation only; source never executed; child environment fixed to PATH=/usr/bin:/bin, LANG=C, LC_ALL=C"}, Constructs: []Entry{}, Examples: []Example{}, Diagnostics: ds, CompilerNotes: notes, Requirements: requirements}
	return json.Marshal(r)
}

func sessionEnvironment() []string { return []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"} }
