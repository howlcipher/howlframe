//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/words.txt")
	if err != nil {
		panic(err)
	}
	counts := map[string]int{}
	for _, line := range strings.Split(strings.ToLower(string(data)), "\n") {
		for _, word := range strings.Split(line, " ") {
			counts[word]++
		}
	}
	out, err := json.MarshalIndent(counts, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
