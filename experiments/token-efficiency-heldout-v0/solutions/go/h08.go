//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/data.csv")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	col := -1
	for i, name := range strings.Split(lines[0], ",") {
		if name == "amount" {
			col = i
		}
	}
	total := 0
	for _, row := range lines[1:] {
		amount, err := strconv.Atoi(strings.Split(row, ",")[col])
		if err != nil {
			panic(err)
		}
		total += amount
	}
	if err := os.WriteFile("out/result.txt", []byte(fmt.Sprintf("%d\n", total)), 0o644); err != nil {
		panic(err)
	}
}
