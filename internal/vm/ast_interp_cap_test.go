package vm

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
