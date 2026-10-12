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

func flat(d map[string]any, pre string, out map[string]any) {
	for k, v := range d {
		if m, ok := v.(map[string]any); ok && len(m) > 0 {
			flat(m, pre+k+".", out)
		} else {
			out[pre+k] = v
		}
	}
}

func main() {
	var d map[string]any
	readJSON("in/nested.json", &d)
	out := map[string]any{}
	flat(d, "", out)
	writeJSON(out)
}
