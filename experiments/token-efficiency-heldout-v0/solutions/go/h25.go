//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/seq.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	values := make([]int, len(lines))
	for i, line := range lines {
		n, err := strconv.Atoi(line)
		if err != nil {
			panic(err)
		}
		values[i] = n
	}
	result := "sorted"
	for i := 1; i < len(values); i++ {
		if values[i] < values[i-1] {
			result = fmt.Sprintf("unsorted at %d", i+1)
			break
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(result+"\n"), 0o644); err != nil {
		panic(err)
	}
}
