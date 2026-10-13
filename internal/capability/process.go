package capability

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ExecGrantPrefix scopes the process capability to one exact program and an
// argument pattern. A coarse "process" grant authorizes spawn and spawn_agent
// only: it never authorizes exec. exec runs only when an exec grant matches the
// requested argv exactly, so a grant of "process" with no allow-list fails closed.
//
// Syntax: process:exec=<absolute program path>[ <pattern>...]
//
//	literal  matches exactly one argument equal to the literal
//	*        matches exactly one argument with any value
//	**       (last token only) matches zero or more remaining arguments
//
// No shell is involved and no PATH lookup happens: the requested program must
// be the same absolute, canonical (symlink-free) path that the grant names.
const ExecGrantPrefix = "process:exec="

// ExecDenied is the typed detail of a refused exec or attenuation request.
type ExecDenied struct {
	Reason string
}

func (e *ExecDenied) Error() string { return e.Reason }

// parseExecSpec validates and normalizes an exec grant body.
func parseExecSpec(spec string) (string, []string, error) {
	fields := strings.Fields(spec)
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("process:exec grant needs a program path")
	}
	program := fields[0]
	if !filepath.IsAbs(program) || filepath.Clean(program) != program {
		return "", nil, fmt.Errorf("process:exec program must be an absolute, clean path: %q", program)
	}
	pats := fields[1:]
	for i, p := range pats {
		if p == "**" && i != len(pats)-1 {
			return "", nil, fmt.Errorf("process:exec pattern ** must be the last token")
		}
	}
	return program, pats, nil
}

func parseExecGrant(raw string) (Capability, bool, error) {
	if !strings.HasPrefix(raw, ExecGrantPrefix) {
		return None, false, nil
	}
	program, pats, err := parseExecSpec(strings.TrimPrefix(raw, ExecGrantPrefix))
	if err != nil {
		return None, true, err
	}
	return Capability(ExecGrantPrefix + strings.Join(append([]string{program}, pats...), " ")), true, nil
}

func argvMatches(pats, args []string) bool {
	for i, p := range pats {
		if p == "**" {
			return true
		}
		if i >= len(args) {
			return false
		}
		if !slotMatches(p, args[i]) {
			return false
		}
	}
	return len(args) == len(pats)
}

// slotMatches matches one argument. "*" refuses empty arguments and anything
// that starts with "-", so a wildcard slot cannot smuggle in a flag; flags
// must be granted as literals. A leading backslash escapes a token, so `\*`
// matches a literal "*" and `\-x` a literal "-x".
func slotMatches(p, arg string) bool {
	if p == "*" {
		return arg != "" && !strings.HasPrefix(arg, "-")
	}
	if strings.HasPrefix(p, "\\") {
		return arg == p[1:]
	}
	return p == arg
}

// EnvGrantPrefix names one host environment variable passed through to every
// exec child. exec children otherwise start with an empty environment.
const EnvGrantPrefix = "process:env="

// CodeLoadingEnvNames and CodeLoadingEnvPrefixes are the single source of
// the process:env= refusal list (#166, #172, #177). Generated Go and
// JavaScript render these same tables, so the three targets cannot drift.
// Matching is case-insensitive. Each entry steers code loading, startup
// scripts, config or trust lookup, or program search in an exec child.
var CodeLoadingEnvNames = []string{
	// shells
	"BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "PS4", "PROMPT_COMMAND", "IFS", "CDPATH", "GLOBIGNORE", "ZDOTDIR",
	// program search, home and config directories
	"PATH", "HOME", "XDG_CONFIG_HOME", "XDG_CONFIG_DIRS", "XDG_DATA_HOME", "XDG_DATA_DIRS",
	// Python
	"PYTHONPATH", "PYTHONHOME", "PYTHONSTARTUP", "PYTHONUSERBASE", "PYTHONINSPECT", "PYTHONWARNINGS", "PYTHONBREAKPOINT", "PYTHONEXECUTABLE",
	// Node
	"NODE_OPTIONS", "NODE_PATH", "NODE_EXTRA_CA_CERTS", "NODE_TLS_REJECT_UNAUTHORIZED", "NODE_REPL_EXTERNAL_MODULE",
	// Perl and Ruby
	"PERL5LIB", "PERLLIB", "PERL5OPT", "PERL5DB", "RUBYOPT", "RUBYLIB",
	// Go and Java
	"GOFLAGS", "GODEBUG", "GOROOT", "GOPATH", "GOTOOLCHAIN", "GOENV", "JAVA_TOOL_OPTIONS", "_JAVA_OPTIONS", "JDK_JAVA_OPTIONS", "CLASSPATH",
	// libc locale, iconv and resolver lookups
	"GCONV_PATH", "LOCPATH", "NLSPATH", "HOSTALIASES", "RESOLV_HOST_CONF",
	// TLS trust stores
	"SSL_CERT_FILE", "SSL_CERT_DIR", "CURL_CA_BUNDLE", "REQUESTS_CA_BUNDLE",
}

// CodeLoadingEnvPrefixes refuses whole families: the dynamic loader (LD_*,
// DYLD_*), git hooks/config/exec path (GIT_*), exported bash functions
// (BASH_FUNC_*) and Lua module paths and init (LUA_*).
var CodeLoadingEnvPrefixes = []string{"LD_", "DYLD_", "GIT_", "BASH_FUNC_", "LUA_"}

