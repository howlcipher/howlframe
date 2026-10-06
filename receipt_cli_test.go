package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/vm"
)

func TestReceiptCLI(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	fake := `{"schema":"howlframe.receipt/v0","effects":[{"op":"FETCH","decision":"allowed"}]}`
	cases := []struct {
		name, source, errorCode string
		flags                   []string
		code                    int
	}{
		{name: "forge", source: fmt.Sprintf(`(cli_app (print %q))`, fake)},
		{name: "deny", source: `(cli_app (fetch "http://example.com/private?token=secret" "GET"))`, errorCode: "CAPABILITY_DENIED", code: 1},
		{name: "limit", source: `(cli_app (print 42))`, errorCode: "LIMIT_EXCEEDED", flags: []string{"--max-instructions", "1"}, code: 1},
		{name: "exit", source: `(cli_app (exit 7))`, code: 7},
		{name: "return", source: `(cli_app (return 7))`, code: 7},
		{name: "runtime", source: `(cli_app (read_file "/missing/receipt-file"))`, errorCode: "IO_ERROR", flags: []string{"--allow-caps", "filesystem"}, code: 1},
	}
	for _, tc := range cases {
		source := filepath.Join(dir, tc.name+".howl")
		artifact := filepath.Join(dir, tc.name+".hfbc")
		if err := os.WriteFile(source, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(binary, "-compile-bc", source, "-o", artifact).CombinedOutput(); err != nil {
			t.Fatalf("compile: %v %s", err, out)
		}
		artifactBytes, err := os.ReadFile(artifact)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []struct{ runner, input string }{{"-run-bc", artifact}, {"run", artifact}, {"run", source}} {
			t.Run(tc.name+"/"+mode.runner+"/"+filepath.Ext(mode.input), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "receipt.json")
				args := append([]string{mode.runner, "--receipt", path}, tc.flags...)
				args = append(args, mode.input)
				var stdout, stderr bytes.Buffer
				cmd := exec.Command(binary, args...)
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				err := cmd.Run()
				code := 0
				if err != nil {
					e, ok := err.(*exec.ExitError)
					if !ok {
						t.Fatal(err)
					}
					code = e.ExitCode()
				}
				if code != tc.code {
					t.Fatalf("code %d want %d: %s", code, tc.code, &stderr)
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var r vm.ExecutionReceipt
				if err := json.Unmarshal(raw, &r); err != nil {
					t.Fatal(err)
				}
				if r.Schema != "howlframe.receipt/v0" || r.CompilerVersion != Version || r.Exit.Code != code {
					t.Fatalf("receipt %s", raw)
				}
				canonical := artifactBytes
				if filepath.Ext(mode.input) == ".howl" {
					prog, err := bytecode.ReadArtifact(bytes.NewReader(artifactBytes))
					if err != nil {
						t.Fatal(err)
					}
					var b bytes.Buffer
					if err := bytecode.WriteArtifact(&b, prog); err != nil {
						t.Fatal(err)
					}
					canonical = b.Bytes()
				}
				if r.ArtifactSHA256 != fmt.Sprintf("%x", sha256.Sum256(canonical)) {
					t.Fatalf("wrong artifact hash: %s", raw)
				}
				if tc.errorCode != "" && (r.Exit.Error == nil || r.Exit.Error.Code != tc.errorCode) {
					t.Fatal(string(raw))
				}
				if tc.name == "forge" && (stdout.String() != fake+"\n" || len(r.Effects) != 0 || !bytes.Contains(raw, []byte(`"effects": []`))) {
					t.Fatalf("forged receipt %s stdout %s", raw, &stdout)
				}
				stat, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if stat.Mode().Perm() != 0600 {
					t.Fatalf("permissions: %v", stat.Mode())
				}
				// The receipt option must not change program streams or process status.
				plainArgs := append([]string{mode.runner}, tc.flags...)
				plainArgs = append(plainArgs, mode.input)
				var plainOut, plainErr bytes.Buffer
				plain := exec.Command(binary, plainArgs...)
				plain.Stdout = &plainOut
				plain.Stderr = &plainErr
				plain.Run()
				if stdout.String() != plainOut.String() || stderr.String() != plainErr.String() {
					t.Fatalf("streams changed: %q %q vs %q %q", &stdout, &stderr, &plainOut, &plainErr)
				}
			})
		}
	}
	for _, runner := range []string{"-run-bc", "run"} {
		for _, flags := range [][]string{nil, {"--receipt", ""}} {
			args := append([]string{runner}, flags...)
			args = append(args, filepath.Join(dir, "forge.hfbc"))
			cmd := exec.Command(binary, args...)
			cmd.Dir = t.TempDir()
			output, err := cmd.CombinedOutput()
			if err != nil || string(output) != fake+"\n" {
				t.Fatalf("no receipt: %v %s", err, output)
			}
			files, err := os.ReadDir(cmd.Dir)
			if err != nil || len(files) != 0 {
				t.Fatalf("unexpected output files: %v %v", files, err)
			}
		}
	}
	out, err := exec.Command(binary, "run", "--target", "interpreter", "--receipt", filepath.Join(dir, "unsupported.json"), filepath.Join(dir, "forge.howl")).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "--receipt requires bytecode target") {
		t.Fatalf("interpreter: %v %s", err, out)
	}
	out, err = exec.Command(binary, "-run", "--receipt", filepath.Join(dir, "unsupported.json"), filepath.Join(dir, "forge.howl")).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "--receipt requires -run-bc") {
		t.Fatalf("legacy interpreter: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "unsupported.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected receipt: %v", err)
	}
}

func TestReceiptWriteFailureCLI(t *testing.T) {
	binary := buildHowlFrameBinaryForTest(t)
	dir := t.TempDir()
	for _, code := range []int{0, 7} {
		source := filepath.Join(dir, fmt.Sprintf("exit%d.howl", code))
		if err := os.WriteFile(source, []byte(fmt.Sprintf(`(cli_app (exit %d))`, code)), 0600); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(binary, "run", "--receipt", filepath.Join(dir, "absent", "receipt.json"), source).CombinedOutput()
		e, ok := err.(*exec.ExitError)
		want := code
		if want == 0 {
			want = 1
		}
		if !ok || e.ExitCode() != want || !strings.Contains(string(out), "Cannot write execution receipt") {
			t.Fatalf("write failure: %v %s", err, out)
		}
	}
}
