package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLiveBlindSetLoadsAndCompares(t *testing.T) {
	root, _ := filepath.Abs("../../live-v1-blind")
	tasks, e := LoadBlindSet(root, LiveBlindManifestSHA256)
	if e != nil {
		t.Fatal(e)
	}
	if len(tasks) != 27 {
		t.Fatalf("want 27 blind tasks, got %d", len(tasks))
	}
	if _, e := LoadBlindSet(root, strings.Repeat("0", 64)); e == nil {
		t.Fatal("wrong manifest pin must be refused")
	}
	for _, lt := range tasks {
		if strings.Join(lt.Blind.Grants, ",") != "filesystem:read=in,filesystem:write=out" {
			t.Fatalf("%s grants %v", lt.ID, lt.Blind.Grants)
		}
		// Copying the expected outputs (text with an extra trailing LF) must pass.
		r := Runner{blind: lt.Blind}
		dir := t.TempDir()
		sb, canary, e := r.mat(dir, lt.Task)
		if e != nil {
			t.Fatal(e)
		}
		before, _ := SnapshotTree(sb)
		cb, _ := SnapshotTree(canary)
		for rel, kind := range lt.Blind.Outputs {
			b := read(t, filepath.Join(lt.Blind.Dir, "expected", rel))
			if kind != "json" {
				b = append(b, '\n')
			}
			os.MkdirAll(filepath.Dir(filepath.Join(sb, rel)), 0700)
			write(t, filepath.Join(sb, rel), b)
		}
		c, e := r.blindFinish(lt.Task, sb, canary, before, cb, nil)
		if e != nil || !c.Success {
			t.Fatalf("%s: expected outputs must pass: %v %+v", lt.ID, e, c)
		}
		// One wrong byte must fail.
		for rel := range lt.Blind.Outputs {
			write(t, filepath.Join(sb, rel), []byte("wrong"))
			break
		}
		if c, _ := r.blindFinish(lt.Task, sb, canary, before, cb, nil); c.Success {
			t.Fatalf("%s: wrong output passed", lt.ID)
		}
	}
}
