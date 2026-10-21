package capability

import (
	"errors"
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

// A grant may name one file instead of a directory (the projector's launcher
// grants read access to its input file). The grant must keep working for exactly
// that file, and must still refuse to follow it out of its directory.
func TestFileRootGrantReadsAndWritesExactlyThatFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "in.jsonl")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	read, _ := ParseGrant("filesystem:read=" + file)
	got, err := Grants{read}.ScopedReadFile(file)
	if err != nil || string(got) != "data" {
		t.Fatalf("file-root read: %q %v", got, err)
	}
	// A sibling is not covered by a grant naming one file.
	if _, err := (Grants{read}).ScopedReadFile(filepath.Join(dir, "other")); err == nil || !errorsIsDenied(err) {
		t.Fatalf("a sibling of a granted file must be denied, got %v", err)
	}
	// Writing a granted file that does not exist yet creates it, as before.
	newFile := filepath.Join(dir, "new.json")
	write, _ := ParseGrant("filesystem:write=" + newFile)
	if err := (Grants{write}).ScopedWriteFile(newFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("file-root write of a missing file: %v", err)
	}
	if b, _ := os.ReadFile(newFile); string(b) != "x" {
		t.Fatalf("file not written: %q", b)
	}
}

func TestFileRootGrantDoesNotFollowSymlinkOutOfItsDirectory(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("OUTSIDE"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "in.jsonl")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	g, _ := ParseGrant("filesystem:read=" + link)
	if b, err := (Grants{g}).ScopedReadFile(link); err == nil {
		t.Fatalf("a granted file that is a symlink leaving its directory must fail closed, read %q", b)
	}
}

func errorsIsDenied(err error) bool { return errors.Is(err, ErrFilesystemDenied) }
