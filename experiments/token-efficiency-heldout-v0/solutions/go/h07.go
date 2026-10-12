//go:build ignore

package main

import (
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
		if name == "age" {
			col = i
		}
	}
	kept := []string{lines[0]}
	for _, row := range lines[1:] {
		age, err := strconv.Atoi(strings.Split(row, ",")[col])
		if err != nil {
			panic(err)
		}
		if age >= 30 {
			kept = append(kept, row)
		}
	}
	if err := os.WriteFile("out/result.csv", []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
