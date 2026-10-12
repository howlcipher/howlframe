//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/times.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	total := 0
	for _, line := range lines {
		h, m, _ := strings.Cut(line, ":")
		hours, err := strconv.Atoi(h)
		if err != nil {
			panic(err)
		}
		minutes, err := strconv.Atoi(m)
		if err != nil {
			panic(err)
		}
		total += hours*60 + minutes
	}
	if err := os.WriteFile("out/result.txt", []byte(fmt.Sprintf("%d\n", total)), 0o644); err != nil {
		panic(err)
	}
}
