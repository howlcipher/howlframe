//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}

func main() {
	var checks []map[string]interface{}
	if err := json.Unmarshal(mustRead("evidence/checks.json"), &checks); err != nil {
		panic(err)
	}
	mustRead("evidence/version.kv")
	mustRead("evidence/note.txt")
	selected := make([]map[string]interface{}, 0)
	for _, check := range checks {
		if check["status"] == "PASS" {
			selected = append(selected, check)
		}
	}
	record := map[string]interface{}{"batch": "06", "selected": selected}
	path := "out/release.json"
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		panic(err)
	}
}
