//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
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
	var flat map[string]any
	readJSON("in/flat.json", &flat)
	out := map[string]any{}
	for k, v := range flat {
		parts := strings.Split(k, ".")
		node := out
		for _, p := range parts[:len(parts)-1] {
			next, ok := node[p].(map[string]any)
			if !ok {
				next = map[string]any{}
				node[p] = next
			}
			node = next
		}
		node[parts[len(parts)-1]] = v
	}
	writeJSON(out)
}
