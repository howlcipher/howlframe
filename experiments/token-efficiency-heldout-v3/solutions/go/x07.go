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
	var rows []map[string]any
	var ren map[string]string
	readJSON("in/rows.json", &rows)
	readJSON("in/rename.json", &ren)
	out := []map[string]any{}
	for _, r := range rows {
		m := map[string]any{}
		for k, v := range r {
			if n, ok := ren[k]; ok {
				k = n
			}
			m[k] = v
		}
		out = append(out, m)
	}
	writeJSON(out)
}
