package capability

import (
	"os"
	"path/filepath"
	"testing"
)

// IsRootEscape matches the standard library's message; this pins it against
// the running toolchain so a wording change fails here instead of silently
// turning denials into ordinary I/O errors.
func TestRootEscapeErrorIsRecognized(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "out")); err != nil {
		t.Fatal(err)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if _, err := r.Open("out/f"); !IsRootEscape(err) {
		t.Fatalf("expected a recognized root-escape error, got %v", err)
	}
	if _, err := r.Open("missing"); IsRootEscape(err) {
		t.Fatalf("a missing file must not be reported as an escape: %v", err)
	}
}

func TestOpenScopeMissingRootIsIOErrorNotDenial(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	g, err := ParseGrant("filesystem:read=" + missing)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Grants{g}.ScopedReadFile(filepath.Join(missing, "f"))
	if err == nil || !os.IsNotExist(err) {
		t.Fatalf("want a not-exist error, got %v", err)
	}
}

func TestScopedMkdirCreatesMissingGrantedRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "newroot")
	g, _ := ParseGrant("filesystem:write=" + root)
	if err := (Grants{g}).ScopedMkdirAll(filepath.Join(root, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(filepath.Join(root, "a", "b")); err != nil || !st.IsDir() {
		t.Fatalf("directory not created: %v", err)
	}
}
