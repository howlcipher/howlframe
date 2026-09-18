package vm

import (
	"bytes"
	"os"
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
