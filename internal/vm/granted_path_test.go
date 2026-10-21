package vm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
)

func TestGrantedPathQueriesMatchFilesystemAuthorization(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	read, err := capability.ParseGrant("filesystem:read=" + root)
	if err != nil {
		t.Fatal(err)
	}
	write, err := capability.ParseGrant("filesystem:write=" + filepath.Join(root, "output.json"))
	if err != nil {
		t.Fatal(err)
	}
	caps := []capability.Capability{read, write}
	for _, tc := range []struct {
		query string
		want  bool
	}{
		{"filesystem:read=" + filepath.Join(root, "missing.json"), true},
		{"filesystem:write=" + filepath.Join(root, "output.json"), true},
		{"filesystem:write=" + filepath.Join(root, "other.json"), false},
		{"filesystem:write=" + outside, false},
		{"filesystem:read=" + outside, false},
		{"filesystem:read=", false},
		{"filesystem:write=", false},
		{"filesystem", true},
		{"database", false},
	} {
		if got := capabilityGranted(caps, tc.query); got != tc.want {
			t.Errorf("granted(%q) = %v, want %v", tc.query, got, tc.want)
		}
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if capabilityGranted(caps, "filesystem:read="+filepath.Join(link, "private.json")) {
		t.Fatal("scope query followed a symlink outside its grant")
	}
	if _, err := os.Stat(filepath.Join(root, "output.json")); !os.IsNotExist(err) {
		t.Fatalf("permission query created a file: %v", err)
	}
}
