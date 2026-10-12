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
	for i := range temps {
		temps[i] *= 2
	}
	out, err := json.MarshalIndent(temps, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
