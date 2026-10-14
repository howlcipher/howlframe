package vm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicRelativeStoreDoesNotDependOnSystemTemp(t *testing.T) {
	// The store must work even when the global temporary directory is missing
	// or on another device; only the destination directory is required.
	file, err := os.CreateTemp(".", "atomic-relative-store-*")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Base(file.Name())
	t.Cleanup(func() { os.Remove(path) })
	if _, err := file.WriteString("old"); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "missing-system-temp")
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, missing)
	}
	for _, value := range []string{"new", "replacement"} {
		if err := writeFileAtomic(path, []byte(value)); err != nil {
			t.Fatalf("relative store depends on global temp: %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != value {
			t.Fatalf("persisted store = %q, %v", data, err)
		}
	}
}
