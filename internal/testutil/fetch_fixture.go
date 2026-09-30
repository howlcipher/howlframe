package testutil

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// LockABIFetchFixture serializes tests that bind the loopback address in
// tests/conformance/abi_v1/08_fetch_capability.howl. go test runs packages in
// parallel, and two listeners on 127.0.0.1:47653 cannot both own that fixture.
// The returned function releases the lock. Call it after the listener is closed.
func LockABIFetchFixture(t *testing.T) func() {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(os.TempDir(), "howlframe-abi-v1-phase2d.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("open fetch fixture lock: %v", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		t.Fatalf("lock fetch fixture: %v", err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}
}
