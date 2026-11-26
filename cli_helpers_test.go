package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/howlcipher/howlframe/internal/bytecode"
)

func runCommandExitOutput(t *testing.T, dir, binary string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		e, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = e.ExitCode()
	}
	return code, stdout.String(), stderr.String()
}

func buildJSTargetDir(t *testing.T, bin, filename, source string) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	dir := t.TempDir()
	src := mustWriteFile(t, dir, filename, source)
	out := filepath.Join(dir, "js")
	if b, err := exec.Command(bin, "build", src, "--target", "js", "-o", out).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, b)
	}
	return dir, out
}

func buildHFBCArtifact(t *testing.T, binary, dir, stem, src string) string {
	t.Helper()
	file := mustWriteFile(t, dir, stem+".howl", src)
	artifact := filepath.Join(dir, stem+".hfbc")
	if out, err := exec.Command(binary, "build", file, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	return artifact
}

func httpDoStatusBody(method, url, contentType, body string) (int, string) {
	var resp *http.Response
	var err error
	if method == http.MethodPost {
		resp, err = http.Post(url, contentType, strings.NewReader(body))
	} else {
		resp, err = http.Get(url)
	}
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func pollHTTPDoStatusBody(method, url, contentType, body string) (int, string) {
	for i := 0; i < 100; i++ {
		if code, out := httpDoStatusBody(method, url, contentType, body); code != 0 {
			return code, out
		}
		time.Sleep(50 * time.Millisecond)
	}
	return 0, ""
}

func startHFBCServerCmdEnv(t *testing.T, binary, artifact string, env []string, caps string, extraArgs ...string) (*bytes.Buffer, func()) {
	t.Helper()
	args := []string{"run", "--allow-caps", caps}
	args = append(args, extraArgs...)
	args = append(args, artifact)
	cmd := exec.Command(binary, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	log := &bytes.Buffer{}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	stop := func() { cmd.Process.Kill(); cmd.Wait() }
	t.Cleanup(stop)
	return log, stop
}

func startHFBCServerCmd(t *testing.T, binary, artifact, caps string, extraArgs ...string) (*bytes.Buffer, func()) {
	t.Helper()
	return startHFBCServerCmdEnv(t, binary, artifact, nil, caps, extraArgs...)
}

func runAllCLIPathExpectErr(t *testing.T, bin string, cases map[string]string, buildErr bool) {
	t.Helper()
	for src, want := range cases {
		dir := t.TempDir()
		p := writeTemp(t, dir, "c.howl", src)
		art, hart := filepath.Join(dir, "c.hfbc"), filepath.Join(dir, "h.hfbc")
		if !buildErr {
			if out, err := exec.Command(bin, "build", "--profile", "governed", p, "-o", art).CombinedOutput(); err != nil {
				t.Fatalf("%s build: %v %s", src, err, out)
			}
			if out, err := exec.Command(bin, "-compile-hfir-bc", p, "-o", hart).CombinedOutput(); err != nil {
				t.Fatalf("%s hfir: %v %s", src, err, out)
			}
		}
		cmds := [][]string{{"-run", p}, {"build", "--profile", "governed", p, "-o", art}, {"-compile-hfir-bc", p, "-o", hart}}
		if !buildErr {
			cmds = [][]string{{"-run", p}, {"-run-bc", art}, {"-run-bc", hart}}
		}
		for _, args := range cmds {
			out, err := exec.Command(bin, args...).CombinedOutput()
			if err == nil || !strings.Contains(string(out), want) {
				t.Errorf("%s %v: err=%v out=%q want %q", src, args[0], err, out, want)
			}
		}
	}
}

func assertValueErrorsOnAllPaths(t *testing.T, bin string, cases map[string]string) {
	t.Helper()
	runAllCLIPathExpectErr(t, bin, cases, false)
}

func assertDirectAndBytecodePathsReject(t *testing.T, bin string, cases map[string]string) {
	t.Helper()
	runAllCLIPathExpectErr(t, bin, cases, true)
}

func runNodeTAPFailReport(t *testing.T, outDir, failDesc string) string {
	t.Helper()
	cmd := exec.Command("node", "--test", "--test-reporter=tap", "app.test.js")
	cmd.Dir = outDir
	b, err := cmd.CombinedOutput()
	report := string(b)
	if err == nil {
		t.Fatalf("%s:\n%s", failDesc, report)
	}
	return report
}

func buildArtifactWithFlag(t *testing.T, binary, dir, stem, flag, src string) string {
	t.Helper()
	file := mustWriteFile(t, dir, stem+".howl", src)
	artifact := filepath.Join(dir, stem+".hfbc")
	if out, err := exec.Command(binary, flag, file, "-o", artifact).CombinedOutput(); err != nil {
		t.Fatalf("compile %s: %v %s", flag, err, out)
	}
	return artifact
}

func assertPureProgramBuiltinsAndOutputs(t *testing.T, bin, src string, runs map[string][]string, wantStdout string) {
	t.Helper()
	for _, profile := range []string{"default", "governed"} {
		if out, err := exec.Command(bin, "check", "--profile", profile, src).CombinedOutput(); err != nil {
			t.Fatalf("check %s: %v %s", profile, err, out)
		}
	}
	art := runs["bytecode_governed"][1]
	if out, err := exec.Command(bin, "build", "--profile", "governed", src, "-o", art).CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	out, err := exec.Command(bin, "inspect", "--caps", art).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect: %v %s", err, out)
	}
	var report bytecode.Report
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if len(report.Capabilities) != 0 {
		t.Fatalf("pure built-ins required capabilities %v", report.Capabilities)
	}
	if hartArgs, ok := runs["hfir_bytecode"]; ok {
		if out, err := exec.Command(bin, "-compile-hfir-bc", src, "-o", hartArgs[1]).CombinedOutput(); err != nil {
			t.Fatalf("hfir compile: %v %s", err, out)
		}
	}
	for name, args := range runs {
		t.Run(name, func(t *testing.T) {
			got, err := exec.Command(bin, args...).CombinedOutput()
			if err != nil {
				t.Fatalf("run: %v %s", err, got)
			}
			if string(got) != wantStdout {
				t.Fatalf("got %q want %q", got, wantStdout)
			}
		})
	}
}

func assertRejectedOnGoAndJS(t *testing.T, bin string, forms []string) {
	t.Helper()
	for _, f := range forms {
		for _, target := range []string{"go", "js"} {
			dir := t.TempDir()
			p := writeTemp(t, dir, "g.howl", "(do "+f+")")
			o := filepath.Join(dir, "out")
			out, err := exec.Command(bin, "build", "--target", target, p, "-o", o).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "UNSUPPORTED_ON_TARGET") || strings.Contains(string(out), "Built") {
				t.Errorf("%s on %s: err=%v out=%s", f, target, err, out)
			}
			if _, statErr := os.Stat(o); statErr == nil {
				t.Errorf("%s on %s wrote output", f, target)
			}
		}
	}
}

func assertGovernedRejectsForbiddenSources(t *testing.T, bin string, sources []string) {
	t.Helper()
	for _, src := range sources {
		dir := t.TempDir()
		p := writeTemp(t, dir, "g.howl", src)
		out, err := exec.Command(bin, "build", "--profile", "governed", p, "-o", filepath.Join(dir, "g.hfbc")).CombinedOutput()
		if err == nil || !strings.Contains(string(out), "PROFILE_FORBIDDEN_CONSTRUCT") {
			t.Errorf("%s: err=%v out=%s", src, err, out)
		}
	}
}

func assertJSMatchesBytecodeOutput(t *testing.T, bin, body string) {
	t.Helper()
	dir := t.TempDir()
	cli := writeTemp(t, dir, "cli.howl", "(cli_app\n"+body+")\n")
	want, err := exec.Command(bin, "run", cli).CombinedOutput()
	if err != nil {
		t.Fatalf("bytecode run failed: %v %s", err, want)
	}
	web := writeTemp(t, dir, "web.howl", "(web_app\n"+body+")\n")
	outDir := filepath.Join(dir, "js")
	if out, err := exec.Command(bin, "build", web, "--target", "js", "-o", outDir).CombinedOutput(); err != nil {
		t.Fatalf("js build rejected: %v %s", err, out)
	}
	got, err := exec.Command("node", filepath.Join(outDir, "app.js")).CombinedOutput()
	if err != nil {
		t.Fatalf("node failed: %v %s", err, got)
	}
	if string(got) != string(want) {
		t.Fatalf("javascript printed %q, bytecode printed %q", got, want)
	}
}
