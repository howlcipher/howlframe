//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/grid.txt")
	if err != nil {
		panic(err)
	}
	var counts []int
	for _, row := range strings.Split(string(data), "\n") {
		counts = append(counts, strings.Count(row, "#"))
	}
	out, err := json.MarshalIndent(counts, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
