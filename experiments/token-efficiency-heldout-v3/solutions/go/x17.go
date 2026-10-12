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
	var doc map[string]any
	var ops []struct {
		Op, Path string
		Value    any
	}
	readJSON("in/doc.json", &doc)
	readJSON("in/ops.json", &ops)
	for _, op := range ops {
		parts := strings.Split(op.Path, ".")
		node := doc
		for _, p := range parts[:len(parts)-1] {
			next, ok := node[p].(map[string]any)
			if !ok {
				next = map[string]any{}
				node[p] = next
			}
			node = next
		}
		if last := parts[len(parts)-1]; op.Op == "set" {
			node[last] = op.Value
		} else {
			delete(node, last)
		}
	}
	writeJSON(doc)
}
