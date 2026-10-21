package capability

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrFilesystemDenied reports that a filesystem operation is not covered by any
// granted scope, or that it tried to leave the granted scope while running.
var ErrFilesystemDenied = errors.New("filesystem: path is outside every granted scope")

// FileSystem is the small surface the VM needs for governed file I/O. For a
// scoped grant it is an *os.Root opened on the granted directory, and every
// name is relative to that root: the operating system then refuses to follow a
// path, including a symlink, out of the root at the moment the file is opened.
// That closes the window between "is this path allowed?" and "open this path".
type FileSystem interface {
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	MkdirAll(name string, perm os.FileMode) error
	Rename(oldname, newname string) error
	Remove(name string) error
}

// Scope is one authorized filesystem target: a FileSystem plus the name to use
// with it.
type Scope struct {
	FS     FileSystem
	Name   string
	closer func() error
}

// Close releases the root handle held by a scoped target.
func (s *Scope) Close() error {
	if s == nil || s.closer == nil {
		return nil
	}
	return s.closer()
}

// ReadFile reads the whole target.
func (s *Scope) ReadFile() ([]byte, error) {
	f, err := s.FS.OpenFile(s.Name, os.O_RDONLY, 0)
	if err != nil {
		return nil, s.classify(err)
	}
	defer f.Close()
	return io.ReadAll(f)
}

// WriteFile creates or truncates the target, like os.WriteFile.
func (s *Scope) WriteFile(data []byte, perm os.FileMode) error {
	f, err := s.FS.OpenFile(s.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return s.classify(err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// MkdirAll creates the target directory and any missing parents, like
// os.MkdirAll.
func (s *Scope) MkdirAll(perm os.FileMode) error {
	return s.classify(s.FS.MkdirAll(s.Name, perm))
}

// Sibling returns the name of a file next to the target, for temporary files
// that must stay in the same directory (and so under the same root).
func (s *Scope) Sibling(base string) string {
	return filepath.Join(filepath.Dir(s.Name), base)
}

// Dir is the name of the directory holding the target.
func (s *Scope) Dir() string { return filepath.Dir(s.Name) }

// Classify turns an operating-system refusal to leave the root into
// ErrFilesystemDenied and passes every other error through unchanged.
func (s *Scope) Classify(err error) error { return s.classify(err) }

func (s *Scope) classify(err error) error {
	if err != nil && IsRootEscape(err) {
		return fmt.Errorf("%w: %v", ErrFilesystemDenied, err)
	}
	return err
}

// IsRootEscape reports whether err is the error os.Root returns when a path
// tries to leave the root. The standard library does not export a sentinel, so
// the message is matched; TestRootEscapeErrorIsRecognized pins it against the
// running toolchain.
func IsRootEscape(err error) bool {
	return err != nil && strings.Contains(err.Error(), "path escapes from parent")
}

// osFS is the unrestricted filesystem used for the legacy coarse "filesystem"
// grant, which authorizes every path by design.
type osFS struct{}

func (osFS) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}
func (osFS) MkdirAll(name string, perm os.FileMode) error { return os.MkdirAll(name, perm) }
func (osFS) Rename(oldname, newname string) error         { return os.Rename(oldname, newname) }
func (osFS) Remove(name string) error                     { return os.Remove(name) }

// OpenScope authorizes path for reading or writing and returns a target bound
// to the granted root. The existing lexical and symlink-resolved containment
// check (allows) still decides whether the operation is permitted; the returned
// Scope additionally pins the I/O itself to that root, so a path component
// replaced between the check and the open cannot redirect the operation.
//
// It returns ErrFilesystemDenied when no grant covers path.
func (g Grants) OpenScope(path string, write bool) (*Scope, error) {
	return g.openScope(path, write, false)
}

// openScope is OpenScope; createRoot lets a directory-creating operation
// create a granted root that does not exist yet, as os.MkdirAll always did.
func (g Grants) openScope(path string, write, createRoot bool) (*Scope, error) {
	mode := "read"
	if write {
		mode = "write"
	}
	if g.HasUnrestrictedFilesystem() {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFilesystemDenied, err)
		}
		return &Scope{FS: osFS{}, Name: abs}, nil
	}
	root, ok := g.matchRoot(path, mode)
	if !ok {
		return nil, ErrFilesystemDenied
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFilesystemDenied, err)
	}
	rel, err := filepath.Rel(root, filepath.Clean(target))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFilesystemDenied, err)
	}
	handle, err := os.OpenRoot(root)
	if err != nil && createRoot && errors.Is(err, fs.ErrNotExist) {
		if mkErr := os.MkdirAll(root, 0o755); mkErr == nil {
			handle, err = os.OpenRoot(root)
		}
	}
	if err != nil {
		// Authorization was already decided by matchRoot, so a root that cannot
		// be opened (missing, not a directory, unreadable) only means the target
		// cannot be reached. Report the ordinary I/O error, not a denial, and
		// perform no operation.
		return nil, err
	}
	return &Scope{FS: handle, Name: rel, closer: handle.Close}, nil
}

// ScopedReadFile reads path within the granted scope.
func (g Grants) ScopedReadFile(path string) ([]byte, error) {
	sc, err := g.OpenScope(path, false)
	if err != nil {
		return nil, err
	}
	defer sc.Close()
	return sc.ReadFile()
}

// ScopedWriteFile writes path within the granted scope.
func (g Grants) ScopedWriteFile(path string, data []byte, perm os.FileMode) error {
	sc, err := g.OpenScope(path, true)
	if err != nil {
		return err
	}
	defer sc.Close()
	return sc.WriteFile(data, perm)
}

// ScopedMkdirAll creates path (and parents) within the granted scope.
func (g Grants) ScopedMkdirAll(path string, perm os.FileMode) error {
	sc, err := g.openScope(path, true, true)
	if err != nil {
		return err
	}
	defer sc.Close()
	return sc.MkdirAll(perm)
}
