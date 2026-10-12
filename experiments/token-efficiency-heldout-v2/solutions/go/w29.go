//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	bd, err := os.ReadFile("in/budget.txt")
	if err != nil {
		panic(err)
	}
	budget, err := strconv.Atoi(strings.TrimSpace(string(bd)))
	if err != nil {
		panic(err)
	}
	data, err := os.ReadFile("in/spend.txt")
	if err != nil {
		panic(err)
	}
	result := "within budget"
	total := 0
	for i, line := range strings.Split(string(data), "\n") {
		v, err := strconv.Atoi(line)
		if err != nil {
			panic(err)
		}
		total += v
		if total > budget {
			result = fmt.Sprintf("over at %d", i+1)
			break
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(result+"\n"), 0o644); err != nil {
		panic(err)
	}
}
