//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/meter.json")
	if err != nil {
		panic(err)
	}
	var m []int
	if err := json.Unmarshal(data, &m); err != nil {
		panic(err)
	}
	out := []int{}
	for i := 1; i < len(m); i++ {
		out = append(out, m[i]-m[i-1])
	}
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
