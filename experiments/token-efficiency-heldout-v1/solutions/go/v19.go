//go:build ignore

package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile("in/queue.json")
	if err != nil {
		panic(err)
	}
	var queue []string
	if err := json.Unmarshal(data, &queue); err != nil {
		panic(err)
	}
	counts := map[string]int{}
	for _, item := range queue {
		counts[item]++
	}
	out, err := json.MarshalIndent(counts, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
