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

func merge(a, b map[string]any) map[string]any {
	for k, v := range b {
		am, ok1 := a[k].(map[string]any)
		bm, ok2 := v.(map[string]any)
		if ok1 && ok2 {
			a[k] = merge(am, bm)
		} else {
			a[k] = v
		}
	}
	return a
}

func main() {
	var base, over map[string]any
	readJSON("in/base.json", &base)
	readJSON("in/override.json", &over)
	writeJSON(merge(base, over))
}
