package gogen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type goTypeErrCase struct {
	name, source, want string
}

func runGoTypeErrorCases(t *testing.T, cases []goTypeErrCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertGoCrashContains(t, tc.source, tc.want)
		})
	}
}

var standardGoFSGrants = [][2]string{
	{"unset grant", "__UNSET__"},
	{"empty grant", "HOWLFRAME_ALLOW_CAPS="},
	{"process grant", "HOWLFRAME_ALLOW_CAPS=process"},
}

func assertGoMediatedBeforeRaw(t *testing.T, code, rawNeedle, mediatedNeedle, helperDef, rawInHelper string) {
	t.Helper()
	if strings.Contains(code, rawNeedle) {
		t.Fatalf("call bypasses mediation (%s):\n%s", rawNeedle, code)
	}
	if !strings.Contains(code, mediatedNeedle) {
		t.Fatalf("call is not mediated (%s):\n%s", mediatedNeedle, code)
	}
	helperAt := strings.Index(code, helperDef)
	if helperAt < 0 {
		t.Fatalf("missing helper %s:\n%s", helperDef, code)
	}
	helper := code[helperAt:]
	denyAt := strings.Index(helper, "CAPABILITY_DENIED")
	rawAt := strings.Index(helper, rawInHelper)
	if denyAt < 0 || rawAt < 0 || denyAt > rawAt {
		t.Fatalf("grant check must precede %s:\n%s", rawInHelper, helper)
	}
}

func assertGoCrashContains(t *testing.T, source, want string) {
	t.Helper()
	code := generateCheckedGo(t, source)
	stdout, crash := goRunCrash(t, code)
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("stdout = %q, want empty\nsource:\n%s", stdout, code)
	}
	if !strings.Contains(crash, want) {
		t.Fatalf("crash = %q, want %q", crash, want)
	}
}

func assertGoGrantedStdout(t *testing.T, rootDir, code, capEnv, want string) {
	t.Helper()
	granted, grantErr := runGenerated(t, rootDir, code, "HOWLFRAME_ALLOW_CAPS="+capEnv)
	if grantErr != "" {
		t.Fatalf("%s grant failed: %s\n%s", capEnv, grantErr, granted)
	}
	if strings.TrimSpace(granted) != want {
		t.Fatalf("granted stdout = %q, want %s", granted, want)
	}
}

func assertGoCapabilityDenied(t *testing.T, rootDir, code, capName, leakDesc string, leaks []string, beforeCheck func(label string), envCases [][2]string) {
	t.Helper()
	assertGoCapabilityDeniedEnv(t, rootDir, code, capName, leakDesc, leaks, beforeCheck, nil, envCases)
}

func assertGoCapabilityDeniedEnv(t *testing.T, rootDir, code, capName, leakDesc string, leaks []string, beforeCheck func(label string), extraEnv []string, envCases [][2]string) {
	t.Helper()
	crashPath := filepath.Join(rootDir, "crash.json")
	t.Cleanup(func() { os.Remove(crashPath) })

	for _, c := range envCases {
		label, envVal := c[0], c[1]
		var env []string
		if envVal != "__UNSET__" {
			env = append(env, envVal)
		}
		env = append(env, extraEnv...)
		out, errText := runGenerated(t, rootDir, code, env...)
		if errText == "" {
			t.Fatalf("%s: expected capability denial, succeeded: %s", label, out)
		}
		if beforeCheck != nil {
			beforeCheck(label)
		}
		for _, leak := range leaks {
			if strings.Contains(out, leak) {
				t.Fatalf("%s: process output leaked %s (%s): %s", label, leakDesc, leak, out)
			}
		}
		crash, err := os.ReadFile(crashPath)
		if err != nil {
			t.Fatalf("%s: denial did not write crash.json: %v\n%s", label, err, out)
		}
		for _, leak := range leaks {
			if strings.Contains(string(crash), leak) {
				t.Fatalf("%s: crash.json leaked %s (%s): %s", label, leakDesc, leak, crash)
			}
		}
		if !strings.Contains(string(crash), "CAPABILITY_DENIED") || !strings.Contains(string(crash), "capability denied: "+capName) {
			t.Fatalf("%s: crash.json = %s, want CAPABILITY_DENIED (%s)", label, crash, capName)
		}
		os.Remove(crashPath)
	}
}
