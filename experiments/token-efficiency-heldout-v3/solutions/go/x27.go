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

func flat(v any, out []any) []any {
	if l, ok := v.([]any); ok {
		for _, e := range l {
			out = flat(e, out)
		}
		return out
	}
	return append(out, v)
}

func main() {
	var deep []any
	readJSON("in/deep.json", &deep)
	writeJSON(flat(deep, []any{}))
}
