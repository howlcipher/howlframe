package vm

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
)

// --timing-log is diagnostic only: a timed persist must write exactly the bytes
// an untimed one writes, and a slow one must report every phase.
func TestStoreTimingLineIsDiagnosticOnly(t *testing.T) {
	saved := storeTimingThreshold
	storeTimingThreshold = 0 // force the line for every write
	t.Cleanup(func() { storeTimingThreshold = saved })

	dir := t.TempDir()
	records := map[string]map[string]any{"t1": {"title": "a"}, "t2": {"title": "b"}}
	want, _ := json.MarshalIndent(records, "", "  ")
	line := regexp.MustCompile(`^store_timing kind=(put|delete) bytes=(\d+) marshal_ms=\d+\.\d{3} write_ms=\d+\.\d{3} fsync_ms=\d+\.\d{3} close_ms=\d+\.\d{3} rename_ms=\d+\.\d{3} dirsync_ms=\d+\.\d{3} total_ms=\d+\.\d{3} mono_ms=\d+\.\d{3}\n$`)

	for _, tc := range []struct {
		name   string
		timing bool
		op     bytecode.Opcode
		kind   string
	}{{"untimed", false, bytecode.OpStorePut, ""}, {"timed-put", true, bytecode.OpStorePut, "put"}, {"timed-delete", true, bytecode.OpStoreDelete, "delete"}} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".json")
			errOut := &bytes.Buffer{}
			machine := &BCVM{Limits: VMLimits{LogTiming: tc.timing}, ErrOut: errOut}
			store := &bcMemoryStore{file: path}
			machine.persistStore(store, records, tc.op)
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("persisted bytes differ with timing=%v: %v", tc.timing, err)
			}
			if leftovers, _ := filepath.Glob(path + ".tmp-*"); len(leftovers) != 0 {
				t.Fatalf("temporary files left: %v", leftovers)
			}
			if !tc.timing {
				if errOut.Len() != 0 {
					t.Fatalf("store timing must require LogTiming, got %q", errOut)
				}
				return
			}
			m := line.FindStringSubmatch(errOut.String())
			if m == nil || m[1] != tc.kind || m[2] != itoa(len(want)) {
				t.Fatalf("store_timing line = %q", errOut)
			}
		})
	}
}

func TestStoreTimingBelowThresholdIsSilent(t *testing.T) {
	saved := storeTimingThreshold
	storeTimingThreshold = 1 << 62
	t.Cleanup(func() { storeTimingThreshold = saved })
	errOut := &bytes.Buffer{}
	machine := &BCVM{Limits: VMLimits{LogTiming: true}, ErrOut: errOut}
	machine.persistStore(&bcMemoryStore{file: filepath.Join(t.TempDir(), "s.json")}, map[string]map[string]any{}, bytecode.OpStorePut)
	if errOut.Len() != 0 {
		t.Fatalf("fast write must not log: %q", errOut)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
