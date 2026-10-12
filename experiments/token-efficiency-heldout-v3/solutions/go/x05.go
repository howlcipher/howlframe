//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func readJSON(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		panic(err)
	}
}

func writeJSON(v any) {
	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	var contacts []map[string]any
	readJSON("in/contacts.json", &contacts)
	out := map[string][]any{"present": {}, "null": {}, "absent": {}}
	for _, c := range contacts {
		email, ok := c["email"]
		key := "present"
		if !ok {
			key = "absent"
		} else if email == nil {
			key = "null"
		}
		out[key] = append(out[key], c["id"])
	}
	writeJSON(out)
}
