//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/romans.txt")
	if err != nil {
		panic(err)
	}
	values := map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100, 'D': 500, 'M': 1000}
	out := []int{}
	for _, line := range strings.Split(string(data), "\n") {
		total := 0
		for i := 0; i < len(line); i++ {
			d := values[line[i]]
			if i+1 < len(line) && d < values[line[i+1]] {
				total -= d
			} else {
				total += d
			}
		}
		out = append(out, total)
	}
	enc, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}