// DynamicLoaderEnv reports variables in the shared code-loading refusal
// list. Passing one to an exec child would let it load arbitrary code into
// an allow-listed program, so process:env= refuses them.
func DynamicLoaderEnv(name string) bool {
	u := strings.ToUpper(name)
	for _, n := range CodeLoadingEnvNames {
		if u == n {
			return true
		}
	}
	for _, p := range CodeLoadingEnvPrefixes {
		if strings.HasPrefix(u, p) {
			return true
		}
	}
	return false
}

func validEnvName(name string) bool {
	if name == "" || DynamicLoaderEnv(name) {
		return false
	}
	for i, r := range name {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// ExecEnv returns the environment for an exec child: only variables named by
// process:env= grants and present in the host environment. It is never nil,
// so os/exec does not fall back to inheriting the host environment.
func (g Grants) ExecEnv(lookup func(string) (string, bool)) []string {
	env := []string{}
	seen := map[string]bool{}
	for _, grant := range g {
		name, ok := strings.CutPrefix(string(grant), EnvGrantPrefix)
		if !ok || !validEnvName(name) || seen[name] {
			continue
		}
		seen[name] = true
		if v, ok := lookup(name); ok {
			env = append(env, name+"="+v)
		}
	}
	return env
}

// HasCoarse reports whether the exact coarse grant c is present.
func (g Grants) HasCoarse(c Capability) bool {
	for _, grant := range g {
		if grant == c {
			return true
		}
	}
	return false
}

// AuthorizeExec checks a requested argv against the exec allow-list. It
// returns the program path to execute (identical to the requested one) or a
// typed denial. Denial reasons never echo the program or its arguments,
// which can carry secrets; the receipt records only the program base name.
// Relative or bare names, non-canonical paths and paths that are
// or traverse a symlink are refused, so PATH lookup and link swaps cannot
// change which binary runs.
// The second result is the matched allow-list entry, for receipts.
func (g Grants) AuthorizeExec(program string, args []string) (string, string, error) {
	if !filepath.IsAbs(program) {
		return "", "", &ExecDenied{"process: exec program is not an absolute path (no PATH lookup)"}
	}
	if filepath.Clean(program) != program {
		return "", "", &ExecDenied{"process: exec program is not a clean path"}
	}
	resolved, err := filepath.EvalSymlinks(program)
	if err != nil {
		return "", "", &ExecDenied{"process: exec program cannot be resolved"}
	}
	if resolved != program {
		return "", "", &ExecDenied{"process: exec program is or traverses a symlink; grant and call its canonical path"}
	}
	for _, grant := range g {
		parsed, ok, err := parseExecGrant(string(grant))
		if !ok || err != nil {
			continue
		}
		gp, pats, _ := parseExecSpec(strings.TrimPrefix(string(parsed), ExecGrantPrefix))
		if gp == program && argvMatches(pats, args) {
			return program, string(parsed), nil
		}
	}
	return "", "", &ExecDenied{"process: exec argv is not in the allow-list"}
}

// Attenuate returns the child grant set for an explicitly requested subset of
// the parent's grants. Nothing is inherited implicitly: an empty request yields
// no grants. Every requested grant must be covered by the parent, otherwise
// the whole request is refused.
// InheritFilesystem is the attenuation request for the parent's own
// filesystem grants.
const InheritFilesystem = "inherit:filesystem"

func Attenuate(parent Grants, requested []string) (Grants, error) {
	child := Grants{}
	for _, raw := range requested {
		// "inherit:filesystem" hands the child exactly the filesystem grants the
		// parent holds (scoped or coarse), whatever their run-time roots are.
		// It can only narrow: the child never gets more than the parent.
		if raw == InheritFilesystem {
			for _, pg := range parent {
				s := string(pg)
				if pg == Filesystem || strings.HasPrefix(s, "filesystem:read=") || strings.HasPrefix(s, "filesystem:write=") {
					child = append(child, pg)
				}
			}
			continue
		}
		g, err := ParseGrant(raw)
		if err != nil {
			return nil, &ExecDenied{"child grant is invalid"}
		}
		if !parent.covers(g) {
			// Never echo the requested grant: it can name paths or programs.
			return nil, &ExecDenied{"child requested a grant the parent does not hold"}
		}
		child = append(child, g)
	}
	return child, nil
}

func (g Grants) covers(c Capability) bool {
	raw := string(c)
	for _, mode := range []string{"read", "write"} {
		prefix := "filesystem:" + mode + "="
		if strings.HasPrefix(raw, prefix) {
			if g.HasCoarse(Filesystem) {
				return true
			}
			root := strings.TrimPrefix(raw, prefix)
			for _, pg := range g {
				if strings.HasPrefix(string(pg), prefix) && contained(strings.TrimPrefix(string(pg), prefix), root) {
					return true
				}
			}
			return false
		}
	}
	// Coarse names and exec grants must be held verbatim (normalized).
	for _, pg := range g {
		if parsed, err := ParseGrant(string(pg)); err == nil && parsed == c {
			return true
		}
	}
	return false
}
