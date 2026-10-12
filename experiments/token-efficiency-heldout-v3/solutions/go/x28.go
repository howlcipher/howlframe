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

func only(xs, other []string) []string {
	out := []string{}
	for _, x := range xs {
		if !slices.Contains(other, x) && !slices.Contains(out, x) {
			out = append(out, x)
		}
	}
	return out
}

func main() {
	var a, b []string
	readJSON("in/a.json", &a)
	readJSON("in/b.json", &b)
	writeJSON(map[string][]string{"only_a": only(a, b), "only_b": only(b, a)})
}
