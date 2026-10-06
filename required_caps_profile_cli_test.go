package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/howlcipher/howlframe/internal/bytecode"
	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/vm"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func c1c2Binary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "howlframe")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return bin
}
func TestCLIRequiredCapsAndGoverned(t *testing.T) {
	bin := c1c2Binary(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "x.howl")
	artifact := filepath.Join(dir, "x.hfbc")
	side := filepath.Join(dir, "side")
	os.WriteFile(source, []byte(`(cli_app (do (print "EXECUTED") (write_file "`+side+`" "bad")))`), 0600)
	run := func(args ...string) (string, error) {
		out, err := exec.Command(bin, args...).CombinedOutput()
		return string(out), err
	}
	if out, err := run("build", source, "-o", artifact); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	var previous string
	for _, args := range [][]string{{"inspect", "--caps", artifact}, {"-required-caps", artifact}} {
		out, err := run(args...)
		if err != nil {
			t.Fatalf("%v %s", err, out)
		}
		var r bytecode.Report
		if err := json.Unmarshal([]byte(out), &r); err != nil {
			t.Fatal(out)
		}
		if strings.Join(r.Capabilities, ",") != "filesystem" {
			t.Fatal(r)
		}
		if previous != "" && previous != out {
			t.Fatal("CLI reports differ")
		}
		previous = out
		if _, err := os.Stat(side); !os.IsNotExist(err) {
			t.Fatal("inspection executed")
		}
	}
	for _, args := range [][]string{{"inspect", artifact}, {"inspect", "--caps", source}, {"check", "--profile", "unknown", source}} {
		if out, err := run(args...); err == nil {
			t.Fatalf("accepted %v: %s", args, out)
		}
	}
	for _, app := range []string{"action_executor", "release_authority"} {
		if out, err := run("check", "--profile=governed", filepath.Join("apps", app, app+".howl")); err != nil {
			t.Fatalf("%s: %v %s", app, err, out)
		}
	}
	// Valid default programs for every excluded registry construct.
	forms := map[string]string{
		"achieve": `(achieve "goal")`, "confidence": `(print (confidence "test"))`, "db_connect": `(db_connect db "sqlite3" ":memory:")`, "ephemeral_circuit": `(print (ephemeral_circuit (1) "test"))`, "exec": `(print (exec "echo" "x"))`, "lazy_synthesize": `(lazy_synthesize f () "test")`, "llm_generate": `(print (llm_generate "test"))`, "neural_circuit": `(print (neural_circuit (1) "test"))`, "spawn": `(spawn (lambda () (print "x")))`, "spawn_agent": `(spawn_agent "x" (task "test"))`, "sql_query": `(db_connect db "sqlite3" ":memory:") (print (sql_query db "SELECT 1"))`,
	}
	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			os.WriteFile(source, []byte("(cli_app\n "+form+")"), 0600)
			if out, err := run("check", "--profile", "default", source); err != nil {
				t.Fatalf("default: %v %s", err, out)
			}
			for _, cmd := range []string{"check", "build"} {
				dest := filepath.Join(dir, name+".hfbc")
				args := []string{cmd, "--profile", "governed", source}
				if cmd == "build" {
					args = append(args, "-o", dest)
				}
				out, err := run(args...)
				if err == nil || !strings.Contains(out, "PROFILE_FORBIDDEN_CONSTRUCT") || !strings.Contains(out, name) {
					t.Fatalf("%v %s", err, out)
				}
				if _, err := os.Stat(dest); !os.IsNotExist(err) {
					t.Fatal("artifact written")
				}
			}
		})
	}
	os.WriteFile(source, []byte(`(cli_app (print 1))`), 0600)
	for _, target := range []string{"go", "js"} {
		if out, err := run("build", "--profile=governed", "--target="+target, source, "-o", filepath.Join(dir, "forbidden")); err == nil || !strings.Contains(out, "PROFILE_FORBIDDEN_CONSTRUCT") {
			t.Fatalf("%v %s", err, out)
		}
	}
}

