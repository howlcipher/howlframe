//go:build ignore

package main

import (
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/latency.txt")
	if err != nil {
		panic(err)
	}
	var values []int
	for _, line := range strings.Split(string(data), "\n") {
		v, err := strconv.Atoi(line)
		if err != nil {
			panic(err)
		}
		values = append(values, v)
	}
	sort.Ints(values)
	if err := os.WriteFile("out/result.txt", []byte(strconv.Itoa(values[len(values)/2])+"\n"), 0o644); err != nil {
		panic(err)
	}
}
