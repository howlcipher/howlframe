package capability

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGrant(t *testing.T) {
	for _, raw := range []string{"filesystem", "network", "filesystem:read=.", "filesystem:write=../out"} {
		grant, err := ParseGrant(raw)
		if err != nil || !(Grants{grant}).Has(Capability(raw)) && !(Grants{grant}).Has(Filesystem) {
			t.Fatalf("parse %q: %q %v", raw, grant, err)
		}
	}
	for _, raw := range []string{"filesystem:read=", "filesystem:write=", "filesystem:execute=/tmp", "network:read=/tmp", "unknown", "filesystem:read"} {
		if _, err := ParseGrant(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	grant, _ := ParseGrant("filesystem:read=.")
	cwd, _ := os.Getwd()
	if grant != Capability("filesystem:read="+cwd) {
		t.Fatalf("root not anchored: %q", grant)
	}
}

func TestFilesystemGrantContainment(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "data")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	read, _ := ParseGrant("filesystem:read=" + root)
	write, _ := ParseGrant("filesystem:write=" + root)
	for _, path := range []string{root, filepath.Join(root, "new", "file")} {
		if !(Grants{read}).AllowsRead(path) || (Grants{read}).AllowsWrite(path) || !(Grants{write}).AllowsWrite(path) || (Grants{write}).AllowsRead(path) {
			t.Fatalf("wrong scope for %s", path)
		}
	}
	for _, path := range []string{filepath.Join(base, "data2", "file"), root + "/../escape"} {
		if (Grants{read, write}).AllowsRead(path) || (Grants{read, write}).AllowsWrite(path) {
			t.Fatalf("escape allowed: %s", path)
		}
	}
	if !(Grants{Filesystem}).AllowsWrite(filepath.Join(base, "outside")) {
		t.Fatal("coarse alias denied")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if (Grants{read, write}).AllowsWrite(filepath.Join(root, "link", "new")) {
		t.Fatal("symlink escape allowed")
	}
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	if (Grants{write}).AllowsWrite(filepath.Join(root, "dangling")) {
		t.Fatal("dangling symlink allowed")
	}
}
