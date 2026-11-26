package javascript

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func filteredHowlEnv(extra []string) []string {
	out := make([]string, 0, len(extra)+len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "HOWLFRAME_ALLOW_CAPS=") && !strings.HasPrefix(entry, "HOWLFRAME_ABI_SECRET=") {
			out = append(out, entry)
		}
	}
	return append(out, extra...)
}

func runNodeScript(t *testing.T, source string, envPairs ...string) (string, string, int) {
	t.Helper()
	jsPath := filepath.Join(t.TempDir(), "app.js")
	if err := os.WriteFile(jsPath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", jsPath)
	if len(envPairs) > 0 {
		cmd.Env = filteredHowlEnv(envPairs)
	}
	var outBuf, errBuf strings.Builder
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), exitErr.ExitCode()
		}
		t.Fatal(err)
	}
	return outBuf.String(), errBuf.String(), 0
}

func assertJSMediatedBeforeRaw(t *testing.T, code, rawCall, mediatedCall, helperDecl, rawInHelper string) {
	t.Helper()
	if strings.Contains(code, rawCall) {
		t.Fatalf("raw call at call site (%s):\n%s", rawCall, code)
	}
	if !strings.Contains(code, mediatedCall) {
		t.Fatalf("missing mediation (%s):\n%s", mediatedCall, code)
	}
	idx := strings.Index(code, helperDecl)
	if idx < 0 {
		t.Fatalf("missing helper %s:\n%s", helperDecl, code)
	}
	body := code[idx:]
	if deny, raw := strings.Index(body, "CAPABILITY_DENIED"), strings.Index(body, rawInHelper); deny < 0 || raw < 0 || deny > raw {
		t.Fatalf("grant check must precede %s:\n%s", rawInHelper, body)
	}
}

type jsTypeErrCase struct {
	name, source, want string
}

func runJSTypeErrorCases(t *testing.T, cases []jsTypeErrCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertJSTypeCrash(t, tc.source, tc.want)
		})
	}
}

func assertJSTypeCrash(t *testing.T, source, want string) {
	t.Helper()
	code := generateCheckedJS(t, source)
	stdout, stderr, exitCode := runNodeScript(t, code)
	if exitCode == 0 {
		t.Fatalf("wrong type succeeded, stdout=%q stderr=%q\nsource:\n%s", stdout, stderr, code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func assertJSGrantedStdout(t *testing.T, code, capEnv, want string) {
	t.Helper()
	grantedOut, grantedErr, grantedCode := runJS(t, code, "HOWLFRAME_ALLOW_CAPS="+capEnv)
	if grantedCode != 0 {
		t.Fatalf("%s grant failed (%d): %s", capEnv, grantedCode, grantedErr)
	}
	if strings.TrimSpace(grantedOut) != want {
		t.Fatalf("granted stdout = %q, want %s", grantedOut, want)
	}
}

func assertJSCapabilityDenied(t *testing.T, code, capName, leakDesc string, leaks []string, extraCheck func(label string), envCases [][2]string) {
	t.Helper()
	for _, c := range envCases {
		label, envVal := c[0], c[1]
		var stdout, stderr string
		var exitCode int
		if envVal == "__UNSET__" {
			stdout, stderr, exitCode = runJS(t, code)
		} else {
			stdout, stderr, exitCode = runJS(t, code, envVal)
		}
		if exitCode == 0 {
			t.Fatalf("%s: expected capability denial, succeeded: %s", label, stdout+stderr)
		}
		if extraCheck != nil {
			extraCheck(label)
		}
		combined := stdout + stderr
		for _, leak := range leaks {
			if strings.Contains(combined, leak) {
				t.Fatalf("%s: output leaked %s (%s): stdout=%q stderr=%q", label, leakDesc, leak, stdout, stderr)
			}
		}
		if !strings.Contains(stderr, "CAPABILITY_DENIED") || !strings.Contains(stderr, "capability denied: "+capName) {
			t.Fatalf("%s: stderr = %q, want CAPABILITY_DENIED (%s)", label, stderr, capName)
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("%s: denied stdout = %q, want empty", label, stdout)
		}
	}
}
