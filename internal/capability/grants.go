package capability

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseGrant validates a grant and anchors filesystem roots to the current cwd.
// The returned value can be passed through the existing []Capability APIs.
func ParseGrant(raw string) (Capability, error) {
	for _, c := range All() {
		if raw == string(c) {
			return c, nil
		}
	}
	for _, mode := range []string{"read", "write"} {
		prefix := "filesystem:" + mode + "="
		if strings.HasPrefix(raw, prefix) && len(raw) > len(prefix) {
			root, err := filepath.Abs(strings.TrimPrefix(raw, prefix))
			if err == nil {
				return Capability(prefix + filepath.Clean(root)), nil
			}
		}
	}
	return None, fmt.Errorf("unknown capability: %q", raw)
}

// Grants retains coarse capability compatibility while enforcing path scopes.
type Grants []Capability

func (g Grants) Has(c Capability) bool {
	for _, grant := range g {
		parsed, err := ParseGrant(string(grant))
		if err == nil && (parsed == c || c == Filesystem && strings.HasPrefix(string(parsed), "filesystem:")) {
			return true
		}
	}
	return false
}

// HasUnrestrictedFilesystem reports whether the legacy coarse alias is present.
func (g Grants) HasUnrestrictedFilesystem() bool {
	for _, grant := range g {
		if grant == Filesystem {
			return true
		}
	}
	return false
}

func (g Grants) AllowsRead(path string) bool  { return g.allows(path, "read") }
func (g Grants) AllowsWrite(path string) bool { return g.allows(path, "write") }

func contained(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// resolveAncestor follows symlinks even for a target that has not been created.
// Errors other than a missing component fail closed (including dangling links).
func resolveAncestor(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	if _, e := os.Lstat(path); e == nil {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = resolveAncestor(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func (g Grants) allows(path, mode string) bool {
	if g.HasUnrestrictedFilesystem() {
		return true
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	target = filepath.Clean(target)
	resolvedTarget, err := resolveAncestor(target)
	if err != nil {
		return false
	}
	prefix := "filesystem:" + mode + "="
	for _, grant := range g {
		// Direct API callers must use ParseGrant to anchor relative roots.
		raw := string(grant)
		if !strings.HasPrefix(raw, prefix) {
			continue
		}
		root := strings.TrimPrefix(raw, prefix)
		if root == "" || !filepath.IsAbs(root) || !contained(filepath.Clean(root), target) {
			continue
		}
		resolvedRoot, err := resolveAncestor(filepath.Clean(root))
		if err == nil && contained(resolvedRoot, resolvedTarget) {
			return true
		}
	}
	return false
}
