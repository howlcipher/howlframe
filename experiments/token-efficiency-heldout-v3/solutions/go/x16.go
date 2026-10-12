//go:build ignore

package main

import (
	"encoding/json"
	"os"
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
	var env map[string]string
	readJSON("in/env.json", &env)
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []map[string]string{}
	for _, k := range keys {
		out = append(out, map[string]string{"key": k, "value": env[k]})
	}
	writeJSON(out)
}
