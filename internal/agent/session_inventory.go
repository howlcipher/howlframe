package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func gitSession(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	argv := append([]string{"-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-C", root}, args...)
	git, err := sessionGitExecutable()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, git, argv...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0"}
	var stdout, stderr boundedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	e := cmd.Run()
	if e != nil {
		return nil, fmt.Errorf("git inventory failed: %w", e)
	}
	if stdout.truncated || stderr.truncated {
		return nil, errors.New("git inventory exceeds 1 MiB")
	}
	return stdout.Bytes(), nil
}
func captureSession(o sessionOptions, exe string) (sessionManifest, error) {
	m := sessionManifest{Schema: sessionSchema, Task: o.task, Root: o.root, Source: o.source, Claims: o.claims, Next: o.next, Producer: "current compiler: agent snapshot; built-in bytecode check only", Inputs: []sessionInput{}, CompilerRevision: "unknown"}
	top, e := gitSession(o.root, "rev-parse", "--show-toplevel")
	if e != nil {
		return m, e
	}
	if strings.TrimSpace(string(top)) != o.root {
		return m, errors.New("root must be the Git worktree top level")
	}
	head, e := gitSession(o.root, "rev-parse", "--verify", "HEAD")
	if e != nil {
		return m, e
	}
	m.Head = strings.TrimSpace(string(head))
	index, e := gitSession(o.root, "ls-files", "--stage", "-z")
	if e != nil {
		return m, e
	}
	m.IndexSHA256 = digest(index)
	m.CheckArgv = []string{"agent", "check", "--json", filepath.Join(o.root, o.source)}
	m.BuildArgvTemplate = []string{exe, "build", "--target", "bytecode", "-o", "<temporary>/checked.hfbc", "--", filepath.Join(o.root, o.source)}
	m.ChildEnvironment = sessionEnvironment()
	list, e := gitSession(o.root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if e != nil {
		return m, e
	}
	names := strings.Split(strings.TrimSuffix(string(list), "\x00"), "\x00")
	sort.Strings(names)
	if len(names) > sessionMaxFiles {
		return m, errors.New("inventory exceeds 20000 files")
	}
	total := 0
	sourceFound := false
	previous := ""
	for _, name := range names {
		if name == previous {
			continue
		}
		previous = name
		if !utf8.ValidString(name) {
			return m, errors.New("inventory filenames must be valid UTF-8")
		}
		if !filepath.IsLocal(name) || name == ".git" || strings.HasPrefix(name, ".git/") {
			return m, errors.New("unsafe inventory path")
		}
		b, e := readSessionFile(o.root, name, sessionMaxFile)
		if os.IsNotExist(e) {
			m.Inputs = append(m.Inputs, sessionInput{name, "deleted"})
			continue
		}
		if e != nil {
			return m, e
		}
		total += len(b)
		if total > sessionMaxTotal {
			return m, errors.New("inventory exceeds 256 MiB")
		}
		m.Inputs = append(m.Inputs, sessionInput{name, digest(b)})
		if name == o.source {
			if bytes.Contains(b, []byte("include")) || bytes.Contains(b, []byte("use")) {
				return m, errors.New("include/use dependency coverage unsupported; even comments, strings and identifiers containing those substrings are rejected")
			}
			sourceFound = true
		}
	}
	if !sourceFound {
		return m, errors.New("source must exist in tracked or nonignored untracked inventory")
	}
	exe, e = secureLocation(exe)
	if e != nil {
		return m, e
	}
	binary, e := readSessionFile(filepath.Dir(exe), filepath.Base(exe), 128<<20)
	if e != nil {
		return m, e
	}
	m.Compiler = sessionInput{exe, digest(binary)}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				m.CompilerRevision = s.Value
			}
		}
	}
	value, present := os.LookupEnv("HOWLFRAME_ALLOW_CAPS")
	env, _ := json.Marshal(struct {
		Fixed   []string
		Name    string
		Present bool
		Value   []byte
	}{sessionEnvironment(), "HOWLFRAME_ALLOW_CAPS", present, []byte(value)})
	m.EnvironmentSHA256 = digest(env)
	return m, nil
}

// Search the declared recipe explicitly: exec.Command("git") would consult the
// parent environment before cmd.Env is installed.
func sessionGitExecutable() (string, error) {
	for _, path := range []string{"/usr/bin/git", "/bin/git"} {
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return path, nil
		}
	}
	return "", errors.New("Git unavailable in trusted /usr/bin:/bin search path")
}
