package testutil

import (
	"path/filepath"
	"strings"
	"testing"
)

// CanonicalExecFixture binds trusted positive-fixture text and exact grants to
// installed executables. Some hosts expose coreutils through symlinks, which
// the process gate correctly rejects. Explicit symlink-denial fixtures must
// retain their original paths and must not use this helper.
func CanonicalExecFixture(t *testing.T, text string) string {
	t.Helper()
	for _, name := range []string{"printf", "touch", "true", "env", "echo"} {
		original := "/usr/bin/" + name
		if !strings.Contains(text, original) {
			continue
		}
		canonical, err := filepath.EvalSymlinks(original)
		if err != nil {
			t.Fatalf("required exec fixture %s is unavailable: %v", original, err)
		}
		text = strings.ReplaceAll(text, original, canonical)
	}
	return text
}