// Compile through the production subprocess so parser/checker ReportError exits
// cannot abort this test. Unsafe effect opcodes are never granted in the corpus;
// those paths receive denial-only checks. Safe filesystem/agent fixtures run
// with their exact report, a finite instruction budget, and isolated working dir.
func TestRequiredCapsFixtureSoundness(t *testing.T) {
	bin := c1c2Binary(t)
	scratch := t.TempDir()
	var fixtures []string
	for _, root := range []string{"tests", "apps"} {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(path, ".howl") {
				fixtures = append(fixtures, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	compiled, granted := 0, 0
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			artifact := filepath.Join(t.TempDir(), "app.hfbc")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if out, err := exec.CommandContext(ctx, bin, "-compile-bc", fixture, "-o", artifact).CombinedOutput(); err != nil {
				t.Skipf("not standalone bytecode: %s", out)
			}
			compiled++
			data, err := os.ReadFile(artifact)
			if err != nil {
				t.Fatal(err)
			}
			prog, err := bytecode.ReadArtifact(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			report := bytecode.RequiredCapabilities(prog)
			unsafe := false
			for _, site := range report.Sites {
				switch site.Capability {
				case "network":
					unsafe = true
				case "process":
					if site.Opcode == "EXEC" || site.Opcode == "SPAWN" {
						unsafe = true
					}
				}
			}
			// Absolute literal paths could escape the scratch directory. Preserve
			// such programs for denial-only checks rather than rewriting semantics.
			inspectPaths := func(insts []bytecode.BCInstruction) {
				for _, inst := range insts {
					if v, ok := inst.ValueOperand.(string); ok && filepath.IsAbs(v) {
						unsafe = true
					}
					if inst.Op == bytecode.OpStoreOpen && strings.HasPrefix(inst.StringOperand2, "file://") && filepath.IsAbs(strings.TrimPrefix(inst.StringOperand2, "file://")) {
						unsafe = true
					}
				}
			}
			inspectPaths(prog.Main)
			for _, fn := range prog.Functions {
				inspectPaths(fn.Instructions)
			}
			args := []string{"-run-bc", "-max-instructions", "1500"}
			if !unsafe {
				args = append(args, "-allow-caps", strings.Join(report.Capabilities, ","))
				granted++
			}
			args = append(args, artifact)
			runctx, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			cmd := exec.CommandContext(runctx, bin, args...)
			cmd.Dir = scratch
			out, _ := cmd.CombinedOutput()
			if !unsafe && strings.Contains(string(out), "CAPABILITY_DENIED") {
				t.Fatalf("underreported %v: %s", report.Capabilities, out)
			}
		})
	}
	if compiled == 0 || granted == 0 {
		t.Fatalf("empty corpus coverage: %d/%d", compiled, granted)
	}
	// Each capability is exercised hermetically; denial precedes any external I/O.
	for _, cap := range capability.All() {
		t.Run("remove_"+string(cap), func(t *testing.T) {
			op := map[capability.Capability]bytecode.Opcode{capability.Network: bytecode.OpFetch, capability.Filesystem: bytecode.OpReadFile, capability.Process: bytecode.OpExec, capability.Environment: bytecode.OpEnv, capability.Database: bytecode.OpStoreOpen}[cap]
			inst := bytecode.BCInstruction{Op: op, StringOperand2: "memory://test"}
			prog := &bytecode.BCProgram{Main: []bytecode.BCInstruction{inst}}
			r := bytecode.RequiredCapabilities(prog)
			var caps []capability.Capability
			for _, c := range r.Capabilities {
				if c != string(cap) {
					caps = append(caps, capability.Capability(c))
				}
			}
			var out, errout bytes.Buffer
			evidence := vm.RunBytecodeWithEvidence(prog, nil, vm.DefaultExecutionPolicy(), caps, strings.NewReader(""), &out, &errout, 0)
			if evidence.RuntimeFailure == nil || evidence.RuntimeFailure.Code != "CAPABILITY_DENIED" {
				t.Fatalf("no denial: %s %s", &out, &errout)
			}
		})
	}
}
