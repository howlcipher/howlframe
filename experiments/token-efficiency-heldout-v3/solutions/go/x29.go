//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"slices"
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
	var v []string
	readJSON("in/visits.json", &v)
	out := []string{}
	for i, x := range v {
		if !slices.Contains(v[i+1:], x) {
			out = append(out, x)
		}
	}
	writeJSON(out)
}
