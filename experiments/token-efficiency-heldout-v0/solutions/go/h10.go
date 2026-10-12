//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/data.csv")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	col := -1
	for i, name := range strings.Split(lines[0], ",") {
		if name == "dept" {
			col = i
		}
	}
	counts := map[string]int{}
	for _, row := range lines[1:] {
		counts[strings.Split(row, ",")[col]]++
	}
	out, err := json.MarshalIndent(counts, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
