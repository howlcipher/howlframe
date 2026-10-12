//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func readList(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var v []string
	if err := json.Unmarshal(data, &v); err != nil {
		panic(err)
	}
	return v
}

func main() {
	left, right := readList("in/left.json"), readList("in/right.json")
	out := []string{}
	for i := 0; i < max(len(left), len(right)); i++ {
		if i < len(left) {
			out = append(out, left[i])
		}
		if i < len(right) {
			out = append(out, right[i])
		}
	}
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
