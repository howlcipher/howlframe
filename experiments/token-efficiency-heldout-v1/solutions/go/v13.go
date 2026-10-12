//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/temps.txt")
	if err != nil {
		panic(err)
	}
	var temps []int
	for _, field := range strings.Split(strings.TrimSpace(string(data)), ",") {
		t, err := strconv.Atoi(field)
		if err != nil {
			panic(err)
		}
		temps = append(temps, t)
	}
	totals := make([]int, len(temps))
	sum := 0
	for i, t := range temps {
		sum += t
		totals[i] = sum
	}
	out, err := json.MarshalIndent(totals, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
