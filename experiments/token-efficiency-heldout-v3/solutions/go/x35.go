//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"slices"
	"sort"
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
	var deps map[string][]string
	readJSON("in/deps.json", &deps)
	names := make([]string, 0, len(deps))
	for t := range deps {
		names = append(names, t)
	}
	sort.Strings(names)
	out := []string{}
	for len(out) < len(names) {
		for _, t := range names {
			ready := !slices.Contains(out, t)
			for _, d := range deps[t] {
				ready = ready && slices.Contains(out, d)
			}
			if ready {
				out = append(out, t)
				break
			}
		}
	}
	writeJSON(out)
}
