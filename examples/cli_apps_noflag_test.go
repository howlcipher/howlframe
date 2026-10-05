package receipt_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestCliAppsNoFlagBytecode runs every examples/ cli_app the way
// TestReceiptNoFlagBytecode runs examples/receipt.howl: the howlframe
// binary and the source path, with no flags. PATH contains only failing
// decoy go, node, and wat2wasm binaries. The test fails if a decoy runs,
// if server.go or server_test.go is written, or if the no-flag path does
// not leave a non-empty <name>.hfbc.
//
// Stdin is the payload the example already documents, or empty. The
// no-flag command does not forward arguments and does not grant
// capabilities. A non-zero status from that default policy, or from a
// program whose documented arguments this command cannot pass, is the
// recorded result. It is not a compiler bug, and the program is not rewritten.
func TestCliAppsNoFlagBytecode(t *testing.T) {
	repoRoot, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	repoRoot = filepath.Dir(repoRoot)

	binaryPath := filepath.Join(t.TempDir(), "howlframe")
	build := exec.Command("go", "build", "-o", binaryPath, ".")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}

	// Fail closed: every cli_app under examples/ is listed below.
	// A program that is not listed fails the test instead of being skipped.
	want := []cliAppNoFlagCase{
		{
			source: "examples/swarm_nested_dogfood/swarm_nested_dogfood.howl",
			exit:   1,
			stdout: "swarm start\n",
			stderr: "{\"phase\":\"runtime\",\"code\":\"CAPABILITY_DENIED\",\"function\":\"main\",\"instruction\":3,\"opcode\":\"SPAWN_AGENT\",\"message\":\"capability denied: process\"}\n",
		},
		{
			source: "examples/swarm_dogfood/swarm_dogfood.howl",
			exit:   1,
			stdout: "swarm start\n",
			stderr: "{\"phase\":\"runtime\",\"code\":\"CAPABILITY_DENIED\",\"function\":\"main\",\"instruction\":3,\"opcode\":\"SPAWN_AGENT\",\"message\":\"capability denied: process\"}\n",
		},
		{
			source: "examples/cli_hello.howl",
			stdin:  "",
			exit:   0,
			stdout: "Hello, World!\nWelcome to HowlFrame\n",
		},
		{
			source: "examples/native_math.howl",
			stdin:  "",
			exit:   0,
		},
		{
			source: "examples/language_tour/language_tour.howl",
			stdin:  "",
			exit:   2,
			stderr: "usage: language_tour <project> <tests> <security> <docs>",
		},
		{
			source: "examples/release_gate/release_gate.howl",
			stdin:  "",
			exit:   2,
			stderr: "usage: release_gate <config_file>",
		},
		{
			source: "examples/capability_lab/capability_lab.howl",
			stdin:  "",
			exit:   1,
			stdout: "Attempting to write to filesystem...\n",
			stderr: "{\"phase\":\"runtime\",\"code\":\"CAPABILITY_DENIED\",\"function\":\"main\",\"instruction\":6,\"opcode\":\"WRITE_FILE\",\"message\":\"capability denied: filesystem\"}\n",
		},
		{
			source: "examples/repo_analyst/repo_analyst.howl",
			stdin:  "",
			exit:   2,
			stderr: "usage: repo_analyst <repository-path> [output-file]",
		},
		{
			source: "examples/receipt.howl",
			stdin:  "apples 2 150\nbread 1 325\nmilk 1 007\n",
			exit:   0,
			stdout: "apples 300\nbread 325\nmilk 7\nlines: 3\nunits: 4\ntotal_cents: 632\n",
		},
		// poll_cli prints "poll ready" and the attempt count, then exits 0.
		// time_now is unix seconds and each sleep is 400ms, with a bound of 3,
		// so the count is 1, 2, or 3 depending on the wall clock. That integer
		// is the clock, not a bytecode bug. The program is not rewritten, and
		// the count stays in the match.
		{
			source:        "examples/poll_cli/poll_cli.howl",
			stdin:         "",
			exit:          0,
			stderr:        "",
			stdoutPattern: "^poll ready [123]\n$",
		},
	}

	found := cliAppSources(t, repoRoot)
	listed := make([]string, len(want))
	for i, c := range want {
		listed[i] = c.source
	}
	sort.Strings(found)
	sort.Strings(listed)
	if strings.Join(found, "\n") != strings.Join(listed, "\n") {
		t.Fatalf("cli_app programs = %v, test lists %v (refusing to skip a program)", found, listed)
	}

	for _, c := range want {
		c := c
		t.Run(c.source, func(t *testing.T) {
			toolDir := t.TempDir()
			invoked := filepath.Join(toolDir, "invoked")
			for _, name := range []string{"go", "node", "wat2wasm"} {
				path := filepath.Join(toolDir, name)
				body := "#!/bin/sh\necho \"$0\" >> " + invoked + "\nexit 99\n"
				if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			workDir := t.TempDir()
			cmd := exec.Command(binaryPath, filepath.Join(repoRoot, filepath.FromSlash(c.source)))
			cmd.Dir = workDir
			cmd.Stdin = strings.NewReader(c.stdin)
			var outBuf, errBuf bytes.Buffer
			cmd.Stdout = &outBuf
			cmd.Stderr = &errBuf
			cmd.Env = toolchainFreeEnv(t, toolDir)

			exit := 0
			err := cmd.Run()
			if err != nil {
				exitErr, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatalf("howlframe %s: %v\nstderr: %s", c.source, err, errBuf.String())
				}
				exit = exitErr.ExitCode()
			}
			if _, statErr := os.Stat(invoked); statErr == nil {
				executed, _ := os.ReadFile(invoked)
				t.Fatalf("transpiler toolchain was invoked: %s", executed)
			}
			if _, statErr := os.Stat(filepath.Join(workDir, "server.go")); !os.IsNotExist(statErr) {
				t.Fatalf("server.go was written: %v", statErr)
			}
			if _, statErr := os.Stat(filepath.Join(workDir, "server_test.go")); !os.IsNotExist(statErr) {
				t.Fatalf("server_test.go was written: %v", statErr)
			}
			base := strings.TrimSuffix(filepath.Base(c.source), filepath.Ext(c.source))
			artifact := filepath.Join(workDir, base+".hfbc")
			info, statErr := os.Stat(artifact)
			if statErr != nil {
				t.Fatalf("%s.hfbc missing: %v\nexit %d\nstdout %q\nstderr %q", base, statErr, exit, outBuf.String(), errBuf.String())
			}
			if info.Size() == 0 {
				t.Fatalf("%s.hfbc is empty", base)
			}
			if c.stdoutPattern != "" {
				if exit != c.exit || !regexp.MustCompile(c.stdoutPattern).MatchString(outBuf.String()) || errBuf.String() != c.stderr {
					t.Fatalf("exit %d stdout %q stderr %q, want exit %d stdout matching %q stderr %q",
						exit, outBuf.String(), errBuf.String(), c.exit, c.stdoutPattern, c.stderr)
				}
			} else if exit != c.exit || outBuf.String() != c.stdout || errBuf.String() != c.stderr {
				t.Fatalf("exit %d stdout %q stderr %q, want exit %d stdout %q stderr %q",
					exit, outBuf.String(), errBuf.String(), c.exit, c.stdout, c.stderr)
			}
		})
	}
}

type cliAppNoFlagCase struct {
	source string
	stdin  string
	exit   int
	stdout string
	stderr string
	// stdoutPattern, when set, is matched against stdout instead of exact
	// equality. Empty means the exact stdout string above.
	stdoutPattern string
}

func cliAppSources(t *testing.T, repoRoot string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(filepath.Join(repoRoot, "examples"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".howl" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !howlRootIsCliApp(string(body)) {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func howlRootIsCliApp(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, ";") {
			continue
		}
		return strings.HasPrefix(trim, "(cli_app")
	}
	return false
}
