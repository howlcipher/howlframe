//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/events.log")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	counts := map[string]int{}
	for _, line := range lines {
		counts[strings.Split(line, "|")[0]]++
	}
	out, err := json.MarshalIndent(counts, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
