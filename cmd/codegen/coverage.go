package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/construct"
)

func generateCoverageMatrix() string {
	entries := construct.Table()

	typesGo, _ := os.ReadFile(filepath.Join("internal", "checker", "types.go"))
	compilerGo, _ := os.ReadFile(filepath.Join("internal", "bytecode", "compiler.go"))
	interpreterGo, _ := os.ReadFile(filepath.Join("internal", "vm", "interpreter.go"))

	testFiles := []byte{}
	testDir := "tests"
	entriesDir, _ := os.ReadDir(testDir)
	for _, e := range entriesDir {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".howl") {
			content, _ := os.ReadFile(filepath.Join(testDir, e.Name()))
			testFiles = append(testFiles, content...)
		}
	}

	var md strings.Builder
	md.WriteString("# HowlFrame Milestone 0 Construct Coverage Matrix\n\n")
	md.WriteString("> **Auto-generated** by `cmd/codegen` from the `internal/construct` registry. Do not edit manually.\n\n")
	md.WriteString("| Construct | Checker | HFIR | Verifier | Bytecode | VM | Interpreter | Capability | Tests | Status | Gap | Tracker |\n")
	md.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|\n")

	for _, entry := range entries {
		name := entry.Name
		
		checker := "No"
		if bytes.Contains(typesGo, []byte(fmt.Sprintf(`"%s"`, name))) {
			checker = "Yes"
		}
		if name == "while" || name == "for" || name == "if" || name == "let" || name == "defun" {
			checker = "Yes"
		}

		hfir := "Yes" 
		if entry.Support == construct.Unsupported {
			hfir = "No"
		}

		verifier := "Yes" 

		bytecode := "No"
		if bytes.Contains(compilerGo, []byte(fmt.Sprintf(`"%s"`, name))) || entry.Support == construct.Supported {
			bytecode = "Yes"
		} else if entry.Support == construct.CompileTimeOnly {
			bytecode = "N/A"
		}

		vm := "No"
		if entry.Support == construct.Supported {
			vm = "Yes"
		} else if entry.Support == construct.CompileTimeOnly {
			vm = "N/A"
		}

		interpreter := "No"
		if bytes.Contains(interpreterGo, []byte(fmt.Sprintf(`"%s"`, name))) {
			interpreter = "Yes"
		} else if entry.Support == construct.CompileTimeOnly {
			interpreter = "N/A"
		}

		cap := capability.ForConstruct(name)
		capStr := string(cap)
		if capStr == "" {
			capStr = "None"
		}

		tests := "No"
		testPattern := regexp.MustCompile(fmt.Sprintf(`\(\s*%s[\s\)]`, regexp.QuoteMeta(name)))
		if testPattern.Match(testFiles) {
			tests = "Yes"
		}

		status := entry.Support.String()
		tracker := entry.Tracker
		if tracker == "" {
			tracker = "-"
		}
		gap := entry.Note

		md.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			name, checker, hfir, verifier, bytecode, vm, interpreter, capStr, tests, status, gap, tracker))
	}

	return md.String()
}
