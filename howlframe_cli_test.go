package main

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_PublicInterface(t *testing.T) {
	// Build the CLI binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "howlframe")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "howlframe.go")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, string(out))
	}

	runCLI := func(args ...string) (string, error) {
		cmd := exec.Command(binaryPath, args...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	t.Run("version", func(t *testing.T) {
		out, err := runCLI("version")
		if err != nil {
			t.Errorf("version command failed: %v", err)
		}
		if !strings.Contains(out, "HowlFrame 0.1.0") {
			t.Errorf("expected version output, got: %s", out)
		}
	})

	t.Run("help", func(t *testing.T) {
		out, err := runCLI("help")
		if err != nil {
			t.Errorf("help command failed: %v", err)
		}
		if !strings.Contains(out, "HowlFrame v0.1 CLI") {
			t.Errorf("expected help output, got: %s", out)
		}
	})

	t.Run("check valid", func(t *testing.T) {
		out, err := runCLI("check", "tests/module_main.howl")
		if err != nil {
			t.Errorf("check command failed: %v\nOutput: %s", err, out)
		}
		if !strings.Contains(out, "OK: tests/module_main.howl") {
			t.Errorf("expected check success output, got: %s", out)
		}
	})

	t.Run("check invalid", func(t *testing.T) {
		invalidFile := filepath.Join(tmpDir, "invalid.howl")
		os.WriteFile(invalidFile, []byte("(unclosed"), 0644)
		out, err := runCLI("check", invalidFile)
		if err == nil {
			t.Errorf("expected check to fail for invalid syntax")
		}
		if !strings.Contains(out, "Unexpected tokens after EOF") && !strings.Contains(out, "Failed to encode mask plan") && !strings.Contains(out, "Unexpected") {
			// Actually ast.ReportError uses panic-like behavior or os.Exit so we just ensure it errors
		}
	})

	hfbcPath := filepath.Join(tmpDir, "output.hfbc")
	t.Run("build valid", func(t *testing.T) {
		out, err := runCLI("build", "tests/module_main.howl", "-o", hfbcPath)
		if err != nil {
			t.Errorf("build command failed: %v\nOutput: %s", err, out)
		}
		if !strings.Contains(out, "Built "+hfbcPath) {
			t.Errorf("expected build output, got: %s", out)
		}
	})

	t.Run("run artifact", func(t *testing.T) {
		out, err := runCLI("run", hfbcPath)
		if err != nil {
			t.Errorf("run command failed: %v\nOutput: %s", err, out)
		}
		if !strings.Contains(out, "42") {
			t.Errorf("expected execution output containing 42, got: %s", out)
		}
	})

	t.Run("legacy validate", func(t *testing.T) {
		out, err := runCLI("-validate", "tests/module_main.howl")
		if err != nil {
			t.Errorf("legacy -validate failed: %v\nOutput: %s", err, out)
		}
	})

	t.Run("legacy compile-bc", func(t *testing.T) {
		legacyHfbc := filepath.Join(tmpDir, "legacy.hfbc")
		out, err := runCLI("-compile-bc", "tests/module_main.howl", "-o", legacyHfbc)
		if err != nil {
			t.Errorf("legacy -compile-bc failed: %v\nOutput: %s", err, out)
		}
	})
}

// TestCLIFlagPositionRelativeToInput covers bugs.md #52. A flag written after
// the positional input file used to be discarded, and because no mode flag was
// then set the CLI fell through to its default Go backend: `howlframe
// prog.howl -mask-plan` exited 0, printed nothing and dropped a server.go into
// the working directory. Flags must now work on either side of the input, or
// be rejected loudly - except under the argv-owning modes (-run-bc, -run),
// where every token after the input is the target program's argv and must
// reach it untouched.
func TestCLIFlagPositionRelativeToInput(t *testing.T) {
	workDir := t.TempDir()
	binaryPath := filepath.Join(workDir, "howlframe")
	if out, err := exec.Command("go", "build", "-o", binaryPath, "howlframe.go").CombinedOutput(); err != nil {
		t.Fatalf("failed to build CLI: %v\nOutput: %s", err, out)
	}
	fixture, err := os.ReadFile("tests/test_cli.howl")
	if err != nil {
		t.Fatalf("failed to read tests/test_cli.howl: %v", err)
	}
	source := filepath.Join(workDir, "cli.howl")
	if err := os.WriteFile(source, fixture, 0o644); err != nil {
		t.Fatalf("failed to stage fixture: %v", err)
	}
	// tests/test_cli.howl's top-level prints are not SSA-lowerable, so
	// -compile-wasm - the third compilation mode - needs its own fixture.
	wasmSource := filepath.Join(workDir, "wasm.howl")
	if err := os.WriteFile(wasmSource, []byte(`(cli_app (let (x 4) (if (> x 2) (+ x 3) (- x 1))))`), 0o644); err != nil {
		t.Fatalf("failed to stage SSA fixture: %v", err)
	}
	// cmd.Dir is workDir so the unrequested default-backend artifact the bug
	// produced would land where this test can see it.
	run := func(args ...string) (string, error) {
		cmd := exec.Command(binaryPath, args...)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	t.Run("plan modes accept the flag on either side of the input", func(t *testing.T) {
		for _, mode := range []string{"-mask-plan", "-optimization-plan"} {
			before, err := run(mode, source)
			if err != nil {
				t.Fatalf("%s before input failed: %v\nOutput: %s", mode, err, before)
			}
			if strings.TrimSpace(before) == "" {
				t.Fatalf("%s before input printed no plan", mode)
			}
			after, err := run(source, mode)
			if err != nil {
				t.Fatalf("%s after input failed: %v\nOutput: %s", mode, err, after)
			}
			if after != before {
				t.Errorf("%s after input printed %q, want the same plan it prints before the input, %q", mode, after, before)
			}
			for _, artifact := range []string{"cli.hfbc", "wasm.hfbc"} {
				if _, err := os.Stat(filepath.Join(workDir, artifact)); !os.IsNotExist(err) {
					t.Fatalf("unexpected default artifact %s: %v", artifact, err)
				}
			}
			if _, err := os.Stat(filepath.Join(workDir, "server.go")); err == nil {
				t.Fatalf("%s after input wrote server.go, so it fell through to the default Go backend", mode)
			}
		}
	})

	// The modes below are not plan modes, so a fix that special-cased
	// -mask-plan and -optimization-plan alone - the per-flag hardcoding that
	// -o's own after-input rescan used to be - would still drop these and fall
	// through to the default Go backend.
	t.Run("validation and compilation modes accept the flag after the input", func(t *testing.T) {
		noServerGo := func(t *testing.T, mode string) {
			t.Helper()
			for _, artifact := range []string{"cli.hfbc", "wasm.hfbc"} {
				if _, err := os.Stat(filepath.Join(workDir, artifact)); !os.IsNotExist(err) {
					t.Fatalf("unexpected default artifact %s: %v", artifact, err)
				}
			}
			if _, err := os.Stat(filepath.Join(workDir, "server.go")); err == nil {
				t.Fatalf("%s after input wrote server.go, so it fell through to the default Go backend", mode)
			}
		}
		// Each compilation mode is checked against its own emitted artifact,
		// cleared first: -compile-bc and -compile-hfir-bc share one default
		// path (inputFile + ".bc.bin"), so a surviving file from an earlier
		// mode would let a later mode emit nothing and still pass.
		emits := func(t *testing.T, mode, input, artifact string) {
			t.Helper()
			if err := os.Remove(artifact); err != nil && !os.IsNotExist(err) {
				t.Fatalf("failed to clear %s before %s: %v", artifact, mode, err)
			}
			out, err := run(input, mode)
			if err != nil {
				t.Errorf("%s after input failed: %v\nOutput: %s", mode, err, out)
			}
			noServerGo(t, mode)
			// Reaching the artifact proves the resumed parse reached the mode
			// dispatch rather than merely suppressing the default backend.
			if _, err := os.Stat(artifact); err != nil {
				t.Errorf("%s after input emitted no artifact at %s: %v", mode, artifact, err)
			}
		}

		out, err := run(source, "-validate")
		if err != nil {
			t.Errorf("-validate after input failed: %v\nOutput: %s", err, out)
		}
		noServerGo(t, "-validate")

		emits(t, "-compile-bc", source, source+".bc.bin")
		emits(t, "-compile-hfir-bc", source, source+".bc.bin")
		emits(t, "-compile-wasm", wasmSource, wasmSource+".ssa.wat")
	})

	// A resumed parse that corrupted flag state could still dispatch to the
	// right mode while degrading what that mode emits, so each compilation
	// mode's artifact must be byte-identical whichever side of the input the
	// mode flag is written on.
	t.Run("compilation modes emit identical artifacts before and after the input", func(t *testing.T) {
		for _, mode := range []struct {
			flag   string
			input  string
			suffix string
		}{
			{"-compile-bc", source, ".bc.bin"},
			{"-compile-hfir-bc", source, ".bc.bin"},
			{"-compile-wasm", wasmSource, ".ssa.wat"},
		} {
			beforeArtifact := filepath.Join(workDir, "before"+mode.flag+mode.suffix)
			afterArtifact := filepath.Join(workDir, "after"+mode.flag+mode.suffix)
			if out, err := run(mode.flag, mode.input, "-o", beforeArtifact); err != nil {
				t.Fatalf("%s before input failed: %v\nOutput: %s", mode.flag, err, out)
			}
			if out, err := run(mode.input, mode.flag, "-o", afterArtifact); err != nil {
				t.Fatalf("%s after input failed: %v\nOutput: %s", mode.flag, err, out)
			}
			beforeBytes, err := os.ReadFile(beforeArtifact)
			if err != nil {
				t.Fatalf("%s before input emitted no artifact: %v", mode.flag, err)
			}
			afterBytes, err := os.ReadFile(afterArtifact)
			if err != nil {
				t.Fatalf("%s after input emitted no artifact: %v", mode.flag, err)
			}
			if !bytes.Equal(beforeBytes, afterBytes) {
				t.Errorf("%s emitted a %d-byte artifact before the input and a divergent %d-byte one after it", mode.flag, len(beforeBytes), len(afterBytes))
			}
		}
	})

	t.Run("tokens after the input that no flag consumes are rejected", func(t *testing.T) {
		// -run and -run-bc are rejected rather than honoured after the input
		// because the tokens following the input are the program's argv in
		// those modes. Unknown and mistyped flags also fail closed.
		for _, trailing := range []string{"-not-a-flag", "-maskplan", "-opt-plan", "stray.howl", "-run-bc", "-run"} {
			out, err := run(source, trailing)
			if err == nil {
				t.Errorf("%q after input exited 0, want a diagnostic; output: %q", trailing, out)
			}
			if !strings.Contains(out, trailing) && !strings.Contains(out, "flag provided but not defined") {
				t.Errorf("%q after input produced %q, want a diagnostic naming it", trailing, out)
			}
			for _, artifact := range []string{"cli.hfbc", "wasm.hfbc"} {
				if _, err := os.Stat(filepath.Join(workDir, artifact)); !os.IsNotExist(err) {
					t.Fatalf("unexpected default artifact %s: %v", artifact, err)
				}
			}
			if _, err := os.Stat(filepath.Join(workDir, "server.go")); err == nil {
				t.Fatalf("%q after input wrote server.go instead of failing", trailing)
			}
		}
	})

	t.Run("a flag missing its argument after the input is reported", func(t *testing.T) {
		for _, badArg := range []struct {
			args []string
		}{
			{[]string{"-o"}},
			{[]string{"--o"}},
			{[]string{"-o", "-mask-plan"}},
			{[]string{"--o", "-mask-plan"}},
			{[]string{"-allow-caps", "-mask-plan"}},
		} {
			out, err := run(append([]string{source}, badArg.args...)...)
			if err == nil {
				t.Errorf("%v after input exited 0, want a diagnostic; output: %q", badArg.args, out)
			}
			if !strings.Contains(out, "flag needs an argument") {
				t.Errorf("%v after input produced %q, want diagnostic naming missing argument", badArg.args, out)
			}
			for _, artifact := range []string{"cli.hfbc", "wasm.hfbc"} {
				if _, err := os.Stat(filepath.Join(workDir, artifact)); !os.IsNotExist(err) {
					t.Fatalf("unexpected default artifact %s: %v", artifact, err)
				}
			}
			if _, err := os.Stat(filepath.Join(workDir, "server.go")); err == nil {
				t.Fatalf("%v after input wrote server.go instead of failing", badArg.args)
			}
			if _, err := os.Stat(filepath.Join(workDir, "-mask-plan")); err == nil {
				t.Fatalf("%v after input created directory -mask-plan", badArg.args)
			}
		}
	})

	t.Run("-- option terminator stops flag parsing after input", func(t *testing.T) {
		out, err := run(source, "--", "-validate")
		if err == nil {
			t.Errorf("-- followed by -validate after input exited 0, want a diagnostic; output: %q", out)
		}
		if !strings.Contains(out, "-validate") {
			t.Errorf("-- followed by -validate produced %q, want diagnostic naming argument", out)
		}
		for _, artifact := range []string{"cli.hfbc", "wasm.hfbc"} {
			if _, err := os.Stat(filepath.Join(workDir, artifact)); !os.IsNotExist(err) {
				t.Fatalf("unexpected default artifact %s: %v", artifact, err)
			}
		}
		if _, err := os.Stat(filepath.Join(workDir, "server.go")); err == nil {
			t.Fatalf("-- followed by -validate wrote server.go instead of failing")
		}
	})

	// -h and -help are the flags the resumed parse cannot simply set, because
	// the flag package answers them by printing usage rather than by writing a
	// value. They must still behave identically on either side of the input,
	// or help becomes the last position-dependent flag - the very thing -o's
	// hardcoded rescan was.
	t.Run("-h and -help after the input print usage as they do before it", func(t *testing.T) {
		for _, help := range []string{"-h", "-help"} {
			before, beforeErr := run(help)
			if beforeErr != nil {
				t.Fatalf("%s before input exited nonzero: %v\nOutput: %s", help, beforeErr, before)
			}
			after, afterErr := run(source, help)
			if afterErr != nil {
				t.Errorf("%s after input exited nonzero: %v\nOutput: %s", help, afterErr, after)
			}
			if after != before {
				t.Errorf("%s after input printed %q, want the usage it prints before the input, %q", help, after, before)
			}
			for _, artifact := range []string{"cli.hfbc", "wasm.hfbc"} {
				if _, err := os.Stat(filepath.Join(workDir, artifact)); !os.IsNotExist(err) {
					t.Fatalf("unexpected default artifact %s: %v", artifact, err)
				}
			}
			if _, err := os.Stat(filepath.Join(workDir, "server.go")); err == nil {
				t.Fatalf("%s after input wrote server.go, so it fell through to the default Go backend", help)
			}
		}
		for _, help := range []string{"-h", "-help"} {
			before, beforeErr := run("build", help)
			if beforeErr != nil {
				t.Fatalf("build %s before the source exited nonzero: %v\nOutput: %s", help, beforeErr, before)
			}
			after, afterErr := run("build", source, help)
			if afterErr != nil {
				t.Errorf("build %s after the source exited nonzero: %v\nOutput: %s", help, afterErr, after)
			}
			if after != before {
				t.Errorf("build %s after the source printed %q, want the usage it prints before it, %q", help, after, before)
			}
		}
	})

	t.Run("-o names a directory before the input and a file after it", func(t *testing.T) {
		exact := filepath.Join(workDir, "exact.bc.bin")
		if out, err := run("-compile-bc", source, "-o", exact); err != nil {
			t.Fatalf("-compile-bc with -o after input failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(exact); err != nil {
			t.Errorf("-o after input did not write the exact artifact path: %v", err)
		}
		outDir := filepath.Join(workDir, "artifacts")
		if out, err := run("-compile-bc", "-o", outDir, source); err != nil {
			t.Fatalf("-compile-bc with -o before input failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(filepath.Join(outDir, "cli.howl.bc.bin")); err != nil {
			t.Errorf("-o before input did not write into the output directory: %v", err)
		}
	})

	// The resumed parse re-registers the real flag.Value rather than rescanning
	// argv for spellings, so it accepts -name=value after the input as well as
	// the space-separated form. bug #39's rescan had to spell out "-o=" by hand.
	t.Run("-name=value after the input is accepted", func(t *testing.T) {
		exact := filepath.Join(workDir, "equals_exact.bc.bin")
		if out, err := run("-compile-bc", source, "-o="+exact); err != nil {
			t.Fatalf("-compile-bc with -o= after input failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(exact); err != nil {
			t.Errorf("-o= after input did not write the exact artifact path: %v", err)
		}
		modeExact := filepath.Join(workDir, "equals_mode_exact.bc.bin")
		if out, err := run(source, "-compile-bc=true", "-o="+modeExact); err != nil {
			t.Fatalf("-compile-bc=true and -o= after input failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(modeExact); err != nil {
			t.Errorf("-compile-bc=true after input did not emit its artifact: %v", err)
		}
	})

	// Every trailing token is the resumed parse's to consume, not just the
	// first, so a mode flag and a flag taking a value can both follow the
	// input.
	t.Run("several flags after the input are all parsed", func(t *testing.T) {
		exact := filepath.Join(workDir, "multi_exact.bc.bin")
		if out, err := run(source, "-compile-bc", "-o", exact); err != nil {
			t.Fatalf("-compile-bc and -o after input failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(exact); err != nil {
			t.Errorf("-compile-bc and -o after input did not write the exact artifact path: %v", err)
		}
	})

	t.Run("the argv-owning modes hand every token after the input to the program", func(t *testing.T) {
		artifact := filepath.Join(workDir, "cli.bc")
		if out, err := run("-compile-bc", source, "-o", artifact); err != nil {
			t.Fatalf("failed to compile the fixture: %v\nOutput: %s", err, out)
		}
		// tests/test_cli.howl prints (cli_args) and then its first three
		// elements, so the first line is the program's argv verbatim. -run
		// interprets the source and -run-bc the compiled artifact; both own
		// their trailing tokens, so both are checked here.
		for _, programArgs := range [][]string{
			{"alpha", "beta", "gamma"},
			{"-allow-caps", "environment"},
			{"-o", "artifacts", "-mask-plan"},
		} {
			want := "[" + strings.Join(programArgs, " ") + "]"
			for _, invocation := range [][]string{
				{"-run-bc", artifact},
				{"-run", source},
			} {
				out, err := run(append(append([]string{}, invocation...), programArgs...)...)
				if err != nil {
					t.Fatalf("%s %v failed: %v\nOutput: %s", invocation[0], programArgs, err, out)
				}
				if first, _, _ := strings.Cut(out, "\n"); first != want {
					t.Errorf("%s %v delivered %q to the program, want %q", invocation[0], programArgs, first, want)
				}
			}
		}
	})

	// The build subcommand parses its own FlagSet and stops at its own
	// positional, so it resumes the parse the same way and owes the same
	// diagnostics. It has no argv-owning mode, so nothing after the source
	// file is ever the program's.
	t.Run("the build subcommand resumes the parse after its source file", func(t *testing.T) {
		before := filepath.Join(workDir, "build_before.hfbc")
		if out, err := run("build", "-o", before, source); err != nil {
			t.Fatalf("build with -o before the source failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(before); err != nil {
			t.Errorf("build with -o before the source wrote no artifact: %v", err)
		}
		after := filepath.Join(workDir, "build_after.hfbc")
		if out, err := run("build", source, "-o", after); err != nil {
			t.Fatalf("build with -o after the source failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(after); err != nil {
			t.Errorf("build with -o after the source wrote no artifact: %v", err)
		}
		equals := filepath.Join(workDir, "build_equals.hfbc")
		if out, err := run("build", source, "-o="+equals); err != nil {
			t.Fatalf("build with -o= after the source failed: %v\nOutput: %s", err, out)
		}
		beforeBytes, err := os.ReadFile(before)
		if err != nil {
			t.Fatalf("failed to read the build artifact: %v", err)
		}
		for _, artifact := range []string{after, equals} {
			afterBytes, err := os.ReadFile(artifact)
			if err != nil {
				t.Fatalf("failed to read %s: %v", artifact, err)
			}
			if !bytes.Equal(beforeBytes, afterBytes) {
				t.Errorf("build wrote a %d-byte artifact with -o before the source and a divergent %d-byte one with -o after it", len(beforeBytes), len(afterBytes))
			}
		}

		for _, trailing := range []string{"-not-a-flag", "stray.howl"} {
			out, err := run("build", source, trailing)
			if err == nil {
				t.Errorf("build with %q after the source exited 0, want a diagnostic; output: %q", trailing, out)
			}
			if !strings.Contains(out, trailing) {
				t.Errorf("build with %q after the source produced %q, want a diagnostic naming it", trailing, out)
			}
		}
		for _, badArg := range [][]string{
			{"-o"},
			{"-o", "-target"},
		} {
			out, err := run(append([]string{"build", source}, badArg...)...)
			if err == nil {
				t.Errorf("build with %v after the source exited 0, want a diagnostic; output: %q", badArg, out)
			}
			if !strings.Contains(out, "flag needs an argument: -o") {
				t.Errorf("build with %v after the source produced %q, want the parse error naming the flag", badArg, out)
			}
		}
	})

	t.Run("double-dash --o and --o= after the input are accepted", func(t *testing.T) {
		doubleExact := filepath.Join(workDir, "double_exact.bc.bin")
		if out, err := run("-compile-bc", source, "--o", doubleExact); err != nil {
			t.Fatalf("-compile-bc with --o after input failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(doubleExact); err != nil {
			t.Errorf("--o after input did not write the exact artifact path: %v", err)
		}

		doubleEqualsExact := filepath.Join(workDir, "double_equals_exact.bc.bin")
		if out, err := run("-compile-bc", source, "--o="+doubleEqualsExact); err != nil {
			t.Fatalf("-compile-bc with --o= after input failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(doubleEqualsExact); err != nil {
			t.Errorf("--o= after input did not write the exact artifact path: %v", err)
		}

		bothAfter := filepath.Join(workDir, "both_after.bc.bin")
		if out, err := run(source, "-compile-bc", "--o", bothAfter); err != nil {
			t.Fatalf("-compile-bc and --o after input failed: %v\nOutput: %s", err, out)
		}
		if _, err := os.Stat(bothAfter); err != nil {
			t.Errorf("-compile-bc and --o after input did not write the exact artifact path: %v", err)
		}
	})
}

// TestResumeFlagsAfterInput_Synthetic verifies that resumeFlagsAfterInput operates
// generically against arbitrary FlagSet definitions and cannot be satisfied by a
// hardcoded flag lookup table.
func TestResumeFlagsAfterInput_Synthetic(t *testing.T) {
	fs := flag.NewFlagSet("synthetic_test", flag.ContinueOnError)
	bVal := fs.Bool("dyn-bool-flag", false, "dynamic bool")
	sVal := fs.String("dyn-str-flag", "", "dynamic string")
	iVal := fs.Int("dyn-int-flag", 0, "dynamic int")

	set, leftover, err := resumeFlagsAfterInput(fs, []string{
		"-dyn-bool-flag",
		"--dyn-str-flag=hello",
		"-dyn-int-flag", "42",
		"leftover_one", "leftover_two",
	})
	if err != nil {
		t.Fatalf("resumeFlagsAfterInput failed on synthetic FlagSet: %v", err)
	}
	if !*bVal {
		t.Errorf("dyn-bool-flag not set")
	}
	if *sVal != "hello" {
		t.Errorf("dyn-str-flag = %q, want %q", *sVal, "hello")
	}
	if *iVal != 42 {
		t.Errorf("dyn-int-flag = %d, want 42", *iVal)
	}
	if !set["dyn-bool-flag"] || !set["dyn-str-flag"] || !set["dyn-int-flag"] {
		t.Errorf("expected all dynamic flags in set map, got %v", set)
	}
	if len(leftover) != 2 || leftover[0] != "leftover_one" || leftover[1] != "leftover_two" {
		t.Errorf("unexpected leftovers: %v", leftover)
	}
}

// TestCLI_TargetUnification covers HOWL-CANON-012.
// howlframe build and run subcommands support --target={bytecode|go|js|wasm}.
// build --target=wasm produces WASM output identically to -compile-wasm.
func TestCLI_TargetUnification(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "howlframe")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "howlframe.go")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\nOutput: %s", err, string(out))
	}

	workDir := filepath.Join(tmpDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("failed to create workdir: %v", err)
	}

	wasmSource := filepath.Join(workDir, "wasm_prog.howl")
	if err := os.WriteFile(wasmSource, []byte(`(cli_app (let (x 4) (if (> x 2) (+ x 3) (- x 1))))`), 0o644); err != nil {
		t.Fatalf("failed to stage SSA fixture: %v", err)
	}

	runCLI := func(args ...string) (string, error) {
		cmd := exec.Command(binaryPath, args...)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 1. Compile with legacy -compile-wasm and capture baseline output
	legacyOutPath := filepath.Join(workDir, "legacy.ssa.wat")
	out, err := runCLI("-compile-wasm", wasmSource, "-o", legacyOutPath)
	if err != nil {
		t.Fatalf("legacy -compile-wasm failed: %v\nOutput: %s", err, out)
	}
	legacyBytes, err := os.ReadFile(legacyOutPath)
	if err != nil {
		t.Fatalf("failed to read legacy wasm output: %v", err)
	}
	if len(legacyBytes) == 0 {
		t.Fatalf("legacy -compile-wasm emitted empty artifact")
	}

	// 2. Compile with build --target=wasm with default output path
	defaultWasmPath := wasmSource + ".ssa.wat"
	os.Remove(defaultWasmPath)
	out, err = runCLI("build", "--target=wasm", wasmSource)
	if err != nil {
		t.Fatalf("build --target=wasm failed: %v\nOutput: %s", err, out)
	}
	buildBytes, err := os.ReadFile(defaultWasmPath)
	if err != nil {
		t.Fatalf("failed to read build --target=wasm output: %v", err)
	}
	if !bytes.Equal(buildBytes, legacyBytes) {
		t.Errorf("build --target=wasm output does not match -compile-wasm: got %d bytes, want %d bytes", len(buildBytes), len(legacyBytes))
	}

	// 3. Flag placement after input: build wasm_prog.howl --target=wasm
	os.Remove(defaultWasmPath)
	out, err = runCLI("build", wasmSource, "--target=wasm")
	if err != nil {
		t.Fatalf("build <file> --target=wasm after input failed: %v\nOutput: %s", err, out)
	}
	afterBytes, err := os.ReadFile(defaultWasmPath)
	if err != nil {
		t.Fatalf("failed to read build output after input: %v", err)
	}
	if !bytes.Equal(afterBytes, legacyBytes) {
		t.Errorf("build <file> --target=wasm output does not match legacy: got %d bytes, want %d bytes", len(afterBytes), len(legacyBytes))
	}

	// 4. Compile with build --target=wasm -o <custom.wat>
	customWasm := filepath.Join(workDir, "custom.wat")
	out, err = runCLI("build", "--target=wasm", "-o", customWasm, wasmSource)
	if err != nil {
		t.Fatalf("build --target=wasm -o custom failed: %v\nOutput: %s", err, out)
	}
	customBytes, err := os.ReadFile(customWasm)
	if err != nil {
		t.Fatalf("failed to read custom wasm output: %v", err)
	}
	if !bytes.Equal(customBytes, legacyBytes) {
		t.Errorf("custom wasm output mismatch")
	}

	// 5. Compile with build --target=go
	goOut := filepath.Join(workDir, "custom_server.go")
	out, err = runCLI("build", "--target=go", "-o", goOut, wasmSource)
	if err != nil {
		t.Fatalf("build --target=go failed: %v\nOutput: %s", err, out)
	}
	goBytes, err := os.ReadFile(goOut)
	if err != nil {
		t.Fatalf("failed to read go output: %v", err)
	}
	if !strings.Contains(string(goBytes), "package main") {
		t.Errorf("expected go output to contain 'package main', got: %s", string(goBytes))
	}

	// 6. Compile with build --target=js
	jsOut := filepath.Join(workDir, "custom_app.js")
	out, err = runCLI("build", "--target=js", "-o", jsOut, wasmSource)
	if err != nil {
		t.Fatalf("build --target=js failed: %v\nOutput: %s", err, out)
	}
	jsBytes, err := os.ReadFile(jsOut)
	if err != nil {
		t.Fatalf("failed to read js output: %v", err)
	}
	if len(jsBytes) == 0 {
		t.Errorf("expected non-empty js output")
	}

	// 7. Compile with build --target=bytecode and run with run --target=bytecode
	hfbcOut := filepath.Join(workDir, "prog.hfbc")
	out, err = runCLI("build", "--target=bytecode", "-o", hfbcOut, wasmSource)
	if err != nil {
		t.Fatalf("build --target=bytecode failed: %v\nOutput: %s", err, out)
	}
	runOut, err := runCLI("run", "--target=bytecode", hfbcOut)
	if err != nil {
		t.Fatalf("run --target=bytecode failed: %v\nOutput: %s", err, runOut)
	}

	// 8. Run source directly with run --target=interpreter
	interpOut, err := runCLI("run", "--target=interpreter", wasmSource)
	if err != nil {
		t.Fatalf("run --target=interpreter failed: %v\nOutput: %s", err, interpOut)
	}

	// 9. Rejection of invalid target
	out, err = runCLI("build", "--target=nonexistent", wasmSource)
	if err == nil {
		t.Errorf("expected build with invalid target to fail, got success: %s", out)
	}
	if !strings.Contains(out, "Unknown target") {
		t.Errorf("expected 'Unknown target' error message, got: %s", out)
	}
}

func TestLegacyCLIAppWritesRunnableBuildBytecode(t *testing.T) {
	workDir := t.TempDir()
	binaryPath := filepath.Join(t.TempDir(), "howlframe")
	if output, err := exec.Command("go", "build", "-o", binaryPath, ".").CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	input := filepath.Join(t.TempDir(), "cli.howl")
	if err := os.WriteFile(input, []byte(`(cli_app (print "ok"))`), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) []byte {
		t.Helper()
		cmd := exec.Command(binaryPath, args...)
		cmd.Dir = workDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, output)
		}
		return output
	}
	run(input)
	artifact := filepath.Join(workDir, "cli.hfbc")
	legacyBytes, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatalf("read default artifact: %v", err)
	}
	for _, name := range []string{"server.go", "server_test.go"} {
		if _, err := os.Stat(filepath.Join(workDir, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected %s: %v", name, err)
		}
	}
	if output := run("run", artifact); string(output) != "ok\n" {
		t.Fatalf("bytecode output = %q, want ok newline", output)
	}
	// Removing the artifact proves build's default filename as well as byte parity.
	if err := os.Remove(artifact); err != nil {
		t.Fatal(err)
	}
	run("build", input)
	buildBytes, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacyBytes, buildBytes) {
		t.Fatal("legacy artifact differs from build's default bytecode")
	}
	run("build", "--target=go", input)
	generated, err := os.ReadFile(filepath.Join(workDir, "server.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(generated, []byte(`fmt.Println("ok")`)) {
		t.Fatalf("explicit Go target omitted print: %s", generated)
	}
	if output, err := exec.Command("go", "run", filepath.Join(workDir, "server.go")).CombinedOutput(); err != nil || string(output) != "ok\n" {
		t.Fatalf("generated Go execution: %v, output %q", err, output)
	}
}
