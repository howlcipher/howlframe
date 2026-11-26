package vm

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/ast"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/testutil"
)

func assertInterpretCapDenied(t *testing.T, label string, node *ast.Node, caps []capability.Capability, wantCap string, leakTokens ...string) string {
	t.Helper()
	exitCode, stdout, stderr := assertInterpretWithCaps(t, node, caps)
	if exitCode == 0 {
		t.Fatalf("%s: expected Interpret to deny %s, got exit 0; stdout=%q", label, wantCap, stdout)
	}
	if wantCap != "" && !strings.Contains(stderr, "capability denied: "+wantCap) {
		t.Fatalf("%s: expected 'capability denied: %s' in stderr, got: %s", label, wantCap, stderr)
	}
	combined := stdout + stderr
	for _, tok := range leakTokens {
		if strings.Contains(combined, tok) {
			t.Fatalf("%s: denial leaked %q: stdout=%q stderr=%q", label, tok, stdout, stderr)
		}
	}
	return stderr
}

func assertInterpretCapGranted(t *testing.T, source string, caps []capability.Capability, wantStdout string) {
	t.Helper()
	node, _ := parseAndCompile(t, source)
	exitCode, stdout, stderr := assertInterpretWithCaps(t, node, caps)
	if exitCode != 0 {
		t.Fatalf("expected Interpret to succeed with %v, got exit %d; stderr=%s", caps, exitCode, stderr)
	}
	if strings.TrimSpace(stdout) != wantStdout {
		t.Fatalf("stdout = %q, want %q", stdout, wantStdout)
	}
}

func TestInterpretEnvironmentCapabilityDeniedByDefault(t *testing.T) {
	os.Setenv("HOWL_SECRET_KEY", "unauthorized_token")
	defer os.Unsetenv("HOWL_SECRET_KEY")

	node, _ := parseAndCompile(t, `(cli_app (print (env "HOWL_SECRET_KEY")))`)
	assertInterpretCapDenied(t, "default", node, nil, "environment")
}

func TestInterpretEnvironmentCapabilityPermittedWhenGranted(t *testing.T) {
	os.Setenv("HOWL_SECRET_KEY", "authorized_token")
	defer os.Unsetenv("HOWL_SECRET_KEY")

	assertInterpretCapGranted(t, `(cli_app (print (env "HOWL_SECRET_KEY")))`, []capability.Capability{capability.Environment}, "authorized_token")
}

func TestInterpretExecDeniedBeforeSpawn(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "phase2b-secret-out")
	node, _ := parseAndCompile(t, `(cli_app (exec "touch" "`+marker+`"))`)

	assertInterpretCapDenied(t, "empty grant", node, nil, "process", marker, "phase2b-secret-out")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("exec spawned touch before denial")
	}

	assertInterpretCapDenied(t, "environment grant", node, []capability.Capability{capability.Environment}, "process")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("environment grant spawned touch")
	}
}

func TestInterpretExecGrantedRuns(t *testing.T) {
	source := testutil.CanonicalExecFixture(t, `(cli_app (print (bytes_to_string (exec "/usr/bin/printf" "phase2b-exec-marker"))))`)
	allowed := []capability.Capability{capability.Process, capability.Capability(testutil.CanonicalExecFixture(t, "process:exec=/usr/bin/printf phase2b-exec-marker"))}
	assertInterpretCapGranted(t, source, allowed, "phase2b-exec-marker")
}

func TestInterpretReadFileDeniedBeforeRead(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "phase2c-secret-path.txt")
	const secretBody = "phase2c-secret-bytes"
	if err := os.WriteFile(secretPath, []byte(secretBody), 0o644); err != nil {
		t.Fatal(err)
	}
	node, _ := parseAndCompile(t, `(cli_app (print (bytes_to_string (read_file "`+secretPath+`"))))`)
	assertInterpretCapDenied(t, "empty grant", node, nil, "filesystem", secretBody, "phase2c-secret-path")
	assertInterpretCapDenied(t, "process grant", node, []capability.Capability{capability.Process}, "filesystem", secretBody)

	missing := filepath.Join(dir, "phase2c-missing-secret.txt")
	missingNode, _ := parseAndCompile(t, `(cli_app (print (bytes_to_string (read_file "`+missing+`"))))`)
	assertInterpretCapDenied(t, "missing file", missingNode, nil, "filesystem", "phase2c-missing-secret", "no such file")
}

func TestInterpretReadFileGrantedReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase2c-read.txt")
	if err := os.WriteFile(path, []byte("phase2c-read-marker"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `(cli_app (print (bytes_to_string (read_file "` + path + `"))))`
	assertInterpretCapGranted(t, source, []capability.Capability{capability.Filesystem}, "phase2c-read-marker")
}

func TestInterpretWriteFileDeniedBeforeWrite(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "phase2e-secret-path.txt")
	const kept = "phase2e-kept"
	const secretBody = "phase2e-secret-bytes"
	if err := os.WriteFile(secretPath, []byte(kept), 0o644); err != nil {
		t.Fatal(err)
	}
	node, _ := parseAndCompile(t, `(cli_app (write_file "`+secretPath+`" "`+secretBody+`") (print "phase2e-wrote"))`)

	for _, caps := range [][]capability.Capability{nil, {capability.Process}} {
		label := "empty grant"
		if len(caps) > 0 {
			label = string(caps[0]) + " grant"
		}
		assertInterpretCapDenied(t, label, node, caps, "filesystem", secretBody, "phase2e-secret-path", "phase2e-wrote")
		body, err := os.ReadFile(secretPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != kept {
			t.Fatalf("%s: file body = %q, want unchanged %s", label, body, kept)
		}
	}

	missing := filepath.Join(dir, "phase2e-missing-secret.txt")
	missingNode, _ := parseAndCompile(t, `(cli_app (write_file "`+missing+`" "`+secretBody+`"))`)
	assertInterpretCapDenied(t, "missing write", missingNode, nil, "filesystem", "phase2e-missing-secret", secretBody)
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("denial created %s", missing)
	}
}

func TestInterpretWriteFileGrantedWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase2e-write.txt")
	source := `(cli_app (write_file "` + path + `" "phase2e-write-marker") (print "phase2e-wrote"))`
	assertInterpretCapGranted(t, source, []capability.Capability{capability.Filesystem}, "phase2e-wrote")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "phase2e-write-marker" {
		t.Fatalf("file = %q, want phase2e-write-marker", body)
	}
}

func TestInterpretMkdirDeniedBeforeCreate(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "phase2e-secret-dir")
	node, _ := parseAndCompile(t, `(cli_app (mkdir "`+missing+`") (print "phase2e-made"))`)

	for _, caps := range [][]capability.Capability{nil, {capability.Process}} {
		label := "empty grant"
		if len(caps) > 0 {
			label = string(caps[0]) + " grant"
		}
		assertInterpretCapDenied(t, label, node, caps, "filesystem", "phase2e-secret-dir", "phase2e-made")
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("%s: mkdir created %s", label, missing)
		}
	}
}

func TestInterpretMkdirGrantedCreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase2e-dir")
	source := `(cli_app (mkdir "` + path + `") (print "phase2e-made"))`
	assertInterpretCapGranted(t, source, []capability.Capability{capability.Filesystem}, "phase2e-made")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", path)
	}
}

func TestInterpretFetchDeniedBeforeRequest(t *testing.T) {
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	const secretBody = "phase2d-secret-bytes"
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, secretBody)
	}))
	t.Cleanup(srv.Close)
	node, _ := parseAndCompile(t, `(cli_app (print (bytes_to_string (fetch "`+srv.URL+`/phase2d-secret-url" "GET"))))`)

	for _, tc := range []struct {
		label string
		caps  []capability.Capability
	}{
		{"empty grant", nil},
		{"filesystem grant", []capability.Capability{capability.Filesystem}},
	} {
		assertInterpretCapDenied(t, tc.label, node, tc.caps, "network", secretBody, "phase2d-secret-url")
		if hits.Load() != 0 {
			t.Fatalf("%s: denial performed %d HTTP request(s)", tc.label, hits.Load())
		}
	}
}

func TestInterpretFetchGrantedReads(t *testing.T) {
	t.Setenv("NO_PROXY", "127.0.0.1,localhost")
	t.Setenv("no_proxy", "127.0.0.1,localhost")
	const secretBody = "phase2d-fetch-marker"
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, secretBody)
	}))
	t.Cleanup(srv.Close)
	source := `(cli_app (print (bytes_to_string (fetch "` + srv.URL + `/phase2d" "GET"))))`
	assertInterpretCapGranted(t, source, []capability.Capability{fetchGrantFor(t, srv.URL)}, secretBody)
	if hits.Load() != 1 {
		t.Fatalf("network grant performed %d HTTP request(s), want 1", hits.Load())
	}
}

func TestInterpretNetworkCapabilityDeniedByDefault(t *testing.T) {
	node, _ := parseAndCompile(t, `(cli_app (neural_circuit () "test prompt"))`)
	assertInterpretCapDenied(t, "default", node, nil, "network")
}
