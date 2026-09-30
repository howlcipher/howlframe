package vm

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/howlcipher/howlframe/internal/capability"
)

func TestInterpretEnvironmentCapabilityDeniedByDefault(t *testing.T) {
	os.Setenv("HOWL_SECRET_KEY", "unauthorized_token")
	defer os.Unsetenv("HOWL_SECRET_KEY")

	source := `(cli_app (print (env "HOWL_SECRET_KEY")))`
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	// Call Interpret with NO capabilities (fail-closed default)
	exitCode := Interpret(node, nil, nil, strings.NewReader(""), &stdout, &stderr)

	if exitCode == 0 {
		t.Fatalf("expected Interpret to fail when reading env without capability, got exit code 0; stdout=%q", stdout.String())
	}

	errStr := stderr.String()
	if !strings.Contains(errStr, "capability denied: environment") {
		t.Fatalf("expected 'capability denied: environment' in stderr, got: %s", errStr)
	}
}

func TestInterpretEnvironmentCapabilityPermittedWhenGranted(t *testing.T) {
	os.Setenv("HOWL_SECRET_KEY", "authorized_token")
	defer os.Unsetenv("HOWL_SECRET_KEY")

	source := `(cli_app (print (env "HOWL_SECRET_KEY")))`
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	// Call Interpret with Environment capability explicitly granted
	allowedCaps := []capability.Capability{capability.Environment}
	exitCode := Interpret(node, nil, allowedCaps, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected Interpret to succeed with Environment capability, got exit code %d; stderr=%s", exitCode, stderr.String())
	}

	if strings.TrimSpace(stdout.String()) != "authorized_token" {
		t.Fatalf("expected 'authorized_token' in stdout, got: %q", stdout.String())
	}
}

