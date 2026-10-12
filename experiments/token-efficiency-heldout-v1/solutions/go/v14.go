//go:build ignore

package main

import (
	"os"
	"slices"
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
	if err := os.WriteFile("out/result.txt", []byte(strconv.Itoa(slices.Index(temps, slices.Max(temps))+1)+"\n"), 0o644); err != nil {
		panic(err)
	}
}
