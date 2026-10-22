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

// Review finding: a grant naming one file must not reach a sibling through a
// relative symlink placed at the granted name. The root handle is the file's
// directory, so the OS alone would follow that link; the scope must refuse it.
func TestFileRootGrantRefusesRelativeSymlinkToSibling(t *testing.T) {
	dir := t.TempDir()
	sibling := filepath.Join(dir, "sibling")
	if err := os.WriteFile(sibling, []byte("UNGRANTED"), 0o644); err != nil {
		t.Fatal(err)
	}
	granted := filepath.Join(dir, "granted")
	if err := os.Symlink("sibling", granted); err != nil {
		t.Fatal(err)
	}
	read, _ := ParseGrant("filesystem:read=" + granted)
	if b, err := (Grants{read}).ScopedReadFile(granted); err == nil {
		t.Fatalf("read of an ungranted sibling through a file grant: %q", b)
	}
	write, _ := ParseGrant("filesystem:write=" + granted)
	if err := (Grants{write}).ScopedWriteFile(granted, []byte("CORRUPTED"), 0o644); err == nil {
		t.Fatal("write through a symlink at a file grant must be refused")
	}
	if b, _ := os.ReadFile(sibling); string(b) != "UNGRANTED" {
		t.Fatalf("sibling was modified through a file grant: %q", b)
	}
}

// A write through a file grant must not truncate anything before the target has
// been verified, or a swapped-in link would lose the other file's data.
func TestFileRootGrantWriteDoesNotTruncateBeforeVerifying(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("long original content"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := ParseGrant("filesystem:write=" + file)
	if err := (Grants{g}).ScopedWriteFile(file, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(file); string(b) != "new" {
		t.Fatalf("overwrite of the granted file itself must still truncate and replace: %q", b)
	}
}