func TestInterpretExecDeniedBeforeSpawn(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "phase2b-secret-out")
	source := `(cli_app (exec "touch" "` + marker + `"))`
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	exitCode := Interpret(node, nil, nil, strings.NewReader(""), &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("expected Interpret to deny exec without process, got exit 0; stdout=%q", stdout.String())
	}
	errStr := stderr.String()
	if !strings.Contains(errStr, "capability denied: process") {
		t.Fatalf("expected 'capability denied: process' in stderr, got: %s", errStr)
	}
	if strings.Contains(errStr+stdout.String(), marker) || strings.Contains(errStr, "phase2b-secret-out") {
		t.Fatalf("denial leaked the command: stdout=%q stderr=%q", stdout.String(), errStr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("exec spawned touch before denial")
	}

	stdout.Reset()
	stderr.Reset()
	other := []capability.Capability{capability.Environment}
	exitCode = Interpret(node, nil, other, strings.NewReader(""), &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("environment grant ran exec; stdout=%q", stdout.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("environment grant spawned touch")
	}
}

func TestInterpretExecGrantedRuns(t *testing.T) {
	source := `(cli_app (print (bytes_to_string (exec "printf" "phase2b-exec-marker"))))`
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	allowed := []capability.Capability{capability.Process}
	exitCode := Interpret(node, nil, allowed, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected Interpret to run exec with process, got exit %d; stderr=%s", exitCode, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "phase2b-exec-marker" {
		t.Fatalf("stdout = %q, want phase2b-exec-marker", stdout.String())
	}
}

func TestInterpretReadFileDeniedBeforeRead(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "phase2c-secret-path.txt")
	const secretBody = "phase2c-secret-bytes"
	if err := os.WriteFile(secretPath, []byte(secretBody), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `(cli_app (print (bytes_to_string (read_file "` + secretPath + `"))))`
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	exitCode := Interpret(node, nil, nil, strings.NewReader(""), &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("expected Interpret to deny read_file without filesystem, got exit 0; stdout=%q", stdout.String())
	}
	errStr := stderr.String()
	if !strings.Contains(errStr, "capability denied: filesystem") {
		t.Fatalf("expected 'capability denied: filesystem' in stderr, got: %s", errStr)
	}
	if strings.Contains(errStr+stdout.String(), secretBody) || strings.Contains(errStr, "phase2c-secret-path") {
		t.Fatalf("denial leaked the file: stdout=%q stderr=%q", stdout.String(), errStr)
	}

	stdout.Reset()
	stderr.Reset()
	other := []capability.Capability{capability.Process}
	exitCode = Interpret(node, nil, other, strings.NewReader(""), &stdout, &stderr)
	if exitCode == 0 {
		t.Fatalf("process grant read the file; stdout=%q", stdout.String())
	}
	if strings.Contains(stdout.String()+stderr.String(), secretBody) {
		t.Fatalf("process grant leaked the file: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	missing := filepath.Join(dir, "phase2c-missing-secret.txt")
	missingSource := `(cli_app (print (bytes_to_string (read_file "` + missing + `"))))`
	missingNode, _ := parseAndCompile(t, missingSource)
	stdout.Reset()
	stderr.Reset()
	exitCode = Interpret(missingNode, nil, nil, strings.NewReader(""), &stdout, &stderr)
	if exitCode == 0 {
		t.Fatal("missing path was read without filesystem")
	}
	if strings.Contains(stderr.String(), "phase2c-missing-secret") || strings.Contains(stderr.String(), "no such file") {
		t.Fatalf("denial reached the filesystem: %s", stderr.String())
	}
}

func TestInterpretReadFileGrantedReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase2c-read.txt")
	if err := os.WriteFile(path, []byte("phase2c-read-marker"), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `(cli_app (print (bytes_to_string (read_file "` + path + `"))))`
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	allowed := []capability.Capability{capability.Filesystem}
	exitCode := Interpret(node, nil, allowed, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected Interpret to read with filesystem, got exit %d; stderr=%s", exitCode, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "phase2c-read-marker" {
		t.Fatalf("stdout = %q, want phase2c-read-marker", stdout.String())
	}
}

func TestInterpretWriteFileDeniedBeforeWrite(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "phase2e-secret-path.txt")
	const kept = "phase2e-kept"
	const secretBody = "phase2e-secret-bytes"
	if err := os.WriteFile(secretPath, []byte(kept), 0o644); err != nil {
		t.Fatal(err)
	}
	source := `(cli_app (write_file "` + secretPath + `" "` + secretBody + `") (print "phase2e-wrote"))`
	node, _ := parseAndCompile(t, source)

	assertDenied := func(label string, caps []capability.Capability) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		exitCode := Interpret(node, nil, caps, strings.NewReader(""), &stdout, &stderr)
		if exitCode == 0 {
			t.Fatalf("%s: expected Interpret to deny write_file, got exit 0; stdout=%q", label, stdout.String())
		}
		errStr := stderr.String()
		if !strings.Contains(errStr, "capability denied: filesystem") {
			t.Fatalf("%s: expected 'capability denied: filesystem' in stderr, got: %s", label, errStr)
		}
		if strings.Contains(errStr+stdout.String(), secretBody) || strings.Contains(errStr, "phase2e-secret-path") || strings.Contains(stdout.String(), "phase2e-wrote") {
			t.Fatalf("%s: denial leaked the write: stdout=%q stderr=%q", label, stdout.String(), errStr)
		}
		body, err := os.ReadFile(secretPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != kept {
			t.Fatalf("%s: file body = %q, want unchanged %s", label, body, kept)
		}
	}

	assertDenied("empty grant", nil)
	assertDenied("process grant", []capability.Capability{capability.Process})

	missing := filepath.Join(dir, "phase2e-missing-secret.txt")
	missingSource := `(cli_app (write_file "` + missing + `" "` + secretBody + `"))`
	missingNode, _ := parseAndCompile(t, missingSource)
	var stdout, stderr bytes.Buffer
	exitCode := Interpret(missingNode, nil, nil, strings.NewReader(""), &stdout, &stderr)
	if exitCode == 0 {
		t.Fatal("missing path was written without filesystem")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("denial created %s", missing)
	}
	if strings.Contains(stderr.String(), "phase2e-missing-secret") || strings.Contains(stderr.String(), secretBody) {
		t.Fatalf("denial reached the filesystem: %s", stderr.String())
	}
}

func TestInterpretWriteFileGrantedWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase2e-write.txt")
	source := `(cli_app (write_file "` + path + `" "phase2e-write-marker") (print "phase2e-wrote"))`
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	allowed := []capability.Capability{capability.Filesystem}
	exitCode := Interpret(node, nil, allowed, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected Interpret to write with filesystem, got exit %d; stderr=%s", exitCode, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "phase2e-wrote" {
		t.Fatalf("stdout = %q, want phase2e-wrote", stdout.String())
	}
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
	source := `(cli_app (mkdir "` + missing + `") (print "phase2e-made"))`
	node, _ := parseAndCompile(t, source)

	assertDenied := func(label string, caps []capability.Capability) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		exitCode := Interpret(node, nil, caps, strings.NewReader(""), &stdout, &stderr)
		if exitCode == 0 {
			t.Fatalf("%s: expected Interpret to deny mkdir, got exit 0; stdout=%q", label, stdout.String())
		}
		errStr := stderr.String()
		if !strings.Contains(errStr, "capability denied: filesystem") {
			t.Fatalf("%s: expected 'capability denied: filesystem' in stderr, got: %s", label, errStr)
		}
		if strings.Contains(errStr+stdout.String(), "phase2e-secret-dir") || strings.Contains(stdout.String(), "phase2e-made") {
			t.Fatalf("%s: denial leaked the directory: stdout=%q stderr=%q", label, stdout.String(), errStr)
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Fatalf("%s: mkdir created %s", label, missing)
		}
	}

	assertDenied("empty grant", nil)
	assertDenied("process grant", []capability.Capability{capability.Process})
}

func TestInterpretMkdirGrantedCreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phase2e-dir")
	source := `(cli_app (mkdir "` + path + `") (print "phase2e-made"))`
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	allowed := []capability.Capability{capability.Filesystem}
	exitCode := Interpret(node, nil, allowed, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected Interpret to mkdir with filesystem, got exit %d; stderr=%s", exitCode, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "phase2e-made" {
		t.Fatalf("stdout = %q, want phase2e-made", stdout.String())
	}
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
	secretURL := srv.URL + "/phase2d-secret-url"
	source := `(cli_app (print (bytes_to_string (fetch "` + secretURL + `" "GET"))))`
	node, _ := parseAndCompile(t, source)

	assertDenied := func(label string, caps []capability.Capability) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		exitCode := Interpret(node, nil, caps, strings.NewReader(""), &stdout, &stderr)
		if exitCode == 0 {
			t.Fatalf("%s: expected Interpret to deny fetch, got exit 0; stdout=%q", label, stdout.String())
		}
		errStr := stderr.String()
		if !strings.Contains(errStr, "capability denied: network") {
			t.Fatalf("%s: expected 'capability denied: network' in stderr, got: %s", label, errStr)
		}
		if hits.Load() != 0 {
			t.Fatalf("%s: denial performed %d HTTP request(s)", label, hits.Load())
		}
		if strings.Contains(errStr+stdout.String(), secretBody) || strings.Contains(errStr, "phase2d-secret-url") {
			t.Fatalf("%s: denial leaked the response or URL: stdout=%q stderr=%q", label, stdout.String(), errStr)
		}
	}

	assertDenied("empty grant", nil)
	assertDenied("filesystem grant", []capability.Capability{capability.Filesystem})
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
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	allowed := []capability.Capability{capability.Network}
	exitCode := Interpret(node, nil, allowed, strings.NewReader(""), &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected Interpret to fetch with network, got exit %d; stderr=%s", exitCode, stderr.String())
	}
	if hits.Load() != 1 {
		t.Fatalf("network grant performed %d HTTP request(s), want 1", hits.Load())
	}
	if strings.TrimSpace(stdout.String()) != secretBody {
		t.Fatalf("stdout = %q, want %s", stdout.String(), secretBody)
	}
}

func TestInterpretNetworkCapabilityDeniedByDefault(t *testing.T) {
	source := `(cli_app (neural_circuit () "test prompt"))`
	node, _ := parseAndCompile(t, source)

	var stdout, stderr bytes.Buffer
	// Call Interpret with NO capabilities
	exitCode := Interpret(node, nil, nil, strings.NewReader(""), &stdout, &stderr)

	if exitCode == 0 {
		t.Fatalf("expected Interpret to fail for neural_circuit without network capability, got exit code 0")
	}

	errStr := stderr.String()
	if !strings.Contains(errStr, "capability denied: network") {
		t.Fatalf("expected 'capability denied: network' in stderr, got: %s", errStr)
	}
}
