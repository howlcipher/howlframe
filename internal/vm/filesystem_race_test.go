package vm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/lexer"
	"github.com/howlcipher/howlframe/internal/parser"
)

// Regression for audit finding HF-01: a scoped filesystem grant must hold even
// when a path component inside the granted root is replaced while the program
// runs. Everything here lives in t.TempDir(); the "outside" directory holds
// only harmless marker files. A scoped operation must never read, write or
// create anything in it.
//
// root/d is always a symlink. A goroutine flips its target between a real
// directory inside root and the outside directory, so a check that resolves the
// path and then opens it again can pass on one target and open the other.

const raceMarker = "OUTSIDE-MARKER"

type raceFixture struct {
	root, outside string
}

func newRaceFixture(t *testing.T) raceFixture {
	t.Helper()
	root, outside := t.TempDir(), t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	for dir, content := range map[string]string{real: "inside", outside: raceMarker} {
		if err := os.WriteFile(filepath.Join(dir, "f"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("real", filepath.Join(root, "d")); err != nil {
		t.Fatal(err)
	}
	return raceFixture{root: root, outside: outside}
}

// flip alternates root/d between the in-root and outside targets until stop is
// closed. The replacement is a symlink rename, so d never disappears.
func (f raceFixture) flip(stop <-chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		targets := []string{"real", f.outside}
		tmp := filepath.Join(f.root, "d.tmp")
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = os.Remove(tmp)
			if os.Symlink(targets[i%2], tmp) == nil {
				_ = os.Rename(tmp, filepath.Join(f.root, "d"))
			}
		}
	}()
}

// runRace runs body (a HowlFrame loop) while the symlink flips. It returns the
// program's stdout.
func (f raceFixture) runRace(t *testing.T, grants []capability.Capability, body string) string {
	t.Helper()
	node := parser.NewParser(lexer.NewLexer("(cli_app "+body+")"), "race.howl").ParseExpression()
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = 200_000_000
	policy.Limits.MaxMemoryBytes = 1 << 30
	stop := make(chan struct{})
	var wg sync.WaitGroup
	f.flip(stop, &wg)
	var out, stderr bytes.Buffer
	ev := RunBytecodeWithEvidence(bytecode.CompileToBytecode(node), nil, policy, grants, strings.NewReader(""), &out, &stderr, 0)
	close(stop)
	wg.Wait()
	if ev.RuntimeFailure != nil {
		t.Fatalf("program failed: %+v", ev.RuntimeFailure)
	}
	return strings.TrimSpace(out.String())
}

func raceGrants(t *testing.T, root string, modes ...string) []capability.Capability {
	t.Helper()
	var grants []capability.Capability
	for _, mode := range modes {
		raw := "filesystem:" + mode + "=" + root
		if mode == "database" {
			raw = "database"
		}
		g, err := capability.ParseGrant(raw)
		if err != nil {
			t.Fatal(err)
		}
		grants = append(grants, g)
	}
	return grants
}

const raceIterations = 4000

func TestScopedReadNeverEscapesWhileSymlinkChanges(t *testing.T) {
	f := newRaceFixture(t)
	body := fmt.Sprintf(`(let (n 0 hits 0)
	  (do (while (< n %d)
	    (do (try_let (b (read_file %q))
	          (catch e (set hits hits))
	          (if (= (bytes_to_string b) %q) (set hits (+ hits 1)) (set hits hits)))
	        (set n (+ n 1))))
	  (print hits)))`, raceIterations, filepath.Join(f.root, "d", "f"), raceMarker)
	if got := f.runRace(t, raceGrants(t, f.root, "read"), body); got != "0" {
		t.Fatalf("scoped read returned content from outside the granted root %s times", got)
	}
}

func TestScopedWriteNeverEscapesWhileSymlinkChanges(t *testing.T) {
	f := newRaceFixture(t)
	body := fmt.Sprintf(`(let (n 0)
	  (do (while (< n %d)
	    (do (try_let (r (do (write_file %q "changed") 1)) (catch e (set n n)) (set n n))
	        (set n (+ n 1))))
	  (print "done")))`, raceIterations, filepath.Join(f.root, "d", "f"))
	f.runRace(t, raceGrants(t, f.root, "write"), body)
	data, err := os.ReadFile(filepath.Join(f.outside, "f"))
	if err != nil || string(data) != raceMarker {
		t.Fatalf("scoped write modified a file outside the granted root: %q %v", data, err)
	}
}

func TestScopedMkdirNeverEscapesWhileSymlinkChanges(t *testing.T) {
	f := newRaceFixture(t)
	body := fmt.Sprintf(`(let (n 0)
	  (do (while (< n %d)
	    (do (try_let (r (do (mkdir %q) 1)) (catch e (set n n)) (set n n))
	        (set n (+ n 1))))
	  (print "done")))`, raceIterations, filepath.Join(f.root, "d", "made", "nested"))
	f.runRace(t, raceGrants(t, f.root, "write"), body)
	if _, err := os.Stat(filepath.Join(f.outside, "made")); !os.IsNotExist(err) {
		t.Fatalf("scoped mkdir created a directory outside the granted root: %v", err)
	}
}

func TestFileStoreNeverEscapesWhileSymlinkChanges(t *testing.T) {
	f := newRaceFixture(t)
	store := filepath.Join(f.root, "d", "s.json")
	body := fmt.Sprintf(`(let (n 0)
	  (do (while (< n %d)
	    (do (try_let (r (do (store_open s "file://%s") (store_put s "k" (dict ("a" n))) 1))
	          (catch e (set n n)) (set n n))
	        (set n (+ n 1))))
	  (print "done")))`, 300, store)
	f.runRace(t, raceGrants(t, f.root, "read", "write", "database"), body)
	entries, _ := os.ReadDir(f.outside)
	for _, e := range entries {
		if e.Name() != "f" {
			t.Fatalf("file-backed store wrote %q outside the granted root", e.Name())
		}
	}
}

// A bounded smoke check that the harness itself is live: with the flipper
// pointing at the in-root target only, the read must succeed.
func TestRaceHarnessReadsInsideRoot(t *testing.T) {
	f := newRaceFixture(t)
	node := parser.NewParser(lexer.NewLexer(fmt.Sprintf("(cli_app (print (bytes_to_string (read_file %q))))", filepath.Join(f.root, "d", "f"))), "x.howl").ParseExpression()
	var out, stderr bytes.Buffer
	ev := RunBytecodeWithEvidence(bytecode.CompileToBytecode(node), nil, DefaultExecutionPolicy(), raceGrants(t, f.root, "read"), strings.NewReader(""), &out, &stderr, 0)
	if ev.RuntimeFailure != nil || strings.TrimSpace(out.String()) != "inside" {
		t.Fatalf("harness sanity failed: %q %+v", out.String(), ev.RuntimeFailure)
	}
}

// Documented limitation: a symlink inside the granted root is followed only
// when it is relative and stays inside the root. An absolute link target is
// refused even if it points back into the root, because the root handle cannot
// vouch for an absolute path. Failing closed is intentional; see
// docs/remediation/HOWLFRAME_SECURITY_REMEDIATION.md.
func TestAbsoluteInRootSymlinkFailsClosed(t *testing.T) {
	f := newRaceFixture(t)
	abs := filepath.Join(f.root, "abs")
	if err := os.Symlink(filepath.Join(f.root, "real"), abs); err != nil {
		t.Fatal(err)
	}
	node := parser.NewParser(lexer.NewLexer(fmt.Sprintf("(cli_app (print (bytes_to_string (read_file %q))))", filepath.Join(abs, "f"))), "x.howl").ParseExpression()
	var out, stderr bytes.Buffer
	ev := RunBytecodeWithEvidence(bytecode.CompileToBytecode(node), nil, DefaultExecutionPolicy(), raceGrants(t, f.root, "read"), strings.NewReader(""), &out, &stderr, 0)
	if ev.RuntimeFailure == nil || ev.RuntimeFailure.Code != "CAPABILITY_DENIED" {
		t.Fatalf("absolute in-root symlink must fail closed with CAPABILITY_DENIED: %q %+v", out.String(), ev.RuntimeFailure)
	}
}

// The same guarantee for a grant that names one file: while the granted file
// flips between a regular file and a symlink to a file outside its directory, a
// read must never return the outside content.
func TestFileRootGrantNeverEscapesWhileSymlinkChanges(t *testing.T) {
	f := newRaceFixture(t)
	granted := filepath.Join(f.root, "grantedfile")
	if err := os.WriteFile(granted, []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "sibling"), []byte(raceMarker), 0o644); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tmp := filepath.Join(f.root, "g.tmp")
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_ = os.Remove(tmp)
			switch i % 3 {
			case 0: // absolute link to a file outside the directory
				if os.Symlink(filepath.Join(f.outside, "f"), tmp) == nil {
					_ = os.Rename(tmp, granted)
				}
			case 1: // relative link to an ungranted sibling in the same directory
				if os.Symlink("sibling", tmp) == nil {
					_ = os.Rename(tmp, granted)
				}
			default:
				if os.WriteFile(tmp, []byte("inside"), 0o644) == nil {
					_ = os.Rename(tmp, granted)
				}
			}
		}
	}()
	defer func() { close(stop); wg.Wait() }()
	grant, err := capability.ParseGrant("filesystem:read=" + granted)
	if err != nil {
		t.Fatal(err)
	}
	node := parser.NewParser(lexer.NewLexer(fmt.Sprintf(`(cli_app (let (n 0 hits 0)
	  (do (while (< n %d)
	    (do (try_let (b (read_file %q))
	          (catch e (set hits hits))
	          (if (= (bytes_to_string b) %q) (set hits (+ hits 1)) (set hits hits)))
	        (set n (+ n 1))))
	  (print hits))))`, raceIterations, granted, raceMarker)), "r.howl").ParseExpression()
	policy := DefaultExecutionPolicy()
	policy.Limits.MaxInstructions = 200_000_000
	policy.Limits.MaxMemoryBytes = 1 << 30
	var out, stderr bytes.Buffer
	ev := RunBytecodeWithEvidence(bytecode.CompileToBytecode(node), nil, policy, []capability.Capability{grant}, strings.NewReader(""), &out, &stderr, 0)
	if ev.RuntimeFailure != nil {
		t.Fatalf("program failed: %+v", ev.RuntimeFailure)
	}
	if got := strings.TrimSpace(out.String()); got != "0" {
		t.Fatalf("a file grant returned content from outside the granted file %s times", got)
	}
}
