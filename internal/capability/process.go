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
		if p != "*" && p != args[i] {
			return false
		}
	}
	return len(args) == len(pats)
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
func (g Grants) AuthorizeExec(program string, args []string) (string, error) {
	if !filepath.IsAbs(program) {
		return "", &ExecDenied{"process: exec program is not an absolute path (no PATH lookup)"}
	}
	if filepath.Clean(program) != program {
		return "", &ExecDenied{"process: exec program is not a clean path"}
	}
	resolved, err := filepath.EvalSymlinks(program)
	if err != nil {
		return "", &ExecDenied{"process: exec program cannot be resolved"}
	}
	if resolved != program {
		return "", &ExecDenied{"process: exec program is or traverses a symlink; grant and call its canonical path"}
	}
	for _, grant := range g {
		parsed, ok, err := parseExecGrant(string(grant))
		if !ok || err != nil {
			continue
		}
		gp, pats, _ := parseExecSpec(strings.TrimPrefix(string(parsed), ExecGrantPrefix))
		if gp == program && argvMatches(pats, args) {
			return program, nil
		}
	}
	return "", &ExecDenied{"process: exec argv is not in the allow-list"}
}

// Attenuate returns the child grant set for an explicitly requested subset of
// the parent's grants. Nothing is inherited implicitly: an empty request yields
// no grants. Every requested grant must be covered by the parent, otherwise
// the whole request is refused.
func Attenuate(parent Grants, requested []string) (Grants, error) {
	child := Grants{}
	for _, raw := range requested {
		g, err := ParseGrant(raw)
		if err != nil {
			return nil, &ExecDenied{fmt.Sprintf("spawn_agent: invalid grant %q", raw)}
		}
		if !parent.covers(g) {
			return nil, &ExecDenied{fmt.Sprintf("spawn_agent: child requested %q, which the parent does not hold", string(g))}
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
