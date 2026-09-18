package observer

import (
	"encoding/json"
	"fmt"
	"os"
)

// Trace logs the start and end of a function block
func Trace(funcName string, vars map[string]any) func() {
	varsJSON, _ := json.Marshal(vars)
	entryMsg := fmt.Sprintf("{\"event\":\"enter\", \"func\":%q, \"vars\":%s}\n", funcName, varsJSON)

	f, err := os.OpenFile("telemetry.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(entryMsg)
	}

	return func() {
		exitMsg := fmt.Sprintf("{\"event\":\"exit\", \"func\":%q}\n", funcName)
		if f != nil {
			f.WriteString(exitMsg)
			f.Close()
		}
	}
}

// RecordOptimizationOpportunity logs when a block exceeds its execution threshold.
// Dynamic native compilation (plugin.Open) was eliminated per HOWL-CANON-001.
func RecordOptimizationOpportunity(metric string, durationMs int64) {
	entryMsg := fmt.Sprintf("{\"event\":\"optimization_opportunity\", \"metric\":%q, \"duration_ms\":%d, \"advisory\":\"dynamic native compilation disabled\"}\n", metric, durationMs)
	f, err := os.OpenFile("telemetry.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(entryMsg)
		f.Close()
	}
}

// OptimizeGoImplementation is retained as a non-executing advisory logger for backward compatibility.
// It records an advisory proposal without compiling or executing untrusted code in-process (HOWL-CANON-001).
func OptimizeGoImplementation(metric string, originalCode string) {
	entryMsg := fmt.Sprintf("{\"event\":\"optimization_proposal\", \"metric\":%q, \"status\":\"advisory_only\"}\n", metric)
	f, err := os.OpenFile("telemetry.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(entryMsg)
		f.Close()
	}
}

// GetOptimizedPlugin is retained for backward compatibility and always returns false.
// In-process dynamic plugin execution via plugin.Open has been permanently disabled (HOWL-CANON-001).
func GetOptimizedPlugin(metric string) (func(), bool) {
	return nil, false
}
