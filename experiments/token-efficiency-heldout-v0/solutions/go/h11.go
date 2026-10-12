//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/numbers.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	var minimum, maximum, sum int
	for i, line := range lines {
		n, err := strconv.Atoi(line)
		if err != nil {
			panic(err)
		}
		if i == 0 || n < minimum {
			minimum = n
		}
		if i == 0 || n > maximum {
			maximum = n
		}
		sum += n
	}
	result := map[string]int{"min": minimum, "max": maximum, "sum": sum}
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
