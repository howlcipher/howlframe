//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
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

func depth(v any) int {
	var kids []any
	switch t := v.(type) {
	case map[string]any:
		for _, x := range t {
			kids = append(kids, x)
		}
	case []any:
		kids = t
	default:
		return 0
	}
	m := 0
	for _, x := range kids {
		m = max(m, depth(x))
	}
	return m + 1
}

func main() {
	var tree any
	readJSON("in/tree.json", &tree)
	if err := os.WriteFile("out/result.txt", []byte(fmt.Sprintln(depth(tree))), 0o644); err != nil {
		panic(err)
	}
}
