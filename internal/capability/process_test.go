package capability

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseExecGrant(t *testing.T) {
	for raw, want := range map[string]string{
		"process:exec=/usr/bin/printf":           "process:exec=/usr/bin/printf",
		"process:exec=/usr/bin/printf  %s  **":   "process:exec=/usr/bin/printf %s **",
		"process:exec=/usr/bin/printf * literal": "process:exec=/usr/bin/printf * literal",
	} {
		got, err := ParseGrant(raw)
		if err != nil || string(got) != want {
			t.Fatalf("ParseGrant(%q) = %q, %v; want %q", raw, got, err, want)
		}
	}
	for _, raw := range []string{"process:exec=", "process:exec=printf", "process:exec=./printf", "process:exec=/usr/bin/../bin/printf", "process:exec=/usr/bin/printf ** x"} {
		if _, err := ParseGrant(raw); err == nil {
			t.Fatalf("ParseGrant(%q) accepted", raw)
		}
	}
}

func TestAuthorizeExec(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	prog := filepath.Join(dir, "tool")
	if err := os.WriteFile(prog, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(prog, link); err != nil {
		t.Skip(err)
	}
	g := Grants{Process, Capability("process:exec=" + prog + " a * **"), Capability("process:exec=" + link)}
	allow := []struct {
		program string
		args    []string
	}{
		{prog, []string{"a", "b"}}, {prog, []string{"a", "b", "c", "d"}},
	}
	for _, c := range allow {
		if _, err := g.AuthorizeExec(c.program, c.args); err != nil {
			t.Fatalf("AuthorizeExec(%q, %q) = %v", c.program, c.args, err)
		}
	}
	deny := []struct {
		program string
		args    []string
	}{
		{prog, nil}, {prog, []string{"a"}}, {prog, []string{"x", "b"}},
		{"tool", []string{"a", "b"}}, {link, nil}, {dir + "/./tool", []string{"a", "b"}},
	}
	for _, c := range deny {
		if _, err := g.AuthorizeExec(c.program, c.args); err == nil {
			t.Fatalf("AuthorizeExec(%q, %q) allowed", c.program, c.args)
		}
	}
	if _, err := (Grants{Process}).AuthorizeExec(prog, nil); err == nil {
		t.Fatal("bare process authorized exec")
	}
}

func TestAttenuate(t *testing.T) {
	parent := Grants{Process, Environment, "filesystem:read=/srv/data", "process:exec=/usr/bin/printf **"}
	ok := [][]string{nil, {}, {"process"}, {"environment"}, {"filesystem:read=/srv/data/sub"}, {"process:exec=/usr/bin/printf **"}}
	for _, req := range ok {
		if _, err := Attenuate(parent, req); err != nil {
			t.Fatalf("Attenuate(%q) = %v", req, err)
		}
	}
	if child, _ := Attenuate(parent, nil); len(child) != 0 {
		t.Fatalf("default child grants = %v, want none", child)
	}
	bad := [][]string{{"network"}, {"filesystem"}, {"filesystem:read=/srv"}, {"filesystem:write=/srv/data"}, {"process:exec=/usr/bin/printf"}, {"process:exec=/usr/bin/true **"}, {"bogus"}, {"environment", "database"}}
	for _, req := range bad {
		if _, err := Attenuate(parent, req); err == nil {
			t.Fatalf("Attenuate(%q) allowed escalation", req)
		}
	}
}
