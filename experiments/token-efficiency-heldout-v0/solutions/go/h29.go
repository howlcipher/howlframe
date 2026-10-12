//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/records.json")
	if err != nil {
		panic(err)
	}
	var records []map[string]any
	if err := json.Unmarshal(data, &records); err != nil {
		panic(err)
	}
	slim := []map[string]any{}
	for _, r := range records {
		slim = append(slim, map[string]any{"id": r["id"], "status": r["status"]})
	}
	out, err := json.MarshalIndent(slim, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
