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
	above := []int{}
	for _, t := range temps {
		if t > temps[0] {
			above = append(above, t)
		}
	}
	out, err := json.MarshalIndent(above, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
