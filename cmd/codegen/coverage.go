package main

import (
	"fmt"
	"strings"

	"github.com/howlcipher/howlframe/internal/capability"
	"github.com/howlcipher/howlframe/internal/construct"
)

func generateCoverageMatrix() string {
	entries := construct.Table()
	var md strings.Builder
	md.WriteString("# HowlFrame Milestone 0 Construct Coverage Matrix\n\n")
	md.WriteString("> **Auto-generated** by `cmd/codegen` from the `internal/construct` and `internal/capability` registries. Do not edit manually.\n\n")
	md.WriteString("| Construct | Standalone Bytecode | Required Capability | Compile-Time Only? | Known Unsupported? | Tracker | Evidence Source |\n")
	md.WriteString("|---|---|---|---|---|---|---|\n")

	for _, entry := range entries {
		name := entry.Name

		bytecode := "No"
		if entry.Support == construct.Supported {
			bytecode = "Yes"
		}

		cap := capability.ForConstruct(name)
		capStr := string(cap)
		if capStr == "" {
			capStr = "None"
		}

		compileTime := "No"
		if entry.Support == construct.CompileTimeOnly {
			compileTime = "Yes"
		}

		unsupported := "No"
		if entry.Support == construct.Unsupported {
			unsupported = "Yes"
		}

		tracker := entry.Tracker
		if tracker == "" {
			tracker = "-"
		}

		evidence := "AUTHORITATIVE"

		md.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %s | %s |\n",
			name, bytecode, capStr, compileTime, unsupported, tracker, evidence))
	}

	return md.String()
}
