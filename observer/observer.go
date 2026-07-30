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
